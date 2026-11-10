package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"sync"

	"github.com/kienvan102/monorepo-microservices-template/commonlib/processor"
)

var errComponentFailed = errors.New("another component failed")

// Parallel returns a process that runs all parts at the same time. With a single
// part, that's simply the process running that one component. A panic in a
// part's Run is turned into an error. If one part fails, the others' context
// is cancelled, and the context.Canceled errors they return because of it are
// left out of the result; the process ends when all parts have returned.
// ProcessOptions such as WithLogger can be passed among the parts.
func Parallel(parts ...Part) processor.Processor {
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
			name := part.flagName(f.Name)
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
		return runs[0].safely(ctx, rt)
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	errs := make([]error, len(runs))
	first := -1
	var once sync.Once
	var wg sync.WaitGroup
	for i, run := range runs {
		wg.Go(func() {
			if errs[i] = run.safely(ctx, rt); errs[i] != nil {
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

func (p *parallel) Stop() error { return nil }
