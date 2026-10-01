// Package app is the process framework every deployment is built from. A
// deployment picks how its process runs (all components together with New,
// or one chosen command with Commands) and mounts the components its
// services provide; app does the rest: -config/-env-file flags, config
// loading, APP_ENV and the logger, and running the components.
package app

import (
	"context"
	"flag"
	"fmt"

	"github.com/kienvan102/monorepo-microservices-template/commonlib/config"
	"github.com/kienvan102/monorepo-microservices-template/commonlib/logger"
)

// Runtime is what every component receives from the process, whatever
// service it belongs to.
type Runtime struct {
	// AppEnv is the process's environment (APP_ENV: dev, testing, staging,
	// production). It is process-wide and never prefixed.
	AppEnv string
	Log    logger.Logger
}

// Component is what a service's transport (CLI, HTTP, worker...) provides
// to run inside a process. C is the service's own config type.
//
// Run decides how long it lives: a run-once component returns when its work
// is done; a long-running one returns once ctx is cancelled (on SIGINT or
// SIGTERM, or when another component of the process fails).
//
// What Run returns is how the process ends. nil is success, including a
// long-running component that stopped cleanly because ctx was cancelled. An
// error means the work failed or was left unfinished; wrap ctx.Err() with %w
// so an interrupted run exits as interrupted (130 or 143) rather than failed
// (1). A *processor.ExitError chooses the exit status.
type Component[C any] interface {
	InitFlags(fs *flag.FlagSet)
	Run(ctx context.Context, rt Runtime, cfg C) error
}

// MountOption adjusts how a component is mounted.
type MountOption func(*Mounted)

// WithPrefix makes the component read its config with the given prefix
// (see config.WithPrefix: "x" reads X_... variables, YAML key x:) and, when
// mounted with New, declare its flags as -x-<flag>. Use it when 2 mounted
// components would otherwise read the same variable or flag name.
func WithPrefix(name string) MountOption {
	return func(m *Mounted) { m.prefix = name }
}

// Mounted is a component ready to be run by New or Commands, with its
// config type hidden so components of different services can be combined.
type Mounted struct {
	prefix    string
	initFlags func(fs *flag.FlagSet)
	load      func(files sources) (runFunc, error)
}

type runFunc func(ctx context.Context, rt Runtime) error

// Mount prepares c to be run by New or Commands.
func Mount[C any](c Component[C], opts ...MountOption) Mounted {
	m := Mounted{initFlags: c.InitFlags}
	for _, opt := range opts {
		opt(&m)
	}
	var configOpts []config.Option
	if m.prefix != "" {
		configOpts = append(configOpts, config.WithPrefix(m.prefix))
	}
	m.load = func(files sources) (runFunc, error) {
		cfg, err := config.Load[C](files.yaml, files.env, configOpts...)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, rt Runtime) error { return c.Run(ctx, rt, cfg) }, nil
	}
	return m
}

// Part is what New takes: a Mounted component or a ProcessOption.
type Part interface {
	part()
}

func (Mounted) part() {}

// ProcessOption adjusts the process itself rather than one component. Pass
// it to New among the components, or to Commands after the commands.
type ProcessOption func(*sources)

func (ProcessOption) part() {}

// WithLogger passes opts to the logger every component receives as
// Runtime.Log. Whatever opts leave unset keeps its APP_ENV default.
func WithLogger(opts ...logger.Option) ProcessOption {
	return func(s *sources) { s.logOpts = append(s.logOpts, opts...) }
}

// sources are the config files named by -config and -env-file, and the
// process options that shape the Runtime built from them.
type sources struct {
	yaml, env string
	logOpts   []logger.Option
}

func (s *sources) initFlags(fs *flag.FlagSet) {
	fs.StringVar(&s.yaml, "config", "config.yaml", "path to the YAML config file; empty skips it")
	fs.StringVar(&s.env, "env-file", ".env", "path to the .env file that overrides the YAML config; skipped if empty or missing")
}

type processConfig struct {
	AppEnv string `env:"APP_ENV" envDefault:"dev"`
}

// runtime reads the process-wide settings and builds the logger.
func (s *sources) runtime() (Runtime, error) {
	pc, err := config.Load[processConfig](s.yaml, s.env)
	if err != nil {
		return Runtime{}, err
	}
	return Runtime{AppEnv: pc.AppEnv, Log: newLogger(pc.AppEnv, s.logOpts)}, nil
}

func flagName(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return fmt.Sprintf("%s-%s", prefix, name)
}
