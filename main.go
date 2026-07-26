package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/bluekeyes/nflogfiled/internal/cmd"
	"github.com/bluekeyes/nflogfiled/internal/daemon"
	"github.com/bluekeyes/nflogfiled/internal/groups"
)

var opts struct {
	logLevel       int
	mappingStrings cmd.StringSliceValue
}

func die(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+msg+"\n", args...)
	os.Exit(1)
}

func init() {
	flag.IntVar(&opts.logLevel, "log-level", 0, "")
	flag.Var(&opts.mappingStrings, "map", "")
}

func main() {
	flag.Parse()

	// TODO(bkeyes): connect to signal handling for graceful shutdown
	ctx := context.Background()

	var mappings []groups.Mapping
	for _, v := range opts.mappingStrings.Values {
		m, err := groups.ParseMapping(v)
		if err != nil {
			die("invalid mapping: %v", err)
		}
		mappings = append(mappings, m...)
	}

	d, err := daemon.NewDaemon(daemon.Config{
		Mappings: mappings,
	})
	if err != nil {
		die("%v", err)
	}

	if err := d.Start(ctx); err != nil {
		die("daemon failed to start: %v", err)
	}
}
