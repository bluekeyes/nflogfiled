package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/cli"
	"github.com/bluekeyes/nflogfiled/internal/daemon"
	"github.com/bluekeyes/nflogfiled/internal/message"
)

var (
	version = "main"
)

type Options struct {
	LogLevel int
	Mapping  cli.GroupMappingValue
	Format   string
	FileMode cli.FileModeValue
	Fields   cli.FieldSetValue

	ChannelBufferSize    int
	PacketBufferSize     uint64
	KernelReadBufferSize int
	KernelFlushTimeout   time.Duration
	KernelFlushCount     uint64
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func main() {
	var opts Options

	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	fs.IntVar(&opts.LogLevel, "log-level", 0, "")
	fs.Var(&opts.Mapping, "map", "")
	fs.StringVar(&opts.Format, "format", "", "")
	fs.Var(&opts.FileMode, "file-mode", "")
	fs.Var(&opts.Fields, "fields", "")
	fs.IntVar(&opts.ChannelBufferSize, "channel-buffer-size", 0, "")
	fs.Uint64Var(&opts.PacketBufferSize, "packet-buffer-size", 0, "")
	fs.IntVar(&opts.KernelReadBufferSize, "kernel-read-buffer-size", 0, "")
	fs.DurationVar(&opts.KernelFlushTimeout, "kernel-flush-timeout", 0, "")
	fs.Uint64Var(&opts.KernelFlushCount, "kernel-flush-count", 0, "")

	var printVersion bool
	fs.BoolVar(&printVersion, "v", false, "version")
	fs.BoolVar(&printVersion, "version", false, "version")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprintln(os.Stdout, helpText())
			os.Exit(0)
		}
		die(err)
	}

	if printVersion {
		fmt.Fprintln(os.Stdout, version)
		os.Exit(0)
	}

	// TODO(bkeyes): configure logging

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	pipelineOpts, err := pipelineOptionsFromFlags(opts)
	if err != nil {
		die(fmt.Errorf("invalid option: %w", err))
	}

	d, err := daemon.NewDaemon(daemon.Config{
		Mappings: opts.Mapping.Values,
		Pipeline: pipelineOpts,
	})
	if err != nil {
		die(err)
	}

	if err := d.Start(ctx); err != nil {
		die(fmt.Errorf("daemon failed to start: %w", err))
	}
}

func pipelineOptionsFromFlags(opts Options) (daemon.PipelineOptions, error) {
	var p daemon.PipelineOptions

	fieldSet := opts.Fields.Value

	if opts.Format != "" || fieldSet != message.NoFields {
		if fieldSet == message.NoFields {
			fieldSet = message.AllFields
		}

		switch opts.Format {
		case "json":
			p.Format = &message.JSONFormatter{Fields: fieldSet}
		case "text", "":
			p.Format = &message.JSONFormatter{Fields: fieldSet}
		default:
			return p, fmt.Errorf("unknown output format: %q", opts.Format)
		}
	}

	if opts.FileMode.Value != 0 {
		p.FileMode = opts.FileMode.Value
	}

	if opts.ChannelBufferSize != 0 {
		p.ChannelBufferSize = opts.ChannelBufferSize
	}

	if opts.PacketBufferSize != 0 {
		if opts.PacketBufferSize > math.MaxUint32 {
			return p, fmt.Errorf("packet buffer size too large: %d", opts.PacketBufferSize)
		}
		p.PacketBufferSize = uint32(opts.PacketBufferSize)
	}

	if opts.KernelReadBufferSize != 0 {
		p.KernelReadBufferSize = opts.KernelReadBufferSize
	}

	if opts.KernelFlushTimeout != 0 {
		p.KernelFlushTimeout = opts.KernelFlushTimeout
	}

	if opts.KernelFlushCount != 0 {
		if opts.KernelFlushCount > math.MaxUint32 {
			return p, fmt.Errorf("kernel flush count too large: %d", opts.KernelFlushCount)
		}
		p.KernelFlushCount = uint32(opts.KernelFlushCount)
	}

	return p, nil
}

func helpText() string {
	defaultFileMode := strconv.FormatInt(daemon.DefaultFileMode, 8)
	defaultChannelBuf := strconv.Itoa(daemon.DefaultChannelBufferSize)
	defaultPacketBuf := strconv.Itoa(daemon.DefaultPacketBufferSize)
	defaultKernelReadBuf := strconv.Itoa(daemon.DefaultKernelReadBufferSize)
	defaultFlushTimeout := daemon.DefaultKernelFlushTimeout.String()
	defaultFlushCount := strconv.Itoa(daemon.DefaultKernelFlushCount)

	// Wrap fields so that they meet the ~82 character limit when indented two
	// spaces past the field description column.
	defaultFields := message.FormatFieldSet(message.AllFields, ", ")
	defaultFields = cli.IndentText(cli.WrapText(defaultFields, 54), 27, false)

	help := `
Usage: nflogfiled [options]

  Run the nflogfiled daemon.

  TODO

Options:

  -map=groups:path       Maps nflog groups to output files. At least one mapping
                         is required, but passing the flag multiple times defines
                         multiple mappings. The 'groups' part of a mapping is a
                         comma-separated list of group numbers and ranges. A group
                         range has the form "start-end" inclusive. The groups are
                         separated from the file path by a colon.

  -format=format         Sets the output format, one of 'text' or 'json'. The
                         default output format is 'text'.

  -file-mode=mode        Sets the file mode for output file that the daemon
                         creates. File modes are given as octal strings. The
                         default file mode is ` + defaultFileMode + `.

  -fields=fields         Sets the nflog fields included in the output. Fields are
                         only printed when they are also present in the message.
                         See the "Fields" section for a list of all fields. The
                         default field list is:

` + defaultFields + `

  -v/-version            Print the version and exit.

  -h/-help               Print this message and exit.

Tuning Options:

  -channel-buffer-size=size       Sets the size of the buffer in messages for
                                  unprocessed nflog messages. The default value
                                  is ` + defaultChannelBuf + `.

  -packet-buffer-size=size        Sets the size of the buffer in bytes for copying
                                  packet data from the kernel to userspace in bytes.
                                  Setting this lower than the default value of ` + defaultPacketBuf + `
                                  may break packet parsing and omit fields.

  -kernel-read-buffer-size=size   Sets the size of the kernel buffer associated
                                  with each nflog connection. The default value is
                                  ` + defaultKernelReadBuf + ` bytes.

  -kernel-flush-timeout=timeout   Sets the maximum time the kernel will wait before
                                  flushing pending nflog message to userspace. The
                                  value is a Go duration string that is truncated
                                  to the closest 1/100 second. Decreasing this may
                                  improve output latency with infrequent messages.
                                  The default value is ` + defaultFlushTimeout + `.

  -kernel-flush-count=count       Sets the maximum number of nflog messages the
                                  kernel will accumulate before flushing them to
                                  userspace. Increasing this may improve perforance
                                  while also increasing output latency. The default
                                  value is ` + defaultFlushCount + `.

Fields:

  TODO

`
	return strings.TrimSpace(help)
}
