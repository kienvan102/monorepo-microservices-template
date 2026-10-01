package logger

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestOnlyErrorHasStack(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(WithOutput(&buf))

	log.Warn("slow")
	if _, ok := decode(t, &buf)["stack"]; ok {
		t.Error("warn entries must not carry a stack")
	}

	buf.Reset()
	line := nextLine()
	log.Error("failed", "error", errors.New("boom"))
	entry := decode(t, &buf)
	if entry["error"] != "boom" {
		t.Errorf("error = %v, want boom", entry["error"])
	}
	stack, _ := entry["stack"].([]any)
	if len(stack) == 0 {
		t.Fatalf("stack = %v, want frames", entry["stack"])
	}
	if want := "logger/caller_test.go:" + line + " logger.TestOnlyErrorHasStack"; stack[0] != want {
		t.Errorf("first frame = %v, want %s", stack[0], want)
	}
	for _, f := range stack {
		if strings.Contains(f.(string), " runtime.") {
			t.Errorf("runtime frame %v must be dropped", f)
		}
	}
}

func TestConsoleLayout(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(WithFormat(FormatConsole), WithOutput(&buf))

	line := nextLine()
	log.Error("script failed", "script", "stats")

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("want the entry plus stack lines, got %q", buf.String())
	}
	for _, want := range []string{"logger/caller_test.go:" + line, "script failed", "func=", "logger.TestConsoleLayout", "script=", "stats"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("first line %q is missing %q", lines[0], want)
		}
	}
	if strings.Contains(lines[0], "stack") {
		t.Errorf("stack must be printed below the entry, not inline: %q", lines[0])
	}
	if want := "    logger/caller_test.go:" + line + " logger.TestConsoleLayout"; lines[1] != want {
		t.Errorf("second line = %q, want %q", lines[1], want)
	}
}
