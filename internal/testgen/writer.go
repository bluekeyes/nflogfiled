package testgen

import (
	"bytes"
	"errors"
	"os"
	"sync"
)

// LineRecordingWriteCloser is an io.WriteCloser that asserts it receives
// writes one line at a time and records the lines.
type LineRecordingWriteCloser struct {
	mu     sync.Mutex
	lines  []string
	closed bool
}

func NewLineRecordingWriteCloser() *LineRecordingWriteCloser {
	return &LineRecordingWriteCloser{}
}

func (w *LineRecordingWriteCloser) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, os.ErrClosed
	}
	if len(p) == 0 || bytes.IndexByte(p, '\n') != len(p)-1 {
		return 0, errors.New("write payload must contain exactly one line")
	}

	w.lines = append(w.lines, string(bytes.Clone(p)))

	return len(p), nil
}

func (w *LineRecordingWriteCloser) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.closed {
		w.closed = true
	}
	return nil
}

func (w *LineRecordingWriteCloser) Lines() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.lines
}
