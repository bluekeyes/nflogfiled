package logfile

import (
	"context"
	"io"
	"log/slog"

	"github.com/bluekeyes/nflogfiled/internal/message"
)

const (
	initialMessageBufferSize = 1024
)

// Fromatter defines the Format function, which appends a formatted nflog
// message to b and returns the result.
type Formatter interface {
	Format(b []byte, msg message.Message) []byte
}

// Writer writes formatted messages to a destination. It is not safe to use
// from multiple goroutines.
type Writer struct {
	dst io.WriteCloser
	fmt Formatter
	buf []byte
}

func NewWriter(dst io.WriteCloser, fmt Formatter) *Writer {
	return &Writer{
		dst: dst,
		fmt: fmt,
		buf: make([]byte, 0, initialMessageBufferSize),
	}
}

// Write writes a single message to destination.
func (w *Writer) Write(ctx context.Context, msg message.Message) {
	if w.dst == nil {
		return
	}

	w.buf = w.fmt.Format(w.buf, msg)
	if _, err := w.dst.Write(w.buf); err != nil {
		slog.ErrorContext(ctx, "error writing message", "error", err)
	}
	w.buf = w.buf[:0]
}

// Reset closes the existing destination and replaces it with a new one. Pass a
// nil destination to close the writer without replacement.
func (w *Writer) Reset(ctx context.Context, dst io.WriteCloser) {
	if err := w.dst.Close(); err != nil {
		slog.ErrorContext(ctx, "error closing writer", "error", err)
	}
	w.dst = dst
}
