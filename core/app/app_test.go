package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kienvan102/monorepo-microservices-template/core/logger"
)

type fakeConfig struct {
	URI string `env:"APPTEST_URI" envDefault:"default"`
}

// fake records what the framework gave it.
type fake struct {
	flagName string
	run      func(ctx context.Context) error

	flagValue string
	rt        Runtime
	cfg       fakeConfig
	ran       bool
}

func (f *fake) InitFlags(fs *flag.FlagSet) {
	if f.flagName != "" {
		fs.StringVar(&f.flagValue, f.flagName, "", "test flag")
	}
}

func (f *fake) Run(ctx context.Context, rt Runtime, cfg fakeConfig) error {
	f.ran, f.rt, f.cfg = true, rt, cfg
	if f.run != nil {
		return f.run(ctx)
	}
	return nil
}

// start runs InitFlags + parse + Activate, like processor.Main without exiting.
func start(t *testing.T, p interface {
	InitFlags(*flag.FlagSet)
	Activate(context.Context) error
}, args ...string) error {
	t.Helper()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	p.InitFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return p.Activate(context.Background())
}

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewSingleComponent(t *testing.T) {
	yaml := writeYAML(t, "appEnv: staging\napptest:\n  uri: from-yaml\n")
	c := &fake{flagName: "action"}

	if err := start(t, New(Mount[fakeConfig](c)), "-config", yaml, "-env-file=", "-action", "check"); err != nil {
		t.Fatal(err)
	}
	if !c.ran || c.flagValue != "check" {
		t.Errorf("ran=%v flag=%q, want ran with flag %q", c.ran, c.flagValue, "check")
	}
	if c.rt.AppEnv != "staging" || c.rt.Log == nil {
		t.Errorf("runtime = %+v, want AppEnv staging and a logger", c.rt)
	}
	if c.cfg.URI != "from-yaml" {
		t.Errorf("cfg.URI = %q, want %q", c.cfg.URI, "from-yaml")
	}
}

func TestNewPrefix(t *testing.T) {
	yaml := writeYAML(t, "appEnv: production\napptest:\n  uri: plain\nx:\n  appEnv: ignored\n  apptest:\n    uri: prefixed\n")
	plain := &fake{flagName: "action"}
	prefixed := &fake{flagName: "action"}
	p := New(Mount[fakeConfig](plain), Mount[fakeConfig](prefixed, WithPrefix("x")))

	if err := start(t, p, "-config", yaml, "-env-file=", "-action", "a", "-x-action", "b"); err != nil {
		t.Fatal(err)
	}
	if plain.flagValue != "a" || prefixed.flagValue != "b" {
		t.Errorf("flags: plain=%q prefixed=%q, want a and b", plain.flagValue, prefixed.flagValue)
	}
	if plain.cfg.URI != "plain" || prefixed.cfg.URI != "prefixed" {
		t.Errorf("config: plain=%q prefixed=%q", plain.cfg.URI, prefixed.cfg.URI)
	}
	if prefixed.rt.AppEnv != "production" {
		t.Errorf("APP_ENV must never be prefixed: got %q", prefixed.rt.AppEnv)
	}
}

func TestNewDuplicateFlag(t *testing.T) {
	a := &fake{flagName: "action"}
	b := &fake{flagName: "action"}
	err := start(t, New(Mount[fakeConfig](a), Mount[fakeConfig](b)), "-config=", "-env-file=")
	if err == nil || !strings.Contains(err.Error(), "-action") {
		t.Fatalf("want a duplicate flag error naming -action, got %v", err)
	}
	if a.ran || b.ran {
		t.Error("no component may run when flags collide")
	}
}

func TestNewCancelsOthersOnError(t *testing.T) {
	boom := errors.New("boom")
	failing := &fake{run: func(context.Context) error { return boom }}
	var sawCancel bool
	longRunning := &fake{run: func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			sawCancel = true
			return nil
		case <-time.After(5 * time.Second):
			return errors.New("not cancelled")
		}
	}}

	err := start(t, New(Mount[fakeConfig](failing), Mount[fakeConfig](longRunning, WithPrefix("b"))), "-config=", "-env-file=")
	if !errors.Is(err, boom) {
		t.Fatalf("want the failing component's error, got %v", err)
	}
	if !sawCancel {
		t.Error("the long-running component's context was not cancelled")
	}
}

func TestNewRecoversPanic(t *testing.T) {
	panicking := &fake{run: func(context.Context) error { panic("nil map") }}
	var sawCancel bool
	longRunning := &fake{run: func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			sawCancel = true
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return errors.New("not cancelled")
		}
	}}

	err := start(t, New(Mount[fakeConfig](panicking), Mount[fakeConfig](longRunning, WithPrefix("b"))), "-config=", "-env-file=")
	if err == nil || !strings.Contains(err.Error(), "component panicked: nil map") {
		t.Fatalf("want the panic as an error, got %v", err)
	}
	if !sawCancel {
		t.Error("the long-running component's context was not cancelled")
	}
	if errors.Is(err, context.Canceled) {
		t.Errorf("the cancelled component's context.Canceled must be left out, got %v", err)
	}
}

func TestNewSingleComponentPanic(t *testing.T) {
	c := &fake{run: func(context.Context) error { panic("boom") }}
	err := start(t, New(Mount[fakeConfig](c)), "-config=", "-env-file=")
	if err == nil || !strings.Contains(err.Error(), "component panicked: boom") {
		t.Fatalf("want the panic as an error, got %v", err)
	}
}

func TestNewDropsCancelledSiblings(t *testing.T) {
	boom := errors.New("boom")
	failing := &fake{run: func(context.Context) error { return boom }}
	stopped := &fake{run: func(ctx context.Context) error {
		<-ctx.Done()
		return fmt.Errorf("worker: %w", ctx.Err())
	}}

	err := start(t, New(Mount[fakeConfig](failing), Mount[fakeConfig](stopped, WithPrefix("b"))), "-config=", "-env-file=")
	if !errors.Is(err, boom) || errors.Is(err, context.Canceled) {
		t.Fatalf("want only the failing component's error, got %v", err)
	}
}

func TestNewKeepsParentCancellation(t *testing.T) {
	stopped := func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}
	p := New(Mount[fakeConfig](&fake{run: stopped}), Mount[fakeConfig](&fake{run: stopped}, WithPrefix("b")))
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	p.InitFlags(fs)
	if err := fs.Parse([]string{"-config=", "-env-file="}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Activate(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled when the process itself is stopping, got %v", err)
	}
}

func TestCommandsRecoversPanic(t *testing.T) {
	c := &fake{run: func(context.Context) error { panic("boom") }}
	err := start(t, Commands(map[string]Mounted{"alpha": Mount[fakeConfig](c)}), "-config=", "-env-file=", "alpha")
	if err == nil || !strings.Contains(err.Error(), "component panicked: boom") {
		t.Fatalf("want the panic as an error, got %v", err)
	}
}

func TestCommands(t *testing.T) {
	newCmds := func() (*fake, *fake, map[string]Mounted) {
		a := &fake{flagName: "action"}
		b := &fake{flagName: "action"}
		return a, b, map[string]Mounted{"alpha": Mount[fakeConfig](a), "beta": Mount[fakeConfig](b)}
	}

	a, b, cmds := newCmds()
	if err := start(t, Commands(cmds), "-config=", "-env-file=", "beta", "-action", "run"); err != nil {
		t.Fatal(err)
	}
	if a.ran || !b.ran || b.flagValue != "run" {
		t.Errorf("alpha ran=%v, beta ran=%v flag=%q; want only beta with flag run", a.ran, b.ran, b.flagValue)
	}

	for _, args := range [][]string{{}, {"gamma"}} {
		_, _, cmds := newCmds()
		err := start(t, Commands(cmds), append([]string{"-config=", "-env-file="}, args...)...)
		if err == nil || !strings.Contains(err.Error(), "alpha, beta") {
			t.Errorf("args %v: want an error listing the commands, got %v", args, err)
		}
	}
}

func TestWithLoggerReachesRuntime(t *testing.T) {
	var newBuf, cmdBuf bytes.Buffer
	a := &fake{}
	b := &fake{}
	jsonTo := func(buf *bytes.Buffer) ProcessOption {
		return WithLogger(logger.WithOutput(buf), logger.WithFormat(logger.FormatJSON))
	}

	if err := start(t, New(Mount[fakeConfig](a), jsonTo(&newBuf)), "-config=", "-env-file="); err != nil {
		t.Fatal(err)
	}
	if err := start(t, Commands(map[string]Mounted{"beta": Mount[fakeConfig](b)}, jsonTo(&cmdBuf)), "-config=", "-env-file=", "beta"); err != nil {
		t.Fatal(err)
	}

	for name, c := range map[string]struct {
		f   *fake
		buf *bytes.Buffer
	}{"New": {a, &newBuf}, "Commands": {b, &cmdBuf}} {
		c.f.rt.Log.Debug("hello")
		if !json.Valid(c.buf.Bytes()) || !strings.Contains(c.buf.String(), `"message":"hello"`) {
			t.Errorf("%s: want the JSON entry in the WithLogger output at the dev default level, got %q", name, c.buf.String())
		}
	}
}
