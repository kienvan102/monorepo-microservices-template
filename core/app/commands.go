package app

import (
	"context"
	"flag"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kienvan102/monorepo-microservices-template/commonlib/processor"
)

// Commands returns a process that runs one of cmds, chosen by the first
// argument after the process's own flags, like git:
//
//	tool [-config file] [-env-file file] <command> [command flags]
//
// Each command parses its own flags, so 2 commands may declare the same flag
// names; WithPrefix only affects their config here.
func Commands(cmds map[string]Mounted, opts ...ProcessOption) processor.Processor {
	c := &commands{cmds: cmds}
	for _, opt := range opts {
		opt(&c.sources)
	}
	return c
}

type commands struct {
	sources
	cmds map[string]Mounted
	fs   *flag.FlagSet
}

func (c *commands) InitFlags(fs *flag.FlagSet) {
	c.sources.initFlags(fs)
	c.fs = fs
}

func (c *commands) Activate(ctx context.Context) error {
	names := slices.Sorted(maps.Keys(c.cmds))
	args := c.fs.Args()
	if len(args) == 0 {
		return fmt.Errorf("missing command; available: %s", strings.Join(names, ", "))
	}
	cmd, ok := c.cmds[args[0]]
	if !ok {
		return fmt.Errorf("unknown command %q; available: %s", args[0], strings.Join(names, ", "))
	}
	own := flag.NewFlagSet(args[0], flag.ExitOnError)
	cmd.initFlags(own)
	_ = own.Parse(args[1:])

	rt, err := c.runtime()
	if err != nil {
		return err
	}
	run, err := cmd.load(c.sources)
	if err != nil {
		return err
	}
	return run.safely(ctx, rt)
}

func (c *commands) Stop() error { return nil }
