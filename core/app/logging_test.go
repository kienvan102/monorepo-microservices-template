package app

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/kienvan102/monorepo-microservices-template/commonlib/logger"
)

func TestLoggerFollowsAppEnv(t *testing.T) {
	cases := []struct {
		appEnv    string
		overrides []logger.Option
		wantJSON  bool
		wantDebug bool
	}{
		{appEnv: "dev", wantDebug: true},
		{appEnv: "testing", wantDebug: true},
		{appEnv: "", wantDebug: true},
		{appEnv: "staging", wantJSON: true},
		{appEnv: "Prod", wantJSON: true},
		{appEnv: "production", overrides: []logger.Option{logger.WithFormat(logger.FormatConsole), logger.WithLevel(logger.LevelDebug)}, wantDebug: true},
		{appEnv: "dev", overrides: []logger.Option{logger.WithFormat(logger.FormatJSON)}, wantJSON: true, wantDebug: true},
		{appEnv: "dev", overrides: []logger.Option{logger.WithFormat("")}, wantDebug: true},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		log := newLogger(c.appEnv, append(c.overrides, logger.WithOutput(&buf)))
		log.Debug("d")
		if gotDebug := buf.Len() > 0; gotDebug != c.wantDebug {
			t.Errorf("APP_ENV=%q, %d overrides: debug written = %v, want %v", c.appEnv, len(c.overrides), gotDebug, c.wantDebug)
		}
		buf.Reset()
		log.Error("e")
		if gotJSON := json.Valid(buf.Bytes()); gotJSON != c.wantJSON {
			t.Errorf("APP_ENV=%q, %d overrides: JSON = %v, want %v (%q)", c.appEnv, len(c.overrides), gotJSON, c.wantJSON, buf.String())
		}
	}
}
