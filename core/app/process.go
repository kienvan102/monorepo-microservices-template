package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"maps"
	"runtime/debug"
	"slices"
	"strings"
	"sync"

	"github.com/kienvan102/monorepo-microservices-template/commonlib/processor"
)

var errComponentFailed = errors.New("another component failed")

// New returns a process that runs all parts at the same time. With a single
// part, that's simply the process running that one component. A panic in a
// part's Run is turned into an error. If one part fails, the others' context
// is cancelled, and the context.Canceled errors they return because of it are
// left out of the result; the process ends when all parts have returned.
// ProcessOptions such as WithLogger can be passed among the parts.
func New(parts ...Part) processor.Processor {
	p := &parallel{}
	for _, part := range parts {
		switch v := part.(type) {
		case Mounted:
			p.parts = append(p.parts, v)
		case ProcessOption:
			v(&p.sources)
		}
	}
	return p
}

type parallel struct {
	sources
	parts   []Mounted
	flagErr error
}

func (p *parallel) InitFlags(fs *flag.FlagSet) {
	p.sources.initFlags(fs)
	for _, part := range p.parts {
		own := flag.NewFlagSet("", flag.ContinueOnError)
		part.initFlags(own)
		own.VisitAll(func(f *flag.Flag) {
			name := flagName(part.prefix, f.Name)
			if fs.Lookup(name) != nil {
				p.flagErr = errors.Join(p.flagErr, fmt.Errorf("flag -%s is declared more than once; mount one of the components WithPrefix", name))
				return
			}
			fs.Var(f.Value, name, f.Usage)
		})
	}
}

func (p *parallel) Activate(ctx context.Context) error {
	if p.flagErr != nil {
		return p.flagErr
	}
	rt, err := p.runtime()
	if err != nil {
		return err
	}
	runs := make([]runFunc, len(p.parts))
	for i, part := range p.parts {
		if runs[i], err = part.load(p.sources); err != nil {
			return err
		}
	}
	if len(runs) == 1 {
		return safeRun(ctx, rt, runs[0])
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	errs := make([]error, len(runs))
	first := -1
	var once sync.Once
	var wg sync.WaitGroup
	for i, run := range runs {
		wg.Go(func() {
			if errs[i] = safeRun(ctx, rt, run); errs[i] != nil {
				once.Do(func() {
					first = i
					cancel(errComponentFailed)
				})
			}
		})
	}
	wg.Wait()
	if errors.Is(context.Cause(ctx), errComponentFailed) {
		for i, err := range errs {
			if i != first && errors.Is(err, context.Canceled) {
				errs[i] = nil
			}
		}
	}
	return errors.Join(errs...)
}

func safeRun(ctx context.Context, rt Runtime, run runFunc) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("component panicked: %v\n%s", r, debug.Stack())
		}
	}()
	return run(ctx, rt)
}

func (p *parallel) Stop() error { return nil }

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
	return safeRun(ctx, rt, run)
}

func (c *commands) Stop() error { return nil }
