package jsrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/entity"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/settings"
)

const (
	resultPrefix   = "ANALYZER_RESULT_JSON:"
	progressPrefix = "ANALYZER_PROGRESS_JSON:"
)

// Runner executes analysis scripts with a real mongosh process, resolving
// them through catalog.
type Runner struct {
	catalog *Catalog
}

func NewRunner(catalog *Catalog) *Runner { return &Runner{catalog: catalog} }

// Run runs one script in mongosh, connected to cfg's database. ctx's
// deadline bounds the whole script: the mongosh process is killed when it
// passes.
func (r *Runner) Run(ctx context.Context, name string, cfg settings.MongoConfig) (entity.ScriptResult, error) {
	result := entity.ScriptResult{Script: name}
	if !r.catalog.Exists(name) {
		return result, fmt.Errorf("unknown script %q", name)
	}
	if err := r.catalog.ValidateTarget(name, cfg.Collection); err != nil {
		return result, err
	}
	source, _, err := r.catalog.ReadSource(name)
	if err != nil {
		return result, err
	}
	mongosh, err := exec.LookPath("mongosh")
	if err != nil {
		return result, errors.New("mongosh not found in PATH; install mongosh to run analysis scripts")
	}

	// The connection string carries credentials, so it goes into a private
	// temp file rather than onto the mongosh command line (visible via ps).
	dir, err := os.MkdirTemp("", "mongoanalyzer-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "script.js")
	program, err := buildProgram(source, cfg)
	if err != nil {
		return result, err
	}
	if err := os.WriteFile(file, program, 0o600); err != nil {
		return result, err
	}

	clean := func(s string) string { return strings.ReplaceAll(s, cfg.URI, "<MONGO_URI redacted>") }
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, mongosh, "--nodb", "--norc", "--quiet", file)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = 5 * time.Second
	started := time.Now()
	runErr := cmd.Run()
	result.DurationMs = time.Since(started).Milliseconds()

	var other []string
	for _, line := range strings.Split(clean(stdout.String()), "\n") {
		if data, ok := strings.CutPrefix(line, resultPrefix); ok && json.Valid([]byte(data)) {
			result.Output = json.RawMessage(data)
			continue
		}
		if data, ok := strings.CutPrefix(line, progressPrefix); ok && json.Valid([]byte(data)) {
			result.PartialResults = append(result.PartialResults, json.RawMessage(data))
			continue
		}
		other = append(other, line)
	}
	result.Stdout = strings.TrimSpace(strings.Join(other, "\n"))
	if result.Output == nil && result.Stdout != "" && json.Valid([]byte(result.Stdout)) {
		result.Output = json.RawMessage(result.Stdout)
		result.Stdout = ""
	}

	if runErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, fmt.Errorf("JS %s: %w", name, ctxErr)
		}
		detail := strings.TrimSpace(clean(stderr.String()))
		if detail == "" {
			detail = runErr.Error()
		}
		return result, fmt.Errorf("JS %s: %s", name, detail)
	}
	result.PartialResults = nil
	return result, nil
}

// buildProgram prepends the options and the database connection to the
// script, so the script runs as if typed into a connected mongosh session.
func buildProgram(source string, cfg settings.MongoConfig) ([]byte, error) {
	options, err := json.Marshal(map[string]any{
		"collection":     cfg.Collection,
		"queryTimeoutMS": cfg.QueryTimeout.Milliseconds(),
		"sampleSize":     cfg.SampleSize,
		"explainLimit":   cfg.ExplainLimit,
	})
	if err != nil {
		return nil, err
	}
	uri, err := json.Marshal(cfg.URI)
	if err != nil {
		return nil, err
	}
	database, err := json.Marshal(cfg.Database)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "globalThis.MONGO_ANALYSIS_OPTIONS = %s;\n", options)
	fmt.Fprintf(&b, "db = connect(%s).getSiblingDB(%s);\n", uri, database)
	b.WriteString(source)
	b.WriteString("\n")
	return []byte(b.String()), nil
}
