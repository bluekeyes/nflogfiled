package main

import (
	"context"
	"flag"

	"github.com/bluekeyes/nflogfiled/internal/daemon"
)

var opts struct {
	logLevel int
}

func init() {
	flag.IntVar(&opts.logLevel, "log-level", 0, "")
}

func main() {
	flag.Parse()

	ctx := context.Background()

	d := daemon.NewDaemon(daemon.Config{})
	d.Start(ctx)
}
