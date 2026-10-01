package logger

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestColorizeJSONOnlyAddsColors(t *testing.T) {
	entry := `{"level":"error","caller":"business/analyzer.go:104","func":"business.(*Analyzer).run","n":3.5,"ok":true,"none":null,"nested":{"a":[1,"<b>",{"c":false}],"d":{}},"empty":[],"stack":["a.go:1 a.f","b.go:2 b.g"],"error":"boom","time":"2026-10-01T10:00:00+07:00","message":"script failed"}`

	colored, err := colorizeJSON([]byte(entry + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := ansi.ReplaceAllString(string(colored), ""); got != entry {
		t.Errorf("without colors:\n got %s\nwant %s", got, entry)
	}
	for _, want := range []string{
		ansiCyan + `"level"` + ansiReset,
		ansiBold + ansiRed + `"error"` + ansiReset,
		ansiBold + `"script failed"` + ansiReset,
		ansiRed + `"boom"` + ansiReset,
	} {
		if !strings.Contains(string(colored), want) {
			t.Errorf("colored entry is missing %q:\n%q", want, colored)
		}
	}
}

func TestColorJSONWriterKeepsUnparsableInput(t *testing.T) {
	var buf bytes.Buffer
	in := []byte("not json\n")
	if n, err := (colorJSONWriter{out: &buf}).Write(in); err != nil || n != len(in) || buf.String() != string(in) {
		t.Errorf("Write = %d, %v, %q; want the input written as is", n, err, buf.String())
	}
}

func TestJSONStaysPlainOffTerminal(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatal(err)
	}
	if isColorTerminal(f) {
		t.Error("a regular file must not be treated as a terminal")
	}
	t.Setenv("NO_COLOR", "1")
	if isColorTerminal(os.Stderr) {
		t.Error("NO_COLOR must turn colors off")
	}
}
