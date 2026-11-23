package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/cli"
	"github.com/bluekeyes/nflogfiled/internal/daemon"
	"github.com/bluekeyes/nflogfiled/internal/message"
	"github.com/bluekeyes/nflogfiled/internal/metrics"
	"github.com/bluekeyes/nflogfiled/internal/version"
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

	DeviceNameTTL time.Duration

	MetricsAddress string
	MetricsPath    string
}

const (
	DefaultMetricsPath     = "/metrics"
	MetricsShutdownTimeout = 5 * time.Second
)

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

	fs.DurationVar(&opts.DeviceNameTTL, "device-name-ttl", 0, "")

	fs.StringVar(&opts.MetricsAddress, "metrics-address", "", "")
	fs.StringVar(&opts.MetricsPath, "metrics-path", "", "")

	var printVersion bool
	fs.BoolVar(&printVersion, "v", false, "version")
	fs.BoolVar(&printVersion, "version", false, "version")

	args, err := cli.ExpandArgs(os.Args[1:])
	if err != nil {
		die(err)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprintln(os.Stdout, helpText())
			os.Exit(0)
		}
		die(err)
	}

	if fs.NArg() > 0 {
		die(fmt.Errorf("unexpected argument: %q", fs.Arg(0)))
	}

	if printVersion {
		fmt.Fprintln(os.Stdout, version.Version())
		os.Exit(0)
	}

	configureLogging(opts)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pipelineOpts, err := pipelineOptionsFromFlags(ctx, opts)
	if err != nil {
		die(fmt.Errorf("invalid option: %w", err))
	}

	if opts.MetricsAddress != "" {
		m := metrics.New()
		pipelineOpts.Metrics = m

		stop, err := startMetricsServer(ctx, opts, m)
		if err != nil {
			die(err)
		}
		defer stop()
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

// startMetricsServer starts the Prometheus HTTP server in the background and
// returns a function that gracefully stops the server. The stop function
// blocks until all requests are complete.
func startMetricsServer(ctx context.Context, opts Options, m *metrics.Metrics) (stop func(), err error) {
	path := opts.MetricsPath
	if path == "" {
		path = DefaultMetricsPath
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("invalid option: metrics path must start with '/': %q", path)
	}

	l, err := net.Listen("tcp", opts.MetricsAddress)
	if err != nil {
		return nil, fmt.Errorf("metrics server failed to listen: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle(path, m.Handler())

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	slog.InfoContext(ctx, "serving metrics", "address", l.Addr().String(), "path", path)
	go func() {
		if err := srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, "metrics server failed", "error", err)
		}
	}()

	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), MetricsShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Warn("error stopping metrics server", "error", err)
		}
	}, nil
}

func configureLogging(opts Options) {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: opts.LogLevel.Value,
	})

	logger := slog.New(handler)

	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(slog.LevelInfo)
}

func pipelineOptionsFromFlags(ctx context.Context, opts Options) (daemon.PipelineOptions, error) {
	var p daemon.PipelineOptions

	fieldSet := opts.Fields.Value
	if fieldSet == message.NoFields {
		fieldSet = message.AllFields
	}

	deviceNameTTL := daemon.DefaultDeviceNameTTL
	if opts.DeviceNameTTL != 0 {
		deviceNameTTL = opts.DeviceNameTTL
	}

	p.DeviceNamer = message.NewListingDeviceNamer(ctx, deviceNameTTL)

	fmtOpts := []message.FormatOption{
		message.WithFields(fieldSet),
	}

	switch opts.Format {
	case "json":
		p.Format = message.NewJSONFormatter(fmtOpts...)
	case "text", "":
		p.Format = message.NewTextFormatter(fmtOpts...)
	default:
		return p, fmt.Errorf("unknown output format: %q", opts.Format)
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
	defaultDeviceNameTTL := daemon.DefaultDeviceNameTTL.String()

	// Wrap fields so that they meet the ~82 character limit when indented two
	// spaces past the field description column.
	defaultFields := message.FormatFieldSet(daemon.DefaultFieldSet, ", ")
	defaultFields = cli.IndentText(cli.WrapText(defaultFields, 54), 27, false)

	help := `
Usage: nflogfiled [options] [@file ...]

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

  -device-name-ttl=duration       Sets approximate time the daemon caches device
                                  index to name mappings. The default value is ` + defaultDeviceNameTTL + `.

Metrics Options:

  -metrics-address=addr  Enables a Prometheus metrics endpoint that listens on the
                         given address in 'host:port' form, e.g. '127.0.0.1:9880'.
                         Metrics are disabled by default. The endpoint uses plain
                         HTTP without authentication, so bind it to a trusted
                         address.

  -metrics-path=path     Sets the HTTP path of the metrics endpoint. The default
                         path is '` + DefaultMetricsPath + `'.

Flag Files:

  A positional argument that starts with '@' indicates a flag file. The remainder
  of the argument gives a path to a file containing one flag per line. After
  removing empty lines and lines that start with '#', the remaining lines take the
  place of the flag file argument in the command.

  Flags that take values must use the '-flag=value' form and a flag file cannot
  include other flag files.

Fields:

  General Fields

    group                nflog group that logged the message
    received_at          time when the daemon received the message
    hook                 netfilter hook that logged the message
    mark                 netfilter mark value on the message
    prefix               prefix string set by the nflog rule
    uid                  user ID of the associated local socket
    gid                  group ID of the associated local socket
    in_device            input interface name or index
    physical_in_device   physical input interface name or index
    out_device           output interface name or index
    physical_out_device  physical output interface name or index
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
