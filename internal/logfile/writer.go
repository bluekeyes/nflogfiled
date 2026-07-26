package logfile

import (
	"bytes"
	"context"
	"io"
	"log/slog"

	"github.com/florianl/go-nflog/v2"
)

// FormatFunc writes a formatted nflog message to the given [io.Writer]. The
// writer will never return an error.
type FormatFunc func(w io.Writer, msg nflog.Attribute)

type Writer struct {
	msgs  <-chan nflog.Attribute
	fmtFn FormatFunc
}

func NewWriter(msgs <-chan nflog.Attribute, fmtFn FormatFunc) *Writer {
	return &Writer{
		msgs:  msgs,
		fmtFn: fmtFn,
	}
}

// Write writes messages from the channel to the file until the context is
// cancelled. Before it returns, it closes the file, flushing any data. Write
// logs any errors encountered while writing messages.
func (w *Writer) Write(ctx context.Context, f *File) {
	var buf bytes.Buffer
	for {
		select {
		case msg := <-w.msgs:
			w.fmtFn(&buf, msg)
			if _, err := buf.WriteTo(f); err != nil {
				slog.ErrorContext(ctx, "error writing message", "error", err)
			}

		case <-ctx.Done():
			if err := f.Close(); err != nil {
				slog.ErrorContext(ctx, "error closing writer", "error", err)
			}
			return
		}
	}
}
