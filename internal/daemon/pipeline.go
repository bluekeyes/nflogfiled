package daemon

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/bluekeyes/nflogfiled/internal/logfile"
	"github.com/florianl/go-nflog/v2"
)

const (
	StdoutFileName = "-"
)

const (
	DefaultFileMode          = 0o644
	DefaultChannelBufferSize = 4096
)

// A pipeline contains a set of nflog connections, the channel to which they
// send messages, and the output writer that reads from the channel.
type Pipeline struct {
	ctx    context.Context
	cancel context.CancelFunc

	conns []*nflog.Nflog

	writeCancel context.CancelFunc
	writeDone   chan struct{}
	w           *logfile.Writer

	fileName     string
	fileMode     fs.FileMode
	fileBuffered bool
}

type PipelineOptions struct {
	Format            logfile.FormatFunc
	FileMode          fs.FileMode
	Unbuffered        bool
	ChannelBufferSize int
}

func StartPipeline(ctx context.Context, mapping groups.Mapping, opts PipelineOptions) (*Pipeline, error) {
	fileMode := opts.FileMode
	if fileMode == 0 {
		fileMode = DefaultFileMode
	}

	chanBufSize := opts.ChannelBufferSize
	if chanBufSize <= 0 {
		chanBufSize = DefaultChannelBufferSize
	}

	fmtFn := opts.Format
	if fmtFn == nil {
		// TODO(bkeyes): set a default format functions here
	}

	ctx, cancel := context.WithCancel(ctx)
	msgs := make(chan nflog.Attribute, chanBufSize)

	p := &Pipeline{
		ctx:    ctx,
		cancel: cancel,

		conns: make([]*nflog.Nflog, len(mapping.Groups)),
		w:     logfile.NewWriter(msgs, fmtFn),

		fileName:     mapping.File,
		fileMode:     fileMode,
		fileBuffered: !opts.Unbuffered,
	}

	for i, g := range mapping.Groups {
		var err error
		p.conns[i], err = openNflogConn(ctx, g, msgs)
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
	var f *logfile.File
	if p.fileName == StdoutFileName {
		f = logfile.Stdout(p.fileBuffered)
	} else {
		var err error
		f, err = logfile.Open(p.fileName, p.fileMode, p.fileBuffered)
		if err != nil {
			return fmt.Errorf("opening output file failed: %w", err)
		}
	}

	ctx, cancel := context.WithCancel(p.ctx)
	p.writeCancel = cancel

	go func() {
		p.w.Write(ctx, f)
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

func openNflogConn(ctx context.Context, group groups.Group, msgs chan<- nflog.Attribute) (*nflog.Nflog, error) {
	config := nflog.Config{
		Group:    uint16(group),
		Copymode: nflog.CopyMeta,
	}

	nf, err := nflog.Open(&config)
	if err != nil {
		return nil, fmt.Errorf("opening nflog connection failed: %w", err)
	}

	// TODO(bkeyes): figure out if these are important and what values to use

	// if err := nf.Con.SetReadBuffer(512 * 1024); err != nil {
	// 	fmt.Fprintf(os.Stderr, "failed to set read buffer: %v", err)
	// 	return
	// }

	// if err := nf.SetOption(netlink.NoENOBUFS, true); err != nil {
	// 	fmt.Fprintf(os.Stderr, "failed to set netlink option %v: %v",
	// 		netlink.NoENOBUFS, err)
	// 	return
	// }

	hook := func(attrs nflog.Attribute) int {
		select {
		case msgs <- attrs:
		default:
			slog.WarnContext(ctx, "dropped nflog message, output channel is full")
		}
		return 0
	}

	errFunc := func(err error) int {
		slog.ErrorContext(ctx, "error receiving nflog message", "error", err)
		return 0
	}

	if err := nf.RegisterWithErrorFunc(ctx, hook, errFunc); err != nil {
		_ = nf.Close()
		return nil, fmt.Errorf("registering nflog hook failed: %w", err)
	}
	return nf, nil
}
