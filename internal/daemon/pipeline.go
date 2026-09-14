package daemon

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/bluekeyes/nflogfiled/internal/logfile"
	"github.com/bluekeyes/nflogfiled/internal/message"
	"github.com/florianl/go-nflog/v2"
)

const (
	StdoutFileName = "-"
)

const (
	DefaultFileMode             = 0o644
	DefaultChannelBufferSize    = 4096
	DefaultPacketBufferSize     = 128
	DefaultKernelReadBufferSize = 2 * 1024 * 1024
	DefaultKernelFlushTimeout   = time.Second
	DefaultKernelFlushCount     = 10
)

const (
	DefaultFieldSet = message.FieldReceivedAt |
		message.FieldGroup |
		message.FieldHook |
		message.FieldPrefix |
		message.FieldInDevice |
		message.FieldOutDevice |
		message.FieldVLANIdentifier |
		message.FieldPacketSrcIP |
		message.FieldPacketSrcPort |
		message.FieldPacketDstIP |
		message.FieldPacketDstPort |
		message.FieldPacketProtocol |
		message.FieldPacketLength |
		message.FieldPacketTCPFlags
)

// A pipeline contains a set of nflog connections, the channel to which they
// send messages, and the output writer that reads from the channel.
type Pipeline struct {
	ctx    context.Context
	cancel context.CancelFunc

	conns []Connection

	writeCancel context.CancelFunc
	writeDone   chan struct{}
	w           *logfile.Writer

	fileName string
	fileMode fs.FileMode
}

type PipelineOptions struct {
	Format   logfile.Formatter
	FileMode fs.FileMode

	ChannelBufferSize    int
	PacketBufferSize     uint32
	KernelReadBufferSize int
	KernelFlushTimeout   time.Duration
	KernelFlushCount     uint32

	NowFunc  func() time.Time
	OpenConn ConnectionFactory
}

func setDefaultOptionsIfMissing(opts *PipelineOptions) {
	if opts.Format == nil {
		opts.Format = &message.TextFormatter{Fields: DefaultFieldSet}
	}
	if opts.FileMode == 0 {
		opts.FileMode = DefaultFileMode
	}

	if opts.ChannelBufferSize <= 0 {
		opts.ChannelBufferSize = DefaultChannelBufferSize
	}
	if opts.PacketBufferSize == 0 {
		opts.PacketBufferSize = DefaultPacketBufferSize
	}
	if opts.KernelReadBufferSize <= 0 {
		opts.KernelReadBufferSize = DefaultKernelReadBufferSize
	}
	if opts.KernelFlushTimeout <= 0 {
		opts.KernelFlushTimeout = DefaultKernelFlushTimeout
	}
	if opts.KernelFlushCount == 0 {
		opts.KernelFlushCount = DefaultKernelFlushCount
	}

	if opts.NowFunc == nil {
		opts.NowFunc = time.Now
	}
	if opts.OpenConn == nil {
		opts.OpenConn = OpenNflogConnection
	}
}

func StartPipeline(ctx context.Context, mapping groups.Mapping, opts PipelineOptions) (*Pipeline, error) {
	setDefaultOptionsIfMissing(&opts)

	ctx, cancel := context.WithCancel(ctx)
	msgs := make(chan message.Message, opts.ChannelBufferSize)

	p := &Pipeline{
		ctx:       ctx,
		cancel:    cancel,
		conns:     make([]Connection, len(mapping.Groups)),
		writeDone: make(chan struct{}),
		w:         logfile.NewWriter(msgs, opts.Format),
		fileName:  mapping.File,
		fileMode:  opts.FileMode,
	}

	for i, g := range mapping.Groups {
		hook := NflogHook(ctx, g, msgs, opts.NowFunc)
		errFunc := NflogErrorFunc(ctx)

		var err error
		p.conns[i], err = opts.OpenConn(ctx, g, hook, errFunc, opts)
		if err != nil {
			p.cleanup()
			return nil, err
		}
	}

	if err := p.startWriter(); err != nil {
		p.cleanup()
		return nil, err
	}

	// Close all connections when the root context is canceled
	go func() {
		<-ctx.Done()
		p.cleanup()
	}()

	return p, nil
}

func (p *Pipeline) ReloadFile() error {
	// Cancel the existing writer and wait for it to finish
	p.writeCancel()
	<-p.writeDone

	return p.startWriter()
}

func (p *Pipeline) startWriter() error {
	var dst io.WriteCloser
	if p.fileName == StdoutFileName {
		dst = logfile.Stdout()
	} else {
		var err error
		dst, err = logfile.Open(p.fileName, p.fileMode)
		if err != nil {
			return fmt.Errorf("opening output file failed: %w", err)
		}
	}

	ctx, cancel := context.WithCancel(p.ctx)
	p.writeCancel = cancel

	go func() {
		p.w.Write(ctx, dst)
		p.writeDone <- struct{}{}
	}()

	return nil
}

func (p *Pipeline) Stop() {
	p.cleanup()
}

func (p *Pipeline) cleanup() {
	p.cancel()
	for _, c := range p.conns {
		if c != nil { // may be called during initialization with a partial connection slice
			_ = c.Close()
		}
	}
}

// Connection represents a connection to a message source.
type Connection interface {
	io.Closer
}

// ConnectionFactory opens a new connection for the specified group that calls
// the appropriate hook on new messages or errors.
type ConnectionFactory func(context.Context, groups.Group, nflog.HookFunc, nflog.ErrorFunc, PipelineOptions) (Connection, error)

func OpenNflogConnection(ctx context.Context, group groups.Group, hook nflog.HookFunc, errFunc nflog.ErrorFunc, opts PipelineOptions) (Connection, error) {
	config := nflog.Config{
		Group:    uint16(group),
		Copymode: nflog.CopyPacket,
		Bufsize:  opts.PacketBufferSize,
		QThresh:  opts.KernelFlushCount,
		Timeout:  uint32(opts.KernelFlushTimeout.Milliseconds() / 10),
		Logger:   nflogLogger{ctx: ctx},
		// Flags: nflog.FlagSeq, // TODO(bkeyes): need to do something with sequence numbers
	}

	nf, err := nflog.Open(&config)
	if err != nil {
		return nil, fmt.Errorf("opening nflog connection failed: %w", err)
	}

	if err := nf.Con.SetReadBuffer(opts.KernelReadBufferSize); err != nil {
		slog.WarnContext(ctx, "setting kernel read buffer size failed, continuing with kernel's default buffer", "error", err)
	}

	if err := nf.RegisterWithErrorFunc(ctx, hook, errFunc); err != nil {
		_ = nf.Close()
		return nil, fmt.Errorf("registering nflog hook failed: %w", err)
	}
	return nf, nil
}
