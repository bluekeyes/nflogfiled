package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/groups"
)

const (
	StdoutFile = "-"
)

const (
	DefaultDrainTimeout = 5 * time.Second
)

type Daemon struct {
	mappings     []groups.Mapping
	pipelineOpts PipelineOptions
	reload       chan struct{}
	drainTimeout time.Duration
}

type Config struct {
	Mappings     []groups.Mapping
	Pipeline     PipelineOptions
	DrainTimeout time.Duration
}

func NewDaemon(config Config) (*Daemon, error) {
	if err := validateMappings(config.Mappings); err != nil {
		return nil, err
	}

	drainTimeout := config.DrainTimeout
	if drainTimeout <= 0 {
		drainTimeout = DefaultDrainTimeout
	}

	d := Daemon{
		mappings:     config.Mappings,
		pipelineOpts: config.Pipeline,
		reload:       make(chan struct{}, 1),
		drainTimeout: drainTimeout,
	}
	return &d, nil
}

// Start starts the daemon and blocks until the context is cancelled. It
// returns an error only for errors that occur during the start process.
func (d *Daemon) Start(ctx context.Context) error {
	pipelines := make([]*Pipeline, len(d.mappings))
	for i, m := range d.mappings {
		var err error
		pipelines[i], err = StartPipeline(m, d.pipelineOpts)
		if err != nil {
			stopPipelines(pipelines, d.drainTimeout)
			return fmt.Errorf("starting pipeline failed: %w", err)
		}
	}

	for {
		select {
		case <-d.reload:
			for _, p := range pipelines {
				if err := p.ReloadFile(); err != nil {
					slog.ErrorContext(ctx, "error reloading file", "error", err)
				}
			}

		case <-ctx.Done():
			stopPipelines(pipelines, d.drainTimeout)
			return nil
		}
	}
}

// Reload tells the daemon to reopen all files.
func (d *Daemon) Reload() {
	select {
	case d.reload <- struct{}{}:
	default:
	}
}

func validateMappings(mappings []groups.Mapping) error {
	allGroups := make(map[groups.Group]bool)
	allFiles := make(map[string]bool)

	if len(mappings) == 0 {
		return fmt.Errorf("the daemon must have at least one group mapping")
	}

	for _, m := range mappings {
		if len(m.Groups) == 0 {
			return fmt.Errorf("mapping defines no groups")
		}
		if m.File == "" {
			return fmt.Errorf("mapping defines no file")
		}

		for _, g := range m.Groups {
			if allGroups[g] {
				return fmt.Errorf("group %d appears in multiple mappings", g)
			}
			allGroups[g] = true
		}

		if allFiles[m.File] {
			return fmt.Errorf("file %q appears in multiple mappings", m.File)
		}
		allFiles[m.File] = true
	}

	return nil
}

func stopPipelines(ps []*Pipeline, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var wg sync.WaitGroup
	for _, p := range ps {
		if p != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p.Stop(ctx)
			}()
		}
	}

	wg.Wait()
}
