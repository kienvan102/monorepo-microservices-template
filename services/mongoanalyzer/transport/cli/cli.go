// Package cli is mongoanalyzer's command-line transport: the flags and
// actions through which a CLI deployment drives the Analyzer.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"

	"github.com/kienvan102/monorepo-microservices-template/commonlib/logger"
	"github.com/kienvan102/monorepo-microservices-template/core/app"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/business"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/jsrunner"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/scripts"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/settings"
)

// Action is one of the values -action accepts.
type Action string

const (
	ActionCheck   Action = "check"
	ActionList    Action = "list"
	ActionInspect Action = "inspect"
	ActionCollect Action = "collect"
	ActionRun     Action = "run"
)

// Builder builds the Analyzer; the deployment decides how (e.g. with wire).
type Builder func(cfg settings.Config, log logger.Logger, catalog *jsrunner.Catalog) *business.Analyzer

var _ app.Component[settings.Config] = (*Handler)(nil)

// Handler runs one -action per process: a run-once component.
type Handler struct {
	build      Builder
	action     Action
	scriptName string
	scriptsDir string
}

func NewHandler(build Builder) *Handler { return &Handler{build: build, action: ActionInspect} }

func (h *Handler) InitFlags(fs *flag.FlagSet) {
	fs.Func("action", "check, list, inspect, collect, or run", func(raw string) error {
		h.action = Action(raw)
		return nil
	})
	fs.StringVar(&h.scriptName, "script", "", "name of the script to run with -action run")
	fs.StringVar(&h.scriptsDir, "scripts-dir", "", "read scripts from this directory instead of the ones built into the binary")
}

// Run performs the parsed -action.
func (h *Handler) Run(ctx context.Context, rt app.Runtime, cfg settings.Config) error {
	scriptFS, err := h.scripts()
	if err != nil {
		return err
	}
	a := h.build(cfg, rt.Log, jsrunner.NewCatalog(scriptFS))

	var execute func(context.Context) error
	switch h.action {
	case ActionList:
		names, err := a.ListScripts()
		if err != nil {
			return err
		}
		for _, name := range names {
			fmt.Println(name)
		}
		return nil
	case ActionCheck:
		execute = a.Check
	case ActionInspect:
		execute = a.Inspect
	case ActionCollect:
		execute = a.Collect
	case ActionRun:
		if !a.HasScript(h.scriptName) {
			return fmt.Errorf("unknown script %q; use make list to see available scripts", h.scriptName)
		}
		execute = func(ctx context.Context) error { return a.RunScripts(ctx, h.scriptName) }
	default:
		return fmt.Errorf("invalid action %q: use check, list, inspect, collect, or run", h.action)
	}

	rt.Log.Debug("configuration loaded", "database", cfg.Mongo.Database, "collection", cfg.Mongo.Collection, "output_dir", string(cfg.Mongo.OutputDir), "app_env", rt.AppEnv)
	return execute(ctx)
}

// scripts returns the script tree for the Analyzer's catalog: the embedded
// scripts, or -scripts-dir when given.
func (h *Handler) scripts() (fs.FS, error) {
	if h.scriptsDir == "" {
		return scripts.FS, nil
	}
	if _, err := os.Stat(h.scriptsDir); err != nil {
		return nil, err
	}
	return os.DirFS(h.scriptsDir), nil
}
