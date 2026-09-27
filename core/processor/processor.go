// Package processor defines the lifecycle every deployment's process follows
// and runs it, so each cmd/<name>/main.go is just processor.Main(New()).
package processor

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

type Processor interface {
	// InitFlags registers the process's flags on fs; called before parsing.
	InitFlags(fs *flag.FlagSet)
	// Activate loads the deployment's config and runs the process.
	Activate(ctx context.Context) error
	// Stop releases what Activate acquired; called even if Activate failed.
	Stop() error
}

type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit status %d", e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }

// Main runs p's lifecycle: InitFlags, parse the command line, Activate, Stop.
// A non-nil error is printed to stderr and sets the exit status:
//   - the Code of an *ExitError in the error chain,
//   - 128+signal (130 for SIGINT, 143 for SIGTERM) when a stop signal arrived
//     and the error wraps context.Canceled: the work was interrupted,
//   - 1 otherwise.
//
// The flag set behaves like the standard library's default one: a bad flag
// prints usage and exits with status 2, -h exits with status 0.
//
// SIGINT/SIGTERM cancel the context passed to Activate, with the signal as
// the context's cause, so the process can stop its work (and any child
// process) cleanly instead of being killed mid-way. A second signal falls
// back to the default: exit immediately.
func Main(p Processor) {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	p.InitFlags(fs)
	_ = fs.Parse(os.Args[1:])
	ctx, received, release := notifyStop(context.Background())
	defer release()
	err := p.Activate(ctx)
	if stopErr := p.Stop(); err == nil {
		err = stopErr
	}
	if err == nil {
		return
	}
	message, code := outcome(err, received())
	fmt.Fprintln(os.Stderr, message)
	os.Exit(code)
}

type stopSignal struct {
	sig os.Signal
}

func (s stopSignal) Error() string { return "received " + s.sig.String() }

func notifyStop(parent context.Context) (context.Context, func() os.Signal, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case s := <-signals:
			signal.Stop(signals)
			cancel(stopSignal{sig: s})
		case <-ctx.Done():
		}
	}()
	received := func() os.Signal {
		var s stopSignal
		if errors.As(context.Cause(ctx), &s) {
			return s.sig
		}
		return nil
	}
	release := func() {
		signal.Stop(signals)
		cancel(nil)
	}
	return ctx, received, release
}

func outcome(err error, sig os.Signal) (string, int) {
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return err.Error(), exitErr.Code
	}
	if s, ok := sig.(syscall.Signal); ok && errors.Is(err, context.Canceled) {
		return fmt.Sprintf("stopped by %s before finishing: %v", signalName(s), err), 128 + int(s)
	}
	return err.Error(), 1
}

func signalName(s syscall.Signal) string {
	switch s {
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGTERM:
		return "SIGTERM"
	default:
		return s.String()
	}
}
