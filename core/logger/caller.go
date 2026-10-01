package logger

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

// funcFieldName holds the function that logged the entry, next to
// zerolog.CallerFieldName (file:line).
const funcFieldName = "func"

// maxStackFrames caps the stack attached to Error entries.
const maxStackFrames = 32

// callerStack returns "file:line func" frames, starting skip frames above
// runtime.Callers, without the Go runtime's own frames.
func callerStack(skip int) []string {
	pcs := make([]uintptr, maxStackFrames)
	n := runtime.Callers(skip, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	stack := make([]string, 0, n)
	for {
		f, more := frames.Next()
		if !strings.HasPrefix(f.Function, "runtime.") {
			stack = append(stack, shortFile(f.File)+":"+strconv.Itoa(f.Line)+" "+shortFunc(f.Function))
		}
		if !more {
			return stack
		}
	}
}

// writeConsoleStack prints an Error entry's stack under it, one frame per
// line, instead of as a JSON array in the middle of the line.
func writeConsoleStack(evt map[string]any, buf *bytes.Buffer) error {
	frames, _ := evt[zerolog.ErrorStackFieldName].([]any)
	for _, f := range frames {
		if s, ok := f.(string); ok {
			buf.WriteString("\n    ")
			buf.WriteString(s)
		}
	}
	return nil
}

// shortFile keeps the file's directory and name:
// ".../services/mongoanalyzer/business/analyzer.go" -> "business/analyzer.go".
func shortFile(path string) string {
	return filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path)
}

// shortFunc drops the import path from a function name:
// ".../mongoanalyzer/business.(*Analyzer).Run" -> "business.(*Analyzer).Run".
func shortFunc(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}
