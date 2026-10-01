package app

import (
	"strings"

	"github.com/kienvan102/monorepo-microservices-template/commonlib/logger"
)

// newLogger builds Runtime.Log: how every process in this monorepo logs.
// APP_ENV picks the defaults (loggerDefaults); WithLogger options passed to
// New or Commands override them.
func newLogger(appEnv string, overrides []logger.Option) logger.Logger {
	return logger.NewLogger(append(loggerDefaults(appEnv), overrides...)...)
}

// loggerDefaults: readable console output with debug entries while
// developing and testing; JSON at info level, for log backends, in staging
// and production.
func loggerDefaults(appEnv string) []logger.Option {
	switch normalizeAppEnv(appEnv) {
	case "staging", "production":
		return []logger.Option{logger.WithFormat(logger.FormatJSON), logger.WithLevel(logger.LevelInfo)}
	default:
		return []logger.Option{logger.WithFormat(logger.FormatConsole), logger.WithLevel(logger.LevelDebug)}
	}
}

func normalizeAppEnv(raw string) string {
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
