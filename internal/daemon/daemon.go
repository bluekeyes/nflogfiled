package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bluekeyes/nflogfiled/internal/groups"
)

const (
	StdoutFile = "-"
)

type Daemon struct {
	mappings     []groups.Mapping
	pipelineOpts PipelineOptions
}

type Config struct {
	Mappings []groups.Mapping
	Pipeline PipelineOptions
}

func NewDaemon(config Config) (*Daemon, error) {
	if err := validateMappings(config.Mappings); err != nil {
		return nil, err
	}

	d := Daemon{
		mappings: config.Mappings,
	}
	return &d, nil
}

// Start starts the daemon and blocks until the context is cancelled. It
// returns an error only for errors that occur during the start process.
func (d *Daemon) Start(ctx context.Context) error {
	pipelines := make([]*Pipeline, len(d.mappings))
	for i, m := range d.mappings {
		var err error
		pipelines[i], err = StartPipeline(ctx, m, d.pipelineOpts)
		if err != nil {
			stopPipelines(pipelines)
			return fmt.Errorf("starting pipeline failed: %w", err)
		}
	}

	reload := make(chan os.Signal, 1)
	signal.Notify(reload, syscall.SIGHUP)

	for {
		select {
		case <-reload:
			for _, p := range pipelines {
				if err := p.ReloadFile(); err != nil {
					slog.ErrorContext(ctx, "error reloading file", "error", err)
				}
			}

		case <-ctx.Done():
			return nil
		}
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

func stopPipelines(ps []*Pipeline) {
	for _, p := range ps {
		if p != nil {
			p.Stop()
		}
	}
}
