package hklog

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"log/slog"
	"strings"
)

// Writer is an io.Writer that forwards lines to slog at a fixed level.
type Writer struct {
	Logger *slog.Logger
	Level  slog.Level
}

func (w Writer) Write(p []byte) (int, error) {
	msg := string(bytes.TrimRight(p, "\n"))
	w.Logger.Log(context.Background(), w.Level, msg)
	return len(p), nil
}

// StdLogger returns a *log.Logger that writes to the provided slog.Logger at the given level.
func StdLogger(l *slog.Logger, level slog.Level) *log.Logger {
	return log.New(Writer{Logger: l, Level: level}, "", 0)
}

// ParseLevel parses a user-provided string into a slog.Level.
// Accepts: debug, info, warn, warning, error. Case-insensitive.
func ParseLevel(s string) (slog.Level, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level: %q", s)
	}
}
