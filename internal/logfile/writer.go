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

type Writer struct {
	msgs <-chan message.Message
	fmt  Formatter
}

func NewWriter(msgs <-chan message.Message, fmt Formatter) *Writer {
	return &Writer{
		msgs: msgs,
		fmt:  fmt,
	}
}

// Write writes messages from the channel to the writer until the context is
// cancelled. Before it returns, it closes the writer, flushing any data. Write
// logs any errors encountered while writing messages.
func (w *Writer) Write(ctx context.Context, dst io.WriteCloser) {
	buf := make([]byte, 0, initialMessageBufferSize)
	for {
		select {
		case msg := <-w.msgs:
			buf = w.fmt.Format(buf, msg)
			if _, err := dst.Write(buf); err != nil {
				slog.ErrorContext(ctx, "error writing message", "error", err)
			}
			buf = buf[:0]

		case <-ctx.Done():
			if err := dst.Close(); err != nil {
				slog.ErrorContext(ctx, "error closing writer", "error", err)
			}
			return
		}
	}
}
