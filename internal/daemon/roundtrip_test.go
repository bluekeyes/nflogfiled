package daemon_test

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
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
)

func TestRandomMessageRoundtrip(t *testing.T) {
	seed := seedFromEnv(t)
	iters := itersFromEnv(t)

	t.Logf("seed=%d iterations=%d (override with NFLOGFILED_TEST_SEED / NFLOGFILED_TEST_ITERATIONS)", seed, iters)

	for i := range iters {
		v := testgen.NewValues(rand.NewPCG(seed, uint64(i)))

		fieldSet := message.AllFields
		if v.BoolRatio(1, 4) {
			fieldSet = message.FieldSet(v.Uint64())
		}

		cases := testgen.GenerateBatch(v, 1+v.IntN(8), testgen.WithMask(fieldSet))

		if errs := roundtripCases(t.Context(), cases, fieldSet); len(errs) > 0 {
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

func FuzzMessageRoundtrip(f *testing.F) {
	const seedLen = 1024

	f.Add(bytes.Repeat([]byte{0x00}, seedLen))
	f.Add(bytes.Repeat([]byte{0xFF}, seedLen))

	f.Fuzz(func(t *testing.T, b []byte) {
		v := testgen.NewValues(testgen.ByteSource(b))

		fieldSet := message.AllFields
		if v.BoolRatio(1, 4) {
			fieldSet = message.FieldSet(v.Uint64())
		}

		cases := testgen.GenerateBatch(v, 1+v.IntN(4), testgen.WithMask(fieldSet))

		if errs := roundtripCases(t.Context(), cases, fieldSet); len(errs) > 0 {
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

func roundtripCases(ctx context.Context, cases []testgen.Case, fieldSet message.FieldSet) []error {
	var errs []error

	textLines := runPipeline(ctx, cases, &message.TextFormatter{Fields: fieldSet})
	jsonLines := runPipeline(ctx, cases, &message.JSONFormatter{Fields: fieldSet})

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

	if diff := testgen.DiffObservations(wantObs, textObs); diff != "" {
		errs = append(errs, fmt.Errorf("incorrect text observations (-want, +got):\n%s", diff))
	}
	if diff := testgen.DiffObservations(wantObs, jsonObs); diff != "" {
		errs = append(errs, fmt.Errorf("incorrect JSON observations (-want, +got):\n%s", diff))
	}
	if diff := testgen.DiffObservations(textObs, jsonObs); diff != "" {
		errs = append(errs, fmt.Errorf("text and JSON observations differ (-text, +json):\n%s", diff))
	}
	return errs
}

func runPipeline(ctx context.Context, cases []testgen.Case, f logfile.Formatter) []string {
	msgs := make(chan message.Message, len(cases))
	done := make(chan struct{})

	dst := testgen.NewLineRecordingWriteCloser()

	go func() {
		w := logfile.NewWriter(dst, f)
		for msg := range msgs {
			w.Write(ctx, msg)
		}

		dst.Close()
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

	// Stop the writer after buffering all messages through the hook, then wait
	// for it to close the destination and exit
	close(msgs)
	<-done

	return dst.Lines()
}

// defaultTestSeed is an arbitrary seed that makes generated tests
// deterministic across runs. The value is a nanosecond epoch timestamp from
// shortly before adding this definition.
const defaultTestSeed uint64 = 0x18d5102dcb24e387

func seedFromEnv(t *testing.T) uint64 {
	t.Helper()

	if s, ok := os.LookupEnv("NFLOGFILED_TEST_SEED"); ok {
		if strings.ToLower(s) == "time" {
			return uint64(time.Now().UnixNano())
		}

		seed, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			t.Fatalf("NFLOGFILED_TEST_SEED contains an invalid seed value: %v", err)
		}
		return seed
	}

	return defaultTestSeed
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
	return 2000
}
