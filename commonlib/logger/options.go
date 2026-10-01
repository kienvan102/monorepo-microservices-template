package logger

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

// Format is how entries are written: human-readable console lines or one
// JSON object per line for log backends.
type Format string

const (
	FormatConsole Format = "console"
	FormatJSON    Format = "json"
)

// Level is the lowest level that is written.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Option overrides one of NewLogger's defaults. An empty or unknown value
// keeps the default.
type Option func(*options)

type options struct {
	format Format
	level  Level
	out    io.Writer
}

// WithFormat sets the output format (FormatConsole or FormatJSON).
func WithFormat(f Format) Option {
	return func(o *options) {
		if f == FormatConsole || f == FormatJSON {
			o.format = f
		}
	}
}

// WithLevel sets the lowest level that is written.
func WithLevel(l Level) Option {
	return func(o *options) {
		if _, ok := zerologLevel(l); ok {
			o.level = l
		}
	}
}

// WithOutput sets where entries are written (stderr by default).
func WithOutput(w io.Writer) Option {
	return func(o *options) {
		if w != nil {
			o.out = w
		}
	}
}

// defaultOptions are what NewLogger uses for anything opts leave unset.
func defaultOptions() options {
	return options{format: FormatJSON, level: LevelInfo, out: os.Stderr}
}

func zerologLevel(l Level) (zerolog.Level, bool) {
	switch l {
	case LevelDebug:
		return zerolog.DebugLevel, true
	case LevelInfo:
		return zerolog.InfoLevel, true
	case LevelWarn:
		return zerolog.WarnLevel, true
	case LevelError:
		return zerolog.ErrorLevel, true
	default:
		return zerolog.NoLevel, false
	}
}
