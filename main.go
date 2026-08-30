package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/cli"
	"github.com/bluekeyes/nflogfiled/internal/daemon"
	"github.com/bluekeyes/nflogfiled/internal/message"
)

var opts struct {
	logLevel int
	mappings cli.GroupMappingValue
	format   string
	fileMode cli.FileModeValue
	fields   cli.FieldSetValue

	channelBufferSize    int
	packetBufferSize     uint64
	kernelReadBufferSize int
	kernelFlushTimeout   time.Duration
	kernelFlushCount     uint64
}

func die(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+msg+"\n", args...)
	os.Exit(1)
}

func init() {
	flag.IntVar(&opts.logLevel, "log-level", 0, "")
	flag.Var(&opts.mappings, "map", "")
	flag.StringVar(&opts.format, "format", "", "")
	flag.Var(&opts.fileMode, "file-mode", "")
	flag.Var(&opts.fields, "fields", "")

	flag.IntVar(&opts.channelBufferSize, "channel-buffer-size", 0, "")
	flag.Uint64Var(&opts.packetBufferSize, "packet-buffer-size", 0, "")
	flag.IntVar(&opts.kernelReadBufferSize, "kernel-read-buffer-size", 0, "")
	flag.DurationVar(&opts.kernelFlushTimeout, "kernel-flush-timeout", 0, "")
	flag.Uint64Var(&opts.kernelFlushCount, "kernel-flush-count", 0, "")
}

func main() {
	flag.Parse()

	// TODO(bkeyes): connect to signal handling for graceful shutdown
	ctx := context.Background()

	pipelineOpts, err := pipelineOptionsFromFlags()
	if err != nil {
		die("invalid option: %v", err)
	}

	d, err := daemon.NewDaemon(daemon.Config{
		Mappings: opts.mappings.Values,
		Pipeline: pipelineOpts,
	})
	if err != nil {
		die("%v", err)
	}

	if err := d.Start(ctx); err != nil {
		die("daemon failed to start: %v", err)
	}
}

func pipelineOptionsFromFlags() (daemon.PipelineOptions, error) {
	var p daemon.PipelineOptions

	fieldSet := opts.fields.Value

	if opts.format != "" || fieldSet != message.NoFields {
		if fieldSet == message.NoFields {
			fieldSet = message.AllFields
		}

		switch opts.format {
		case "json":
			p.Format = &message.JSONFormatter{Fields: fieldSet}
		case "text", "":
			p.Format = &message.JSONFormatter{Fields: fieldSet}
		default:
			return p, fmt.Errorf("unknown output format: %q", opts.format)
		}
	}

	if opts.fileMode.Value != 0 {
		p.FileMode = opts.fileMode.Value
	}

	if opts.channelBufferSize != 0 {
		p.ChannelBufferSize = opts.channelBufferSize
	}

	if opts.packetBufferSize != 0 {
		if opts.packetBufferSize > math.MaxUint32 {
			return p, fmt.Errorf("packet buffer size too large: %d", opts.packetBufferSize)
		}
		p.PacketBufferSize = uint32(opts.packetBufferSize)
	}

	if opts.kernelReadBufferSize != 0 {
		p.KernelReadBufferSize = opts.kernelReadBufferSize
	}

	if opts.kernelFlushTimeout != 0 {
		p.KernelFlushTimeout = opts.kernelFlushTimeout
	}

	if opts.kernelFlushCount != 0 {
		if opts.kernelFlushCount > math.MaxUint32 {
			return p, fmt.Errorf("kernel flush count too large: %d", opts.kernelFlushCount)
		}
		p.KernelFlushCount = uint32(opts.kernelFlushCount)
	}

	return p, nil
}
