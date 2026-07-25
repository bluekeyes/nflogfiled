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
	f     *File
	msgs  <-chan nflog.Attribute
	fmtFn FormatFunc
}

func NewWriter(f *File, msgs <-chan nflog.Attribute, fmtFn FormatFunc) *Writer {
	return &Writer{
		f:     f,
		msgs:  msgs,
		fmtFn: fmtFn,
	}
}

// Run starts the writing process and blocks until the context is cancelled.
// Before it returns, it closes the target file, flushing any data. It logs any
// errors encountered while writing messages.
func (w *Writer) Run(ctx context.Context) {
	var buf bytes.Buffer
	for {
		select {
		case msg := <-w.msgs:
			w.fmtFn(&buf, msg)
			if _, err := buf.WriteTo(w.f); err != nil {
				slog.ErrorContext(ctx, "error writing message", "error", err)
			}

		case <-ctx.Done():
			if err := w.f.Close(); err != nil {
				slog.ErrorContext(ctx, "error closing writer", "error", err)
			}
			return
		}
	}
}
