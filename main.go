package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/cmd"
	"github.com/bluekeyes/nflogfiled/internal/daemon"
	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/bluekeyes/nflogfiled/internal/message"
)

var opts struct {
	logLevel       int
	mappingStrings cmd.StringSliceValue
	format         string
	fileMode       string
	fields         string

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
	flag.Var(&opts.mappingStrings, "map", "")
	flag.StringVar(&opts.format, "format", "", "")
	flag.StringVar(&opts.fileMode, "file-mode", "", "")
	flag.StringVar(&opts.fields, "fields", "", "")

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

	var mappings []groups.Mapping
	for _, v := range opts.mappingStrings.Values {
		m, err := groups.ParseMapping(v)
		if err != nil {
			die("invalid mapping: %v", err)
		}
		mappings = append(mappings, m...)
	}

	pipelineOpts, err := pipelineOptionsFromFlags()
	if err != nil {
		die("invalid option: %v", err)
	}

	d, err := daemon.NewDaemon(daemon.Config{
		Mappings: mappings,
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

	var fieldSet message.FieldSet
	if opts.fields != "" {
		for name := range strings.SplitSeq(opts.fields, ",") {
			name = strings.TrimSpace(name)
			if field, ok := message.FieldsByName[name]; ok {
				fieldSet |= field
			} else {
				return p, fmt.Errorf("unknown output field: %q", name)
			}
		}
	}

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

	if opts.fileMode != "" {
		fileMode, err := strconv.ParseUint(opts.fileMode, 8, 32)
		if err != nil {
			return p, fmt.Errorf("invalid file mode: %q", opts.fileMode)
		}
		p.FileMode = fs.FileMode(fileMode).Perm()
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
