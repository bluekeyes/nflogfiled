package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"github.com/florianl/go-nflog/v2"
)

func NflogHook(ctx context.Context, msgs chan<- nflog.Attribute) nflog.HookFunc {
	var msgCount, droppedMsgCount uint64

	return func(attrs nflog.Attribute) int {
		msgCount++
		select {
		case msgs <- attrs:
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
