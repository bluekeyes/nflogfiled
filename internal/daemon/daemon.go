package daemon

import "context"

type Daemon struct {
}

type Config struct {
}

func NewDaemon(config Config) *Daemon {
	return &Daemon{}
}

// Start starts the daemon and blocks until the context is cancelled. It
// returns an error only for errors that occur during the start process.
func (d *Daemon) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
