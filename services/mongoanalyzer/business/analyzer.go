// Package business holds mongoanalyzer's use cases: connect, run JS analysis
// scripts, write reports. It depends only on the ports in ports.go, never on
// an adapter, and doesn't know how it is run (CLI, scheduler, server...).
package business

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kienvan102/monorepo-microservices-template/core/jsonfile"
	"github.com/kienvan102/monorepo-microservices-template/core/logger"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/entity"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/settings"
)

type Analyzer struct {
	cfg       settings.Config
	logger    logger.Logger
	catalog   ScriptCatalog
	connector Connector
	runner    ScriptRunner
	writer    jsonfile.Writer
}

func NewAnalyzer(cfg settings.Config, log logger.Logger, catalog ScriptCatalog, connector Connector, runner ScriptRunner, writer jsonfile.Writer) *Analyzer {
	return &Analyzer{cfg: cfg, logger: log, catalog: catalog, connector: connector, runner: runner, writer: writer}
}

// ListScripts returns every available script name; it doesn't connect.
func (a *Analyzer) ListScripts() ([]string, error) { return a.catalog.Discover() }

// HasScript reports whether name is an available script.
func (a *Analyzer) HasScript(name string) bool { return a.catalog.Exists(name) }

// Check connects and confirms the configured collection exists.
func (a *Analyzer) Check(ctx context.Context) error {
	client, err := a.connector.Connect(ctx, a.cfg.Mongo)
	if err != nil {
		return err
	}
	client.Disconnect()
	a.logger.Info("connected to database", "database", a.cfg.Mongo.Database, "collection", a.cfg.Mongo.Collection)
	return nil
}

// Inspect runs the common scripts listed in scripts/plan.json.
func (a *Analyzer) Inspect(ctx context.Context) error { return a.runPlan(ctx, entity.PlanGroupInspect) }

// Collect runs the common scripts plus the configured collection's own plan.
func (a *Analyzer) Collect(ctx context.Context) error { return a.runPlan(ctx, entity.PlanGroupDefault) }

// RunScripts runs the named scripts and writes one report for them.
func (a *Analyzer) RunScripts(ctx context.Context, names ...string) error {
	if len(names) == 0 {
		return fmt.Errorf("no scripts selected")
	}
	for _, name := range names {
		if !a.catalog.Exists(name) {
			return fmt.Errorf("script %q not found", name)
		}
		if err := a.catalog.ValidateTarget(name, a.cfg.Mongo.Collection); err != nil {
			return err
		}
	}
	// Fail fast on a bad connection or missing collection before starting
	// any script; the scripts open their own connection through mongosh.
	if err := a.Check(ctx); err != nil {
		return err
	}
	return a.executeAndReport(ctx, names)
}

func (a *Analyzer) runPlan(ctx context.Context, group string) error {
	names, err := a.catalog.Plan(group, a.cfg.Mongo.Collection)
	if err != nil {
		return err
	}
	return a.RunScripts(ctx, names...)
}

// executeAndReport runs each script, writes its result and the run
// manifest, and returns every script's error joined together.
func (a *Analyzer) executeAndReport(ctx context.Context, selected []string) error {
	runDir := filepath.Join(string(a.cfg.Mongo.OutputDir), a.cfg.Mongo.Collection, time.Now().UTC().Format("20060102T150405.000000000Z"))
	manifest := entity.Manifest{CapturedAt: time.Now().UTC(), Database: a.cfg.Mongo.Database, Collection: a.cfg.Mongo.Collection, Entries: []entity.Entry{}}
	var failures []error
	for _, name := range selected {
		started := time.Now()
		scriptCtx, cancel := context.WithTimeout(ctx, a.cfg.Mongo.ScriptTimeout)
		data, runErr := a.runner.Run(scriptCtx, name, a.cfg.Mongo)
		cancel()
		durationMs := time.Since(started).Milliseconds()
		entry := entity.Entry{Script: name, Kind: entity.KindJS, Status: entity.StatusOK, File: strings.ReplaceAll(name, "/", "__") + ".json"}
		if err := a.writer.WriteJSON(runDir, entry.File, data); err != nil {
			runErr = errors.Join(runErr, err)
		}
		if runErr != nil {
			entry.Status, entry.Error = entity.StatusError, runErr.Error()
			failures = append(failures, fmt.Errorf("%s: %w", name, runErr))
			a.logger.Error("script failed", "script", name, "kind", entry.Kind, "duration_ms", durationMs, "error", runErr)
		} else {
			a.logger.Info("script completed", "script", name, "kind", entry.Kind, "status", entry.Status, "duration_ms", durationMs)
		}
		manifest.Entries = append(manifest.Entries, entry)
	}
	if err := a.writer.WriteJSON(runDir, "manifest.json", manifest); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	a.logger.Info("run complete", "output_dir", runDir, "scripts", len(selected), "failures", len(failures))
	return errors.Join(failures...)
}
