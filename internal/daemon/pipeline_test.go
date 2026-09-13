package daemon_test

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/daemon"
	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/bluekeyes/nflogfiled/internal/logfile"
	"github.com/bluekeyes/nflogfiled/internal/message"
	"github.com/bluekeyes/nflogfiled/internal/testgen"
	"github.com/florianl/go-nflog/v2"
	"github.com/google/go-cmp/cmp"
)

var cmpOpts = []cmp.Option{
	cmp.Comparer(func(a, b netip.Addr) bool { return a == b }),
}

func TestRandomMessagePipeline(t *testing.T) {
	seed := seedFromEnv(t)
	iters := itersFromEnv(t)

	t.Logf("seed=%d iterations=%d (override with NFLOGFILED_TEST_SEED / NFLOGFILED_TEST_ITERATIONS)", seed, iters)

	for i := range iters {
		v := testgen.NewValues(rand.NewPCG(seed, uint64(i)))
		cases := testgen.GenerateBatch(v, 1+v.IntN(8))

		if errs := checkCases(t.Context(), cases); len(errs) > 0 {
			var casesStr strings.Builder
			for i, c := range cases {
				fmt.Fprintf(&casesStr, "%d: %s\n", i, c)
			}
			t.Logf("iteration=%d cases=%d cases[]=\n%s", i, len(cases), casesStr.String())

			for _, err := range errs {
				t.Log(err)
			}
			t.FailNow()
		}
	}
}

func FuzzMessagePipeline(f *testing.F) {
	const seedLen = 1024

	f.Add(bytes.Repeat([]byte{0x00}, seedLen))
	f.Add(bytes.Repeat([]byte{0xFF}, seedLen))

	f.Fuzz(func(t *testing.T, b []byte) {
		v := testgen.NewValues(testgen.ByteSource(b))
		cases := testgen.GenerateBatch(v, 1+v.IntN(4))

		if errs := checkCases(t.Context(), cases); len(errs) > 0 {
			var casesStr strings.Builder
			for i, c := range cases {
				fmt.Fprintf(&casesStr, "%d: %s\n", i, c)
			}
			t.Logf("cases=%d cases[]=\n%s", len(cases), casesStr.String())

			for _, err := range errs {
				t.Log(err)
			}
			t.Fail()
		}
	})
}

func checkCases(ctx context.Context, cases []testgen.Case) []error {
	var errs []error

	// TODO(bkeyes): output field selection should also be randomized
	textLines := runPipeline(ctx, cases, &message.TextFormatter{Fields: message.AllFields})
	jsonLines := runPipeline(ctx, cases, &message.JSONFormatter{Fields: message.AllFields})

	textObs := make([]testgen.Observation, len(textLines))
	for i, line := range textLines {
		obs, err := testgen.ObservationFromText(line)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid text line %d: %w", i+1, err))
		}
		textObs[i] = obs
	}

	jsonObs := make([]testgen.Observation, len(jsonLines))
	for i, line := range jsonLines {
		obs, err := testgen.ObservationFromJSON(line)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid JSON line %d: %w", i+1, err))
		}
		jsonObs[i] = obs
	}

	// If any of the messages was invalid, exit early because the comparisons
	// will also fail due to the incomplete observations
	if len(errs) > 0 {
		return errs
	}

	wantObs := make([]testgen.Observation, len(cases))
	for i, c := range cases {
		wantObs[i] = c.Want
	}

	if diff := cmp.Diff(wantObs, textObs, cmpOpts...); diff != "" {
		errs = append(errs, fmt.Errorf("incorrect text observations (-want, +got):\n%s", diff))
	}
	if diff := cmp.Diff(wantObs, jsonObs, cmpOpts...); diff != "" {
		errs = append(errs, fmt.Errorf("incorrect JSON observations (-want, +got):\n%s", diff))
	}
	if diff := cmp.Diff(textObs, jsonObs, cmpOpts...); diff != "" {
		errs = append(errs, fmt.Errorf("text and JSON observations differ (-text, +json):\n%s", diff))
	}
	return errs
}

func runPipeline(ctx context.Context, cases []testgen.Case, f logfile.Formatter) []string {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	msgs := make(chan message.Message, len(cases))
	done := make(chan struct{})

	w := testgen.NewLineRecordingWriteCloser(len(cases))

	go func() {
		logfile.NewWriter(msgs, f).Write(ctx, w)
		close(done)
	}()

	index := 0
	now := func() time.Time {
		return cases[index].ReceivedAt
	}

	hooks := make(map[groups.Group]nflog.HookFunc)
	for _, c := range cases {
		if _, exists := hooks[c.Group]; !exists {
			hooks[c.Group] = daemon.NflogHook(ctx, c.Group, msgs, now)
		}
	}

	for _, c := range cases {
		hooks[c.Group](c.Attr)
		index++
	}

	// Wait for the target to recieve all of the expected cases. This must
	// happen before we cancel the context, as logfile.Writer does not consume
	// all bufferred messages before exiting.
	<-w.Done()

	// Once we received all the output, cancel the context and wait for the
	// writer goroutine to exit.
	cancel()
	<-done

	return w.Lines()
}

func seedFromEnv(t *testing.T) uint64 {
	t.Helper()

	if s, ok := os.LookupEnv("NFLOGFILED_TEST_SEED"); ok {
		seed, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			t.Fatalf("NFLOGFILED_TEST_SEED contains an invalid seed value: %v", err)
		}
		return seed
	}
	return uint64(time.Now().UnixNano())
}

func itersFromEnv(t *testing.T) int {
	t.Helper()

	if i, ok := os.LookupEnv("NFLOGFILED_TEST_ITERATIONS"); ok {
		iters, err := strconv.Atoi(i)
		if err != nil {
			t.Fatalf("NFLOGFILED_TEST_ITERATIONS contains an invalid iterations value: %v", err)
		}
		if iters <= 0 {
			t.Fatal("NFLOGFILED_TEST_ITERATIONS contains an invalid iterations value: iteration count must be positive")
		}
		return iters
	}
	return 1000
}
