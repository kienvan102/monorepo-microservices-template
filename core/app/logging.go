package app

import (
	"strings"

	"github.com/kienvan102/monorepo-microservices-template/commonlib/logger"
)

// newLogger builds Runtime.Log: how every process in this monorepo logs.
// APP_ENV picks the defaults (loggerDefaults); WithLogger options passed to
// Parallel or Commands override them.
func (s *sources) newLogger(appEnv string) logger.Logger {
	return logger.NewLogger(append(env(appEnv).loggerDefaults(), s.logOpts...)...)
}

// env is an APP_ENV value as written, before normalizing.
type env string

// loggerDefaults: readable console output with debug entries while
// developing and testing; JSON at info level, for log backends, in staging
// and production.
func (e env) loggerDefaults() []logger.Option {
	switch e.normalized() {
	case "staging", "production":
		return []logger.Option{logger.WithFormat(logger.FormatJSON), logger.WithLevel(logger.LevelInfo)}
	default:
		return []logger.Option{logger.WithFormat(logger.FormatConsole), logger.WithLevel(logger.LevelDebug)}
	}
}

func (e env) normalized() string {
	switch strings.ToLower(strings.TrimSpace(string(e))) {
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
