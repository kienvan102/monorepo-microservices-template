package logger

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestOptionsOverrideDefaults(t *testing.T) {
	cases := []struct {
		name      string
		opts      []Option
		wantJSON  bool
		wantDebug bool
	}{
		{name: "defaults", wantJSON: true},
		{name: "empty values", opts: []Option{WithFormat(""), WithLevel("")}, wantJSON: true},
		{name: "unknown values", opts: []Option{WithFormat("jsn"), WithLevel("loud")}, wantJSON: true},
		{name: "console debug", opts: []Option{WithFormat(FormatConsole), WithLevel(LevelDebug)}, wantDebug: true},
		{name: "later wins", opts: []Option{WithFormat(FormatConsole), WithLevel(LevelDebug), WithFormat(FormatJSON), WithLevel(LevelWarn)}, wantJSON: true},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		log := NewLogger(append(c.opts, WithOutput(&buf))...)
		log.Debug("d")
		if gotDebug := buf.Len() > 0; gotDebug != c.wantDebug {
			t.Errorf("%s: debug written = %v, want %v", c.name, gotDebug, c.wantDebug)
		}
		buf.Reset()
		log.Error("e")
		if gotJSON := json.Valid(buf.Bytes()); gotJSON != c.wantJSON {
			t.Errorf("%s: JSON = %v, want %v (%q)", c.name, gotJSON, c.wantJSON, buf.String())
		}
	}
}
