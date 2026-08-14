package testgen

import (
	"bytes"
	"errors"
	"sync"
)

// LineRecordingWriteCloser is an io.WriteCloser that asserts it receives
// writes one line at a time and records the lines. It automatically closes
// when it receives a target number of lines.
type LineRecordingWriteCloser struct {
	target int
	done   chan struct{}

	mu     sync.Mutex
	lines  []string
	closed bool
}

func NewLineRecordingWriteCloser(n int) *LineRecordingWriteCloser {
	return &LineRecordingWriteCloser{
		done:   make(chan struct{}),
		target: n,
	}
}

func (w *LineRecordingWriteCloser) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, errors.New("writer is closed")
	}
	if len(p) == 0 || bytes.IndexByte(p, '\n') != len(p)-1 {
		return 0, errors.New("write payload must contain exactly one line")
	}

	w.lines = append(w.lines, string(bytes.Clone(p)))
	if len(w.lines) == w.target {
		w.closeLocked()
	}

	return len(p), nil
}

func (w *LineRecordingWriteCloser) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.closeLocked()
	return nil
}

func (w *LineRecordingWriteCloser) closeLocked() {
	if !w.closed {
		close(w.done)
		w.closed = true
	}
}

func (w *LineRecordingWriteCloser) Lines() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.lines
}

func (w *LineRecordingWriteCloser) Done() <-chan struct{} {
	return w.done
}
