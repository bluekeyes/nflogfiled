package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/bluekeyes/nflogfiled/internal/message"
	"github.com/florianl/go-nflog/v2"
)

// NflogHook returns a hook function that converts nflog.Attribute structs to
// message.Message structs, writing them to msgs. It calls the now function for
// each message to to retrieve the received at timestamp.
func NflogHook(ctx context.Context, group groups.Group, msgs chan<- message.Message, now func() time.Time) nflog.HookFunc {
	var msgCount, droppedMsgCount uint64

	// TODO(bkeyes): Make sure this hook is not called concurrently on a single
	// connection. If it is, we'll need locking or a pool or something.
	parser := message.NewParser()

	return func(attr nflog.Attribute) int {
		msgCount++

		// TODO(bkeyes): consider checking for a full queue before parsing
		msg, err := parser.FromAttribute(group, now(), &attr)
		if err != nil {
			slog.WarnContext(ctx, "dropped unparsable message", "error", err)
			return 0
		}

		select {
		case msgs <- msg:
		default:
			droppedMsgCount++
			if droppedMsgCount%100 == 1 { // TODO(bkeyes): make this configurable
				slog.WarnContext(ctx, "dropped nflog messages, output channel is full", "dropped", droppedMsgCount, "total", msgCount)
			}
		}
		return 0
	}
}

func NflogErrorFunc(ctx context.Context) nflog.ErrorFunc {
	return func(err error) int {
		switch {
		case errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, context.Canceled):
			// Expected after context cancellation when stopping the connection
		case errors.Is(err, syscall.ENOBUFS):
			slog.WarnContext(ctx, "kernel dropped nflog messages, socket buffer is full")
		default:
			slog.ErrorContext(ctx, "error receiving nflog message", "error", err)
		}
		return 0
	}
}

type nflogLogger struct {
	ctx context.Context
}

func (l nflogLogger) Debugf(format string, args ...any) {
	slog.DebugContext(l.ctx, fmt.Sprintf(format, args...), "subsystem", "nflog")
}

func (l nflogLogger) Errorf(format string, args ...any) {
	slog.ErrorContext(l.ctx, fmt.Sprintf(format, args...), "subsystem", "nflog")
}
