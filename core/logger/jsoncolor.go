package logger

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strconv"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
)

// ANSI colors, matching what zerolog's ConsoleWriter uses for the same parts.
const (
	ansiReset    = "\x1b[0m"
	ansiBold     = "\x1b[1m"
	ansiRed      = "\x1b[31m"
	ansiGreen    = "\x1b[32m"
	ansiYellow   = "\x1b[33m"
	ansiCyan     = "\x1b[36m"
	ansiDarkGray = "\x1b[90m"
)

// isColorTerminal reports whether w is a terminal that should get colors:
// a terminal file, unless NO_COLOR (https://no-color.org) is set.
func isColorTerminal(w io.Writer) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	f, ok := w.(*os.File)
	return ok && (isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd()))
}

// colorJSONWriter colors JSON entries for reading in a terminal. Every
// entry stays one line with its fields in the same order; only ANSI codes
// are added. It is used only when the output is a terminal, so files and
// pipes always get plain JSON.
type colorJSONWriter struct {
	out io.Writer
}

func (w colorJSONWriter) Write(p []byte) (int, error) {
	colored, err := colorizeJSON(p)
	if err != nil {
		// Not an entry we can parse: write it as it is rather than lose it.
		return w.out.Write(p)
	}
	colored = append(colored, '\n')
	if _, err := w.out.Write(colored); err != nil {
		return 0, err
	}
	return len(p), nil
}

// jsonLevel is one open object or array while re-encoding.
type jsonLevel struct {
	object  bool
	wantKey bool // object only: the next token is a key
	n       int  // values (arrays) or key/value pairs (objects) written
}

// colorizeJSON re-encodes one JSON entry token by token, so field order is
// kept, coloring keys and the top-level fields people scan for.
func colorizeJSON(p []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(p))
	dec.UseNumber()
	var buf bytes.Buffer
	var stack []jsonLevel
	var key string // the top-level key whose value comes next

	beforeValue := func() {
		if len(stack) > 0 && !stack[len(stack)-1].object && stack[len(stack)-1].n > 0 {
			buf.WriteByte(',')
		}
	}
	afterValue := func() {
		if len(stack) > 0 {
			top := &stack[len(stack)-1]
			top.n++
			top.wantKey = top.object
		}
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if d, ok := tok.(json.Delim); ok {
			switch d {
			case '{', '[':
				beforeValue()
				buf.WriteByte(byte(d))
				stack = append(stack, jsonLevel{object: d == '{', wantKey: d == '{'})
			default:
				stack = stack[:len(stack)-1]
				buf.WriteByte(byte(d))
				afterValue()
			}
			continue
		}

		if top := len(stack) - 1; top >= 0 && stack[top].wantKey {
			if stack[top].n > 0 {
				buf.WriteByte(',')
			}
			name, _ := tok.(string)
			buf.WriteString(ansiCyan + quoteJSON(name) + ansiReset + ":")
			stack[top].wantKey = false
			if top == 0 {
				key = name
			}
			continue
		}

		beforeValue()
		value := encodeToken(tok)
		if color := valueColor(key, tok, len(stack)); color != "" {
			value = color + value + ansiReset
		}
		buf.WriteString(value)
		afterValue()
	}
	return buf.Bytes(), nil
}

// valueColor picks the color of a value of the entry's top-level key.
func valueColor(key string, tok json.Token, depth int) string {
	if depth != 1 {
		return ""
	}
	switch key {
	case zerolog.TimestampFieldName:
		return ansiDarkGray
	case zerolog.LevelFieldName:
		switch tok {
		case zerolog.LevelDebugValue:
			return ansiYellow
		case zerolog.LevelInfoValue:
			return ansiGreen
		case zerolog.LevelWarnValue:
			return ansiRed
		case zerolog.LevelErrorValue, zerolog.LevelFatalValue, zerolog.LevelPanicValue:
			return ansiBold + ansiRed
		}
	case zerolog.MessageFieldName, zerolog.CallerFieldName:
		return ansiBold
	case zerolog.ErrorFieldName:
		return ansiRed
	}
	return ""
}

func encodeToken(tok json.Token) string {
	switch v := tok.(type) {
	case string:
		return quoteJSON(v)
	case json.Number:
		return v.String()
	case bool:
		return strconv.FormatBool(v)
	default: // nil
		return "null"
	}
}

// quoteJSON quotes s as a JSON string without escaping <, > and &.
func quoteJSON(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return string(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
}
