package daemon

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
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

	w       *logfile.Writer
	dstChan chan io.WriteCloser

	fileName string
	fileMode fs.FileMode
	openFunc OpenFunc
}

type PipelineOptions struct {
	Format   logfile.Formatter
	FileMode fs.FileMode

	ChannelBufferSize    int
	PacketBufferSize     uint32
	KernelReadBufferSize int
	KernelFlushTimeout   time.Duration
	KernelFlushCount     uint32

	// Options to modify pipeline behavior for testing. If nil, all use
	// appropriate default functions.
	NowFunc     func() time.Time
	ConnectFunc ConnectFunc
	OpenFunc    OpenFunc
}

// Connection represents a connection to a message source.
type Connection interface {
	io.Closer
}

// ConnectFunc opens a new connection for the specified group that calls the
// appropriate hook on new messages or errors.
type ConnectFunc func(context.Context, groups.Group, nflog.HookFunc, nflog.ErrorFunc, PipelineOptions) (Connection, error)

// OpenFunc returns writer for the given path with the given file mode.
type OpenFunc func(string, os.FileMode) (io.WriteCloser, error)

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
	if opts.ConnectFunc == nil {
		opts.ConnectFunc = OpenNflogConnection
	}
	if opts.OpenFunc == nil {
		opts.OpenFunc = logfile.Open
	}
}

func StartPipeline(ctx context.Context, mapping groups.Mapping, opts PipelineOptions) (*Pipeline, error) {
	setDefaultOptionsIfMissing(&opts)

	ctx, cancel := context.WithCancel(ctx)
	msgs := make(chan message.Message, opts.ChannelBufferSize)

	p := &Pipeline{
		ctx:      ctx,
		cancel:   cancel,
		conns:    make([]Connection, len(mapping.Groups)),
		dstChan:  make(chan io.WriteCloser),
		fileName: mapping.File,
		fileMode: opts.FileMode,
		openFunc: opts.OpenFunc,
	}

	dst, err := p.openDestination()
	if err != nil {
		return nil, err
	}
	p.w = logfile.NewWriter(dst, opts.Format)

	for i, g := range mapping.Groups {
		hook := NflogHook(ctx, g, msgs, opts.NowFunc)
		errFunc := NflogErrorFunc(ctx)

		var err error
		p.conns[i], err = opts.ConnectFunc(ctx, g, hook, errFunc, opts)
		if err != nil {
			p.cleanup()
			return nil, err
		}
	}

	go p.writeForever(msgs)

	// Close all connections when the root context is canceled
	go func() {
		<-ctx.Done()
		p.cleanup()
	}()

	return p, nil
}

func (p *Pipeline) ReloadFile() error {
	dst, err := p.openDestination()
	if err != nil {
		return err
	}

	select {
	case p.dstChan <- dst:
	case <-p.ctx.Done():
		_ = dst.Close()
	}
	return nil
}

func (p *Pipeline) openDestination() (io.WriteCloser, error) {
	if p.fileName == StdoutFileName {
		return logfile.Stdout(), nil
	}

	dst, err := p.openFunc(p.fileName, p.fileMode)
	if err != nil {
		return nil, fmt.Errorf("opening output file failed: %w", err)
	}
	return dst, err
}

func (p *Pipeline) writeForever(msgs <-chan message.Message) {
	for {
		select {
		case msg := <-msgs:
			p.w.Write(p.ctx, msg)

		case dst := <-p.dstChan:
			p.w.Reset(p.ctx, dst)

		case <-p.ctx.Done():
			return
		}
	}
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
