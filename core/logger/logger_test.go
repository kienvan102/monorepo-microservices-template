package logger

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strconv"
	"testing"
)

// nextLine returns the line after the one it is called from, where the
// tests put the log call whose caller they check.
func nextLine() string {
	_, _, line, _ := runtime.Caller(1)
	return strconv.Itoa(line + 1)
}

func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("decode %q: %v", buf.String(), err)
	}
	return entry
}

func TestInfoHasCallerAndMessage(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger("production", WithOutput(&buf))

	line := nextLine()
	log.Info("connected", "database", "shop")

	entry := decode(t, &buf)
	if entry["message"] != "connected" || entry["database"] != "shop" || entry["level"] != "info" {
		t.Errorf("entry = %v, want message, field and level", entry)
	}
	if want := "logger/logger_test.go:" + line; entry["caller"] != want {
		t.Errorf("caller = %v, want %s", entry["caller"], want)
	}
	if entry["func"] != "logger.TestInfoHasCallerAndMessage" {
		t.Errorf("func = %v, want logger.TestInfoHasCallerAndMessage", entry["func"])
	}
	if _, ok := entry["stack"]; ok {
		t.Error("info entries must not carry a stack")
	}
}
