package logger

import (
	"runtime"
	"strconv"

	"github.com/rs/zerolog"
)

// Logger is the logging surface the rest of the app depends on, so callers
// can be unit tested against a fake without pulling in zerolog itself.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// NewLogger builds the logger for envType: console at debug level for
// development and testing, JSON at info level for staging and production,
// written to stderr. opts override any of these. JSON written to a terminal
// is colored for reading; written anywhere else, or with NO_COLOR set, it
// stays plain so log backends can parse it.
func NewLogger(envType string, opts ...Option) Logger {
	o := defaultOptions(envType)
	for _, opt := range opts {
		opt(&o)
	}

	out := o.out
	switch {
	case o.format == FormatJSON && isColorTerminal(o.out):
		out = colorJSONWriter{out: o.out}
	case o.format == FormatConsole:
		out = zerolog.ConsoleWriter{
			Out:           o.out,
			TimeFormat:    "2006-01-02T15:04:05.000Z07:00",
			FieldsExclude: []string{zerolog.ErrorStackFieldName},
			FormatExtra:   writeConsoleStack,
		}
	}
	level, _ := zerologLevel(o.level)
	return zerologLogger{logger: zerolog.New(out).Level(level).With().Timestamp().Logger()}
}

type zerologLogger struct {
	logger zerolog.Logger
}

func (z zerologLogger) Debug(msg string, args ...any) { z.emit(z.logger.Debug(), msg, args, false) }
func (z zerologLogger) Info(msg string, args ...any)  { z.emit(z.logger.Info(), msg, args, false) }
func (z zerologLogger) Warn(msg string, args ...any)  { z.emit(z.logger.Warn(), msg, args, false) }
func (z zerologLogger) Error(msg string, args ...any) { z.emit(z.logger.Error(), msg, args, true) }

// emit maps the "key, value, key, value..." args every call site already
// passes into zerolog fields, so nothing outside this file had to change.
// It must be called directly from the level methods: the caller and stack
// are found by skipping exactly emit and that method.
func (z zerologLogger) emit(event *zerolog.Event, msg string, args []any, withStack bool) {
	if event == nil { // level disabled
		return
	}
	if pc, file, line, ok := runtime.Caller(2); ok {
		event = event.Str(zerolog.CallerFieldName, shortFile(file)+":"+strconv.Itoa(line))
		if fn := runtime.FuncForPC(pc); fn != nil {
			event = event.Str(funcFieldName, shortFunc(fn.Name()))
		}
	}
	if withStack {
		event = event.Strs(zerolog.ErrorStackFieldName, callerStack(4))
	}
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		switch v := args[i+1].(type) {
		case error:
			event = event.Str(key, v.Error())
		case string:
			event = event.Str(key, v)
		case int:
			event = event.Int(key, v)
		case int64:
			event = event.Int64(key, v)
		default:
			event = event.Interface(key, v)
		}
	}
	event.Msg(msg)
}
