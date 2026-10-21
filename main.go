package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/cli"
	"github.com/bluekeyes/nflogfiled/internal/daemon"
	"github.com/bluekeyes/nflogfiled/internal/message"
)

var (
	version = "main"
)

type Options struct {
	LogLevel cli.LogLevelValue
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

	fs.Var(&opts.LogLevel, "log-level", "")
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

	configureLogging(opts)

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

	reload := make(chan os.Signal, 1)
	signal.Notify(reload, syscall.SIGHUP)

	go func() {
		for {
			select {
			case <-reload:
				d.Reload()
			case <-ctx.Done():
				return
			}
		}
	}()

	if err := d.Start(ctx); err != nil {
		die(fmt.Errorf("daemon failed to start: %w", err))
	}
}

func configureLogging(opts Options) {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: opts.LogLevel.Value,
	})

	logger := slog.New(handler)

	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(slog.LevelInfo)
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
	defaultFields := message.FormatFieldSet(daemon.DefaultFieldSet, ", ")
	defaultFields = cli.IndentText(cli.WrapText(defaultFields, 54), 27, false)

	help := `
Usage: nflogfiled [options]

  Run the nflogfiled daemon.

  The daemon listens for nflog messages sent to specific groups, formats them,
  and writes the results to one or more files. It provides more control than
  the built-in output of a tool like nftables while being simpler than a tool
  like ulogd.

  On startup, the daemon creates any output files that do not exist, including
  their parent directories if needed. Send the daemon process the SIGHUP signal
  to reopen files, e.g. after log rotation.

Options:

  -map=groups:path       Maps nflog groups to output files. At least one mapping
                         is required, but passing the flag multiple times defines
                         multiple mappings. The 'groups' part of a mapping is a
                         comma-separated list of group numbers and ranges. A group
                         range has the form "start-end" inclusive. The groups are
                         separated from the file path by a colon. Use '-' as the
                         path to write to standard out.

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

  -log-level=level       Sets the daemon's log level. One of 'debug', 'info',
                         'warn', or 'error'. Logs write to standard error.

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

  General Fields

    group                nflog group that logged the message
    received_at          time when the daemon received the message
    hook                 netfilter hook that logged the message
    mark                 netfilter mark value on the message
    prefix               prefix string set by the nflog rule
    uid                  user ID of the associated local socket
    gid                  group ID of the associated local socket
    in_device            input interface index
    physical_in_device   physical input interface index
    out_device           output interface index
    physical_out_device  physical output interface index
    vlan_protocol        VLAN tag protocol (EtherType)
    vlan_identifier      numeric VLAN ID

  Packet Fields

    packet_src_ip        source IP address
    packet_src_port      source port (TCP or UDP)
    packet_src_mac       source MAC address
    packet_dst_ip        destination IP address
    packet_dst_port      destination port (TCP or UDP)
    packet_dst_mac       destination MAC address
    packet_ttl           IPv4 TTL or IPv6 hop limit
    packet_protocol      IP protocol name or number
    packet_length        total IP packet length in bytes

  TCP Fields

    packet_tcp_seq       TCP sequence number
    packet_tcp_ack       TCP acknowledgement number
    packet_tcp_flags     set TCP flag names
    packet_tcp_window    TCP window size

`
	return strings.TrimSpace(help)
}
