package daemon_test

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/daemon"
	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/bluekeyes/nflogfiled/internal/message"
	"github.com/bluekeyes/nflogfiled/internal/testgen"
	"github.com/florianl/go-nflog/v2"
)

func TestDaemonReload(t *testing.T) {
	const (
		testGroup = groups.Group(1)
	)

	timeValue := testgen.NewTimeValue()
	mockLog := NewMockNflog()

	outputPath := filepath.Join(t.TempDir(), "messages.log")
	outputFile := make(chan *MonitoringFile, 1)

	config := daemon.Config{
		Mappings: []groups.Mapping{
			{
				Groups: []groups.Group{testGroup},
				File:   outputPath,
			},
		},
		Pipeline: daemon.PipelineOptions{
			Format: &message.TextFormatter{
				Fields: daemon.DefaultFieldSet,
			},
			ConnectFunc: mockLog.Connect,
			OpenFunc: func(path string, mode os.FileMode) (io.WriteCloser, error) {
				f, err := OpenMonitoringFile(path, mode)
				if err != nil {
					return nil, err
				}
				outputFile <- f
				return f, nil
			},
			NowFunc: timeValue.Now,
		},
	}

	d, err := daemon.NewDaemon(config)
	if err != nil {
		t.Fatalf("error constructing daemon: %v", err)
	}

	// TODO(bkeyes): do something to actually check this error; I think this
	// means creating a separate context we can cancel after receiving all the
	// messages and essentiall running to go routines, one for the daemon and
	// one for the rest of the test, then reading from both of them before
	// asserting the final result
	daemonErr := make(chan error, 1)
	go func() {
		err := d.Start(t.Context())
		if err != nil {
			t.Logf("daemon startup error: %v", err)
		}
		daemonErr <- err
	}()

	const (
		messageCount          = 100
		preReloadMessageCount = 10
	)

	gen := testgen.NewValues(rand.NewPCG(1, 2)) // TODO(bkeyes): adjust seed?
	cases := testgen.GenerateBatch(
		gen, messageCount,
		testgen.WithGroup(1),
		testgen.WithMask(daemon.DefaultFieldSet),
	)

	go func() {
		if err := mockLog.WaitForConnect(1, 5*time.Second); err != nil {
			t.Error(err)
			return
		}
		for _, c := range cases {
			mockLog.Send(1, c.Attr)
		}
	}()

	file1, err := recv(outputFile, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for a small number of messages to make it more likely messages are
	// still being sent or in the buffer when the signal arrives. Note that
	// there's no way to guarantee this with the current design except maybe by
	// blocking in the Write() function...
	if err := file1.WaitForWrites(preReloadMessageCount, 5*time.Second); err != nil {
		t.Fatal(err)
	}

	d.Reload()

	if err := file1.WaitForClose(5 * time.Second); err != nil {
		t.Fatalf("no close call after reload: %v", err)
	}

	file2, err := recv(outputFile, 5*time.Second)
	if err != nil {
		t.Fatalf("no new file after reload: %v", err)
	}

	postReloadMessageCount := messageCount - file1.WriteCount()
	if err := file2.WaitForWrites(postReloadMessageCount, 10*time.Second); err != nil {
		t.Fatal(err)
	}

	wantMessages := make([]testgen.Observation, messageCount)
	for i, c := range cases {
		wantMessages[i] = c.Want
	}

	fileContent, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("error reading output file: %v", err)
	}

	gotMessages := make([]testgen.Observation, 0, messageCount)
	for obs, err := range testgen.ObservationsFromFile(fileContent, testgen.ObservationFromText) {
		if err != nil {
			t.Errorf("invalid output line: %v", err)
		}
		gotMessages = append(gotMessages, obs)
	}

	if diff := testgen.DiffObservations(wantMessages, gotMessages); diff != "" {
		t.Errorf("incorrect observations (-want, +got):\n%s", diff)
	}
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
		done: make(chan struct{}),
	}

	go func() {
		for {
			select {
			case msg := <-conn.msgs:
				hook(msg)
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

func (m *MockNflog) WaitForConnect(g groups.Group, timeout time.Duration) error {
	m.mu.Lock()
	w := m.notifier.register(g)
	m.mu.Unlock()

	return w.wait(fmt.Sprintf("for connect on group %d", g), timeout)
}

type MockNflogConnection struct {
	msgs chan nflog.Attribute
	done chan struct{}
}

func (c *MockNflogConnection) Send(msg nflog.Attribute) {
	select {
	case c.msgs <- msg:
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

type MonitoringFile struct {
	mu     sync.Mutex
	f      *os.File
	writes int

	writeNotifier *notifier[int]
	closeNotifier *notifier[any]
}

func OpenMonitoringFile(path string, mode os.FileMode) (*MonitoringFile, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, mode)
	if err != nil {
		return nil, err
	}

	mf := MonitoringFile{f: f}

	mf.writeNotifier = newNotifier[int](func(n int) bool {
		return mf.writes >= n
	})
	mf.closeNotifier = newNotifier[any](func(_ any) bool {
		return mf.f == nil
	})

	return &mf, nil
}

func (mf *MonitoringFile) Write(p []byte) (int, error) {
	mf.mu.Lock()
	defer mf.mu.Unlock()

	if mf.f == nil {
		return 0, os.ErrClosed
	}

	// Write calls count whether or not they succeed
	mf.writes++
	mf.writeNotifier.notify()

	return mf.f.Write(p)
}

func (mf *MonitoringFile) Close() error {
	mf.mu.Lock()
	defer mf.mu.Unlock()

	if mf.f == nil {
		return nil
	}

	err := mf.f.Close()
	mf.f = nil

	// Close calls count whether or not they succeed
	mf.closeNotifier.notify()

	return err
}

// WaitForWrites waits for a total of n Write() calls. This value counts from
// the creation of the MonitoringFile, not from the call to WaitForWrites.
func (mf *MonitoringFile) WaitForWrites(n int, timeout time.Duration) error {
	if n <= 0 {
		return nil
	}

	mf.mu.Lock()
	w := mf.writeNotifier.register(n)
	mf.mu.Unlock()

	return w.wait(fmt.Sprintf("for %d/%d writes", mf.WriteCount(), n), timeout)
}

func (mf *MonitoringFile) WriteCount() int {
	mf.mu.Lock()
	defer mf.mu.Unlock()

	return mf.writes
}

func (mf *MonitoringFile) WaitForClose(timeout time.Duration) error {
	mf.mu.Lock()
	w := mf.closeNotifier.register(nil)
	mf.mu.Unlock()

	return w.wait("for close", timeout)
}

func recv[T any](c chan T, timeout time.Duration) (T, error) {
	select {
	case v := <-c:
		return v, nil
	case <-time.After(timeout):
		var zero T
		return zero, fmt.Errorf("timeout after %s receiving from %T", timeout, c)
	}
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

func (w waiter[T]) wait(msg string, timeout time.Duration) error {
	select {
	case <-w.notify:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout after %s waiting %s", timeout, msg)
	}
}
