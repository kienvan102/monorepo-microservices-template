package logger

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestDefaultsFollowEnv(t *testing.T) {
	cases := []struct {
		env       string
		opts      []Option
		wantJSON  bool
		wantDebug bool
	}{
		{env: "production", wantJSON: true},
		{env: "staging", wantJSON: true},
		{env: "dev", wantDebug: true},
		{env: "testing", wantDebug: true},
		{env: "production", opts: []Option{WithFormat(""), WithLevel("")}, wantJSON: true},
		{env: "production", opts: []Option{WithFormat("jsn"), WithLevel("loud")}, wantJSON: true},
		{env: "production", opts: []Option{WithFormat(FormatConsole), WithLevel(LevelDebug)}, wantDebug: true},
		{env: "dev", opts: []Option{WithFormat(FormatJSON), WithLevel(LevelWarn)}, wantJSON: true},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		log := NewLogger(c.env, append(c.opts, WithOutput(&buf))...)
		log.Debug("d")
		if gotDebug := buf.Len() > 0; gotDebug != c.wantDebug {
			t.Errorf("%s %d opts: debug written = %v, want %v", c.env, len(c.opts), gotDebug, c.wantDebug)
		}
		buf.Reset()
		log.Error("e")
		if gotJSON := json.Valid(buf.Bytes()); gotJSON != c.wantJSON {
			t.Errorf("%s %d opts: JSON = %v, want %v (%q)", c.env, len(c.opts), gotJSON, c.wantJSON, buf.String())
		}
	}
}
