package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/florianl/go-nflog/v2"

	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/bluekeyes/nflogfiled/internal/message"
	"github.com/bluekeyes/nflogfiled/internal/metrics"
)

// NflogHook returns a hook function that converts nflog.Attribute structs to
// message.Message structs, writing them to msgs. It uses the options to get
// the received at timestamp, resolve device names, and record metrics.
func NflogHook(ctx context.Context, group groups.Group, msgs chan<- message.Message, opts PipelineOptions) nflog.HookFunc {
	var msgCount, droppedMsgCount uint64

	now := opts.NowFunc
	if now == nil {
		now = time.Now
	}

	// TODO(bkeyes): Make sure this hook is not called concurrently on a single
	// connection. If it is, we'll need locking or a pool or something.
	parser := message.NewParser(opts.DeviceNamer)

	return func(attr nflog.Attribute) int {
		msgCount++

		// TODO(bkeyes): consider checking for a full queue before parsing
		msg, err := parser.FromAttribute(group, now(), &attr)
		if err != nil {
			opts.Metrics.MessageDropped(group, metrics.DropParseError)
			slog.WarnContext(ctx, "dropped unparsable message", "error", err)
			return 0
		}
		opts.Metrics.MessageReceived(msg)

		select {
		case msgs <- msg:
		default:
			droppedMsgCount++
			opts.Metrics.MessageDropped(group, metrics.DropChannelFull)
			if droppedMsgCount%100 == 1 { // TODO(bkeyes): make this configurable
				slog.WarnContext(ctx, "dropped nflog messages, output channel is full", "dropped", droppedMsgCount, "total", msgCount)
			}
		}
		return 0
	}
}

// NflogErrorFunc returns an error function that logs errors from the nflog
// connection for group and records kernel buffer overflows in m.
func NflogErrorFunc(ctx context.Context, group groups.Group, m *metrics.Metrics) nflog.ErrorFunc {
	return func(err error) int {
		switch {
		case errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, context.Canceled):
			// Expected after context cancellation when stopping the connection
		case errors.Is(err, syscall.ENOBUFS):
			m.KernelOverflow(group)
			slog.WarnContext(ctx, "kernel dropped nflog messages, socket buffer is full", "group", group)
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
