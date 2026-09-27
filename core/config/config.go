package config

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Path is a config value holding a file system path. A relative Path is
// resolved against the directory of the YAML file given to Load, or of the
// .env file when no YAML file is used, so it doesn't depend on where the
// process was started from.
type Path string

// Option adjusts how Load reads T.
type Option func(*options)

type options struct {
	prefix string
}

// WithPrefix makes every variable T reads start with name, converted like a
// YAML key and followed by "_": WithPrefix("analyzer") reads
// ANALYZER_MONGO_URI instead of MONGO_URI, which in YAML is analyzer.mongo.uri.
func WithPrefix(name string) Option {
	return func(o *options) { o.prefix = envName(name) + "_" }
}

// Load builds T from up to 4 layers, highest priority first:
//
//  1. real environment variables
//  2. the .env file at envPath (skipped if empty or missing)
//  3. the YAML file at yamlPath (skipped if empty; an error if missing)
//  4. envDefault tags on T
//
// YAML keys are converted to variable names as described in flattenYAML,
// then everything is parsed through T's env tags.
func Load[T any](yamlPath, envPath string, opts ...Option) (T, error) {
	var cfg T
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	merged := map[string]string{}
	if yamlPath != "" {
		data, err := os.ReadFile(yamlPath)
		if err != nil {
			return cfg, fmt.Errorf("read %s: %w", yamlPath, err)
		}
		values, err := flattenYAML(data)
		if err != nil {
			return cfg, fmt.Errorf("parse %s: %w", yamlPath, err)
		}
		maps.Copy(merged, values)
	}
	if envPath != "" {
		values, err := godotenv.Read(envPath)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return cfg, fmt.Errorf("read %s: %w", envPath, err)
		default:
			maps.Copy(merged, values)
		}
	}
	for _, entry := range os.Environ() {
		if name, value, ok := strings.Cut(entry, "="); ok {
			merged[name] = value
		}
	}

	baseDir := ""
	switch {
	case yamlPath != "":
		baseDir = filepath.Dir(yamlPath)
	case envPath != "":
		baseDir = filepath.Dir(envPath)
	}
	parsePath := func(value string) (any, error) {
		if value == "" || baseDir == "" || filepath.IsAbs(value) {
			return Path(value), nil
		}
		return Path(filepath.Join(baseDir, value)), nil
	}

	err := env.ParseWithOptions(&cfg, env.Options{
		Environment: merged,
		Prefix:      o.prefix,
		FuncMap:     map[reflect.Type]env.ParserFunc{reflect.TypeOf(Path("")): parsePath},
	})
	if err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
