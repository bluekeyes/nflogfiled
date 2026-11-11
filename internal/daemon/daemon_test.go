package daemon_test

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/daemon"
	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/bluekeyes/nflogfiled/internal/message"
	"github.com/bluekeyes/nflogfiled/internal/testgen"
	"github.com/florianl/go-nflog/v2"
)

// Scenario is a randomly-generated daemon test case. Tests run the daemon and
// message actions concurrently, then check invariants about the result.
type Scenario struct {
	// Duration gives the total simulated length of the test. Because the test
	// runs in a synctest bubble, this is for ordering events and does not
	// determine how long the test executes in real time.
	Duration time.Duration

	// ReloadAt gives a list of offsets from the start of the test at which to
	// trigger reloads. OpenCalls determines how Open() behaves during each
	// reload and must be the same length as ReloadAt.
	ReloadAt  []time.Duration
	OpenCalls []OpenBehavior

	// WriteDelays determines how Write() calls behave throughout the test. If
	// a call to Write() happens outside of a specified delay, it returns with
	// no dealy.
	WriteDelays []WriteDelay

	// Messages contains the message chunks sent throughout the test. The total
	// duration of chunks must be less than or equal to Duration.
	Messages []MessageChunk
}

func (s Scenario) MarshalJSONTo(enc *jsontext.Encoder) error {
	type rawScenario Scenario

	return json.MarshalEncode(enc, rawScenario(s), json.WithMarshalers(json.JoinMarshalers(
		json.MarshalToFunc(func(enc *jsontext.Encoder, d time.Duration) error {
			return enc.WriteToken(jsontext.String(d.String()))
		}),
	)))
}

// OpenBehavior describes how a particular call to Open() behaves.
type OpenBehavior struct {
	Delay time.Duration
	Error error
}

// WriteDelay describes a delay writing log output to a file. In the time range
// between Start and End, both relative to the start of the test, calls to
// Write wait for Delay before returning.
type WriteDelay struct {
	Start time.Duration
	End   time.Duration
	Delay time.Duration
}

// MessageChunk describes the messages sent over a duration. The messages are
// evenly spaced throughout the duration. If Duration is 0, the messages are
// sent as fast as possible. If Messages is empty and Duration is greater than
// 0, the action waits for Duration without sending any messages.
type MessageChunk struct {
	Messages []testgen.Case
	Duration time.Duration
}

func (ma MessageChunk) String() string {
	// TODO(bkeyes): format this appropriately, may need to indent cases.
	// Alternately, have everything encode to JSON instead of using the custom
	// string formatting for testgen.Case; using JSON would also allow
	// reloading a specific failing scenario from a file. Maybe need a special
	// decoder/type for nflog.Attribute, or stick with default names. Maybe
	// json/v2 can help with this?
	return ""
}

const (
	MaxActionsPerTest = 10

	MaxSleepDuration = 10 * time.Second
	MaxOpenDelay     = 5 * time.Second

	MaxWriteDelay         = 2 * time.Second
	MaxWriteDelayDuration = 10 * time.Second

	MinScenarioDuration = 1 * time.Second
	MaxScenarioDuration = 60 * time.Second

	MinMessageTime     = 1 * time.Second
	MaxMessagesPerTest = 100
)

func TestRandomDaemonOperations(t *testing.T) {
	seed := seedFromEnv(t)
	iters := itersFromEnv(t)

	t.Logf("seed=%d iterations=%d (override with NFLOGFILED_TEST_SEED / NFLOGFILED_TEST_ITERATIONS)", seed, iters)

	for i := range iters {
		// Run each iteration as a subtest to better isolate output
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			configureTestSlog(t)

			v := testgen.NewValues(rand.NewPCG(seed, uint64(i)))
			scenario := generateScenario(v,
				testgen.WithGroups(1), // TODO(bkeyes): figure out how to handle groups
				testgen.WithMask(daemon.DefaultFieldSet),
			)
			runDaemonTest(t, scenario)

			if t.Failed() {
				if b, err := json.Marshal(scenario); err != nil {
					t.Log("error marshalling scenario:", err)
				} else {
					v := jsontext.Value(b)
					v.Indent()
					t.Log("scenario:", v)
				}
			}
		})

		// Stop after the first failure to make output easier to parse
		if t.Failed() {
			return
		}
	}
}

func runDaemonTest(t *testing.T, scenario Scenario) {
	timeValue := testgen.NewTimeValue()
	mockLog := NewMockNflog()

	// TODO(bkeyes): pre-calculate this in the Scenario / provide a helper method?
	totalMessages := 0
	for _, a := range scenario.Messages {
		totalMessages += len(a.Messages)
	}

	var outputs []*ScenarioOutput
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()

		openCount := 0
		nextWriteDelay := func() time.Duration {
			elapsed := time.Since(start)
			for _, d := range scenario.WriteDelays {
				if elapsed >= d.Start && elapsed < d.End {
					return d.Delay
				}
			}
			return 0
		}

		config := daemon.Config{
			Mappings: []groups.Mapping{
				{
					Groups: []groups.Group{1}, // TODO(bkeyes): avoid hardcoded group, maybe part of scenario?
					File:   "messages.log",
				},
			},
			Pipeline: daemon.PipelineOptions{
				Format: &message.TextFormatter{
					Fields: daemon.DefaultFieldSet,
				},
				ChannelBufferSize: totalMessages,
				ConnectFunc:       mockLog.Connect,
				OpenFunc: func(path string, mode os.FileMode) (io.WriteCloser, error) {
					openCount++

					// Open behavior for reloads only applies after the first Open() call
					if openCount > 1 {
						behavior := scenario.OpenCalls[openCount-2]
						if d := behavior.Delay; d > 0 {
							time.Sleep(d)
						}
						if err := behavior.Error; err != nil {
							return nil, err
						}
					}

					o := OpenScenarioOutput(path, mode, nextWriteDelay)
					outputs = append(outputs, o)

					return o, nil
				},
				NowFunc: timeValue.Now,
			},
		}

		d, err := daemon.NewDaemon(config)
		if err != nil {
			t.Fatalf("error constructing daemon: %v", err)
		}

		daemonCtx, cancelDaemon := context.WithCancel(t.Context())
		daemonStopped := make(chan error, 1)

		go func() {
			daemonStopped <- d.Start(daemonCtx)
		}()

		sentAllMessages := make(chan struct{})
		go func() {
			defer close(sentAllMessages)

			// TODO(bkeyes): need to wait for connection on all possible groups here
			mockLog.WaitForConnect(1)

			for _, a := range scenario.Messages {
				if len(a.Messages) == 0 {
					time.Sleep(a.Duration)
					continue
				}

				// TODO(bkeyes): should this account for time spent in Send?
				delay := a.Duration / time.Duration(len(a.Messages))
				for _, msg := range a.Messages {
					if delay > 0 {
						time.Sleep(delay)
					}
					mockLog.Send(msg.Group, msg.Attr)
				}
			}
		}()

		for _, r := range scenario.ReloadAt {
			time.Sleep(r - time.Since(start))
			d.Reload()
		}
		time.Sleep(scenario.Duration - time.Since(start))

		// Wait for the message routine to stop sending, since the times may
		// not add up to exactly the scenario duration
		<-sentAllMessages

		// Cancel the daemon, since we're done sending messages. This should
		// flush anything left in the buffer and terminate the pipeline.
		cancelDaemon()

		// Wait for the daemon to stop. This is also where we find out if it
		// failed during startup. The rest of the test is effectively a no-op
		// if the daemon never started.
		if err := <-daemonStopped; err != nil {
			t.Fatalf("daemon startup error: %v", err)
		}

		// Exit without calling synctest.Wait(), as nothing else should be
		// running at this point and the test should fail if something is
	})

	var gotMessages []testgen.Observation
	for i, out := range outputs {
		if !out.Closed() {
			t.Errorf("output %d was not closed", i)
		}
		if _, after := out.Writes(); after > 0 {
			t.Errorf("output %d recieved %d writes after close", i, after)
		}

		for obs, err := range testgen.ObservationsFromFile(out.Bytes(), testgen.ObservationFromText) {
			if err != nil {
				t.Errorf("output %d contained invalid message: %v", i, err)
			}
			gotMessages = append(gotMessages, obs)
		}
	}

	// Skip final comparison if any output was incorrect or unparseable, since
	// it means the final comparison would also fail.
	if t.Failed() {
		return
	}

	var wantMessages []testgen.Observation
	for _, chunk := range scenario.Messages {
		for _, m := range chunk.Messages {
			wantMessages = append(wantMessages, m.Want)
		}
	}

	if diff := testgen.DiffObservations(wantMessages, gotMessages); diff != "" {
		t.Errorf("incorrect observations (-want, +got):\n%s", diff)
	}
}

type EventDensity int

const (
	None EventDensity = iota
	Sparse
	Dense
)

func generateScenario(v *testgen.Values, opts ...testgen.GenerateOption) Scenario {
	duration := MinScenarioDuration + v.Duration(MaxScenarioDuration-MinScenarioDuration)

	messageCount := 1 + v.IntN(MaxMessagesPerTest)
	messageSlices := int((duration + MinMessageTime) / MinMessageTime)
	messageChunkCount := 1 + v.IntN(messageSlices)
	messageChunks := make([]MessageChunk, messageChunkCount)

	for i := range messageChunkCount {
		var slices, n int
		if i < messageChunkCount-1 {
			remainingChunks := messageChunkCount - i - 1
			slices = 1 + v.IntN(messageSlices-remainingChunks)
			// TODO(bkeyes): make 0 messages a special condition with more weight?
			n = v.IntN(messageCount)
		} else {
			// Always use whatever is left on the last action
			slices = messageSlices
			n = messageCount
		}

		messageChunks[i] = MessageChunk{
			Messages: testgen.GenerateBatch(v, n, opts...),
			Duration: time.Duration(slices) * MinMessageTime,
		}

		messageSlices -= slices
		messageCount -= n
	}

	var reloadCount int
	switch v.Pick(None, Sparse, Dense) {
	case None:
		reloadCount = 0
	case Sparse:
		reloadCount = 1 + v.IntN(2)
	case Dense:
		reloadCount = 1 + v.IntN(min(20, messageSlices))
	}

	reloads := make([]time.Duration, reloadCount)
	for i := range reloadCount {
		var t time.Duration
		if i > 0 {
			t = reloads[i-1]
		}
		reloads[i] = t + v.Duration(duration-t)
	}

	openCalls := make([]OpenBehavior, reloadCount)
	if reloadCount > 0 {
		var openDelayCount int
		switch v.Pick(None, Sparse, Dense) {
		case None:
			openDelayCount = 0
		case Sparse:
			openDelayCount = v.IntN(min(3, reloadCount))
		case Dense:
			openDelayCount = 1 + v.IntN(min(10, reloadCount))
		}

		for i := range v.PickIndexN(openDelayCount, len(openCalls)) {
			// TODO(bkeyes): bias this towards smaller values? coordiate with reloads or messages?
			openCalls[i].Delay = v.Duration(3 * MinMessageTime)
		}

		var openErrorCount int
		switch v.Pick(None, Sparse, Dense) {
		case None:
			openErrorCount = 0
		case Sparse:
			openErrorCount = v.IntN(min(3, reloadCount))
		case Dense:
			openErrorCount = 1 + v.IntN(min(5, reloadCount))
		}

		for i := range v.PickIndexN(openErrorCount, len(openCalls)) {
			openCalls[i].Error = errors.New("error opening file")
		}
	}

	var writeDelayWindow time.Duration
	switch v.Pick(None, Sparse, Dense) {
	case None:
		writeDelayWindow = 0
	case Sparse:
		frac := (10 + float64(v.IntN(15))) / 100
		writeDelayWindow = time.Duration(frac * float64(duration))
	case Dense:
		frac := (50 + float64(v.IntN(25))) / 100
		writeDelayWindow = time.Duration(frac * float64(duration))
	}

	var writeDelays []WriteDelay
	for writeDelayWindow > 0 {
		var window time.Duration
		if writeDelayWindow <= MinMessageTime {
			window = writeDelayWindow
		} else {
			window = v.Duration(writeDelayWindow)
		}
		writeDelayWindow -= window

		var delay time.Duration
		if v.Bool() {
			delay = v.Duration(50 * time.Millisecond)
		} else {
			delay = v.Duration(2 * MinMessageTime)
		}

		var start time.Duration
		if len(writeDelays) > 0 {
			start = writeDelays[len(writeDelays)-1].End
		}

		remaining := max(0, duration-start-writeDelayWindow-window)
		start += v.Duration(remaining)

		writeDelays = append(writeDelays, WriteDelay{
			Start: start,
			End:   start + window,
			Delay: delay,
		})
	}

	return Scenario{
		Duration:    duration,
		Messages:    messageChunks,
		ReloadAt:    reloads,
		OpenCalls:   openCalls,
		WriteDelays: writeDelays,
	}
}

func TestDaemonDrainsMessagesOnStop(t *testing.T) {
	// block writes
	// queue up a bunch of messages
	// call stop
	// unblock writes
	// make sure everything appears on the output
}

func TestDaemonOutputContinuesDuringReload(t *testing.T) {
	// trigger reload
	// once in open, block
	// queue up a bunch of messages
	// verify they appear on the existing file
}

type MockNflog struct {
	mu       sync.Mutex
	conns    map[groups.Group]*MockNflogConnection
	notifier *notifier[groups.Group]
}

func NewMockNflog() *MockNflog {
	m := MockNflog{
		conns: make(map[groups.Group]*MockNflogConnection),
	}

	m.notifier = newNotifier[groups.Group](func(g groups.Group) bool {
		_, ok := m.conns[g]
		return ok
	})

	return &m
}

func (m *MockNflog) Connect(ctx context.Context, group groups.Group, hook nflog.HookFunc, errFunc nflog.ErrorFunc, opts daemon.PipelineOptions) (daemon.Connection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn, exists := m.conns[group]; exists {
		return conn, nil
	}

	conn := &MockNflogConnection{
		msgs: make(chan nflog.Attribute),
		ack:  make(chan struct{}),
		done: make(chan struct{}),
	}

	go func() {
		for {
			select {
			case msg := <-conn.msgs:
				hook(msg)
				conn.ack <- struct{}{}
			case <-conn.done:
				return
			}
		}
	}()

	m.conns[group] = conn
	m.notifier.notify()

	return conn, nil
}

func (m *MockNflog) Send(group groups.Group, msg nflog.Attribute) {
	m.mu.Lock()
	conn, ok := m.conns[group]
	m.mu.Unlock()

	// Send message without holding lock, as it may block
	if ok {
		conn.Send(msg)
	}
}

func (m *MockNflog) WaitForConnect(g groups.Group) {
	m.mu.Lock()
	w := m.notifier.register(g)
	m.mu.Unlock()

	w.wait()
}

func (m *MockNflog) WaitForConnectUntil(g groups.Group, timeout time.Duration) error {
	m.mu.Lock()
	w := m.notifier.register(g)
	m.mu.Unlock()

	return w.waitUntil(fmt.Sprintf("for connect on group %d", g), timeout)
}

type MockNflogConnection struct {
	msgs chan nflog.Attribute
	ack  chan struct{}
	done chan struct{}
}

// Send delivers msg to the connection's hook, blocking until the hook returns.
func (c *MockNflogConnection) Send(msg nflog.Attribute) {
	select {
	case c.msgs <- msg:
		<-c.ack
	case <-c.done:
		// Avoid blocking sends if the connection is closed
		return
	}
}

func (c *MockNflogConnection) Close() error {
	// TODO(bkeyes): does this need to be safe for multiple calls?
	close(c.done)
	return nil
}

// ScenarioOutput is an io.WriteCloser that implements the fault injection and
// monitoring methods need for daemon scenario testing.
type ScenarioOutput struct {
	Path string
	Mode os.FileMode

	b                []byte
	writes           int
	writesAfterClose int
	closed           bool

	delay func() time.Duration
}

func OpenScenarioOutput(path string, mode os.FileMode, delay func() time.Duration) *ScenarioOutput {
	return &ScenarioOutput{
		Path: path,
		Mode: mode,

		delay: delay,
	}
}

func (o *ScenarioOutput) Write(p []byte) (n int, err error) {
	if o.closed {
		o.writesAfterClose++
		return 0, os.ErrClosed
	}

	if o.delay != nil {
		if d := o.delay(); d > 0 {
			time.Sleep(d)
		}
	}

	o.writes++
	o.b = append(o.b, p...)
	return len(p), nil
}

func (o *ScenarioOutput) Close() error {
	o.closed = true
	return nil
}

func (o *ScenarioOutput) Bytes() []byte {
	return bytes.Clone(o.b)
}

func (o *ScenarioOutput) Writes() (writes, writesAfterClose int) {
	return o.writes, o.writesAfterClose
}

func (o *ScenarioOutput) Closed() bool {
	return o.closed
}

type notifier[T any] struct {
	condition func(T) bool
	waiters   []waiter[T]
}

func newNotifier[T any](cond func(T) bool) *notifier[T] {
	return &notifier[T]{
		condition: cond,
	}
}

func (n *notifier[T]) register(trigger T) waiter[T] {
	w := waiter[T]{
		notify:  make(chan struct{}),
		trigger: trigger,
	}

	if n.condition(trigger) {
		close(w.notify)
	} else {
		n.waiters = append(n.waiters, w)
	}

	return w
}

func (n *notifier[T]) notify() {
	var stillWaiting []waiter[T]
	for _, w := range n.waiters {
		if n.condition(w.trigger) {
			close(w.notify)
		} else {
			stillWaiting = append(stillWaiting, w)
		}
	}
	n.waiters = stillWaiting
}

type waiter[T any] struct {
	notify  chan struct{}
	trigger T
}

func (w waiter[T]) wait() {
	<-w.notify
}

func (w waiter[T]) waitUntil(msg string, timeout time.Duration) error {
	select {
	case <-w.notify:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout after %s waiting %s", timeout, msg)
	}
}

func configureTestSlog(t *testing.T) {
	oldLogger := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(t.Output(), nil)))

	t.Cleanup(func() {
		slog.SetDefault(oldLogger)
	})
}
