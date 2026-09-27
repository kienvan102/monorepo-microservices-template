package logger

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// Logger is the logging surface the rest of the app depends on, so callers
// can be unit tested against a fake without pulling in zerolog itself.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

func normalizeEnvType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "test", "testing":
		return "testing"
	case "stage", "staging":
		return "staging"
	case "prod", "production":
		return "production"
	default:
		return "development"
	}
}

func NewLogger(envType string) Logger {
	var zl zerolog.Logger
	switch normalizeEnvType(envType) {
	case "staging", "production":
		zl = zerolog.New(os.Stderr).Level(zerolog.InfoLevel).With().Timestamp().Logger()
	default:
		console := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02T15:04:05.000Z07:00"}
		zl = zerolog.New(console).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	}
	return zerologLogger{logger: zl}
}

type zerologLogger struct {
	logger zerolog.Logger
}

func (z zerologLogger) Debug(msg string, args ...any) { z.emit(z.logger.Debug(), msg, args) }
func (z zerologLogger) Info(msg string, args ...any)  { z.emit(z.logger.Info(), msg, args) }
func (z zerologLogger) Warn(msg string, args ...any)  { z.emit(z.logger.Warn(), msg, args) }
func (z zerologLogger) Error(msg string, args ...any) { z.emit(z.logger.Error(), msg, args) }

// emit maps the "key, value, key, value..." args every call site already
// passes into zerolog fields, so nothing outside this file had to change.
func (z zerologLogger) emit(event *zerolog.Event, msg string, args []any) {
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		switch v := args[i+1].(type) {
		case error:
			event = event.Str(key, v.Error())
		case string:
			event = event.Str(key, v)
		case int:
			event = event.Int(key, v)
		case int64:
			event = event.Int64(key, v)
		default:
			event = event.Interface(key, v)
		}
	}
	event.Msg(msg)
}
