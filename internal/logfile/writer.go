package logfile

import (
	"context"
	"log/slog"

	"github.com/bluekeyes/nflogfiled/internal/message"
)

const (
	initialMessageBufferSize = 1024
)

// FormatFunc appends a formatted nflog message to b and returns the result.
type FormatFunc func(b []byte, msg message.Message) []byte

type Writer struct {
	msgs  <-chan message.Message
	fmtFn FormatFunc
}

func NewWriter(msgs <-chan message.Message, fmtFn FormatFunc) *Writer {
	return &Writer{
		msgs:  msgs,
		fmtFn: fmtFn,
	}
}

// Write writes messages from the channel to the file until the context is
// cancelled. Before it returns, it closes the file, flushing any data. Write
// logs any errors encountered while writing messages.
func (w *Writer) Write(ctx context.Context, f *File) {
	buf := make([]byte, 0, initialMessageBufferSize)
	for {
		select {
		case msg := <-w.msgs:
			buf = w.fmtFn(buf, msg)
			if _, err := f.Write(buf); err != nil {
				slog.ErrorContext(ctx, "error writing message", "error", err)
			}
			buf = buf[:0]

		case <-ctx.Done():
			if err := f.Close(); err != nil {
				slog.ErrorContext(ctx, "error closing writer", "error", err)
			}
			return
		}
	}
}
