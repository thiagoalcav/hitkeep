package hklog

import (
	"context"
	"log/slog"

	"github.com/nsqio/nsq/nsqd"
)

// NSQDLogger satisfies the nsqd.Logger interface.
type NSQDLogger struct {
	Logger *slog.Logger
}

func (l NSQDLogger) Output(calldepth int, s string) error {
	lvl := parseNSQLevel(s)
	l.Logger.Log(context.Background(), lvl, s, "calldepth", calldepth)
	return nil
}

// ApplyNSQDLogger configures the nsqd.Options with the provided slog.Logger and Level.
// We use this helper because nsqd.Options.LogLevel uses an internal type that we cannot
// reference directly in a function signature.
func ApplyNSQDLogger(opts *nsqd.Options, logger *slog.Logger, lvl slog.Level) {
	opts.Logger = NSQDLogger{Logger: logger}

	switch {
	case lvl <= slog.LevelDebug:
		opts.LogLevel = nsqd.LOG_DEBUG
	case lvl <= slog.LevelInfo:
		opts.LogLevel = nsqd.LOG_INFO
	case lvl <= slog.LevelWarn:
		opts.LogLevel = nsqd.LOG_WARN
	default:
		opts.LogLevel = nsqd.LOG_ERROR
	}
}
