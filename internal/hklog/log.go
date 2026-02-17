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

// LevelParsingWriter forwards log lines to slog, inferring level from a prefix.
type LevelParsingWriter struct {
	Logger        *slog.Logger
	DefaultLevel  slog.Level
	ComponentName string
}

func (w LevelParsingWriter) Write(p []byte) (int, error) {
	msg := string(bytes.TrimRight(p, "\n"))
	level := w.DefaultLevel

	trimmed := strings.TrimSpace(msg)
	level, trimmed = parseLevelPrefix(level, trimmed)
	if after, ok := strings.CutPrefix(trimmed, "memberlist:"); ok {
		trimmed = strings.TrimSpace(after)
	}

	logger := w.Logger
	if w.ComponentName != "" {
		logger = logger.With("component", w.ComponentName)
	}
	logger.Log(context.Background(), level, trimmed)
	return len(p), nil
}

// StdLogger returns a *log.Logger that writes to the provided slog.Logger at the given level.
func StdLogger(l *slog.Logger, level slog.Level) *log.Logger {
	return log.New(Writer{Logger: l, Level: level}, "", 0)
}

// MemberlistLogger returns a *log.Logger that maps memberlist log levels to slog.
func MemberlistLogger(l *slog.Logger) *log.Logger {
	return log.New(LevelParsingWriter{
		Logger:        l,
		DefaultLevel:  slog.LevelInfo,
		ComponentName: "memberlist",
	}, "", 0)
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

func parseLevelPrefix(defaultLevel slog.Level, msg string) (slog.Level, string) {
	switch {
	case strings.HasPrefix(msg, "[DEBUG]"):
		return slog.LevelDebug, strings.TrimSpace(strings.TrimPrefix(msg, "[DEBUG]"))
	case strings.HasPrefix(msg, "[INFO]"):
		return slog.LevelInfo, strings.TrimSpace(strings.TrimPrefix(msg, "[INFO]"))
	case strings.HasPrefix(msg, "[WARN]"):
		return slog.LevelWarn, strings.TrimSpace(strings.TrimPrefix(msg, "[WARN]"))
	case strings.HasPrefix(msg, "[WARNING]"):
		return slog.LevelWarn, strings.TrimSpace(strings.TrimPrefix(msg, "[WARNING]"))
	case strings.HasPrefix(msg, "[ERR]"):
		return slog.LevelError, strings.TrimSpace(strings.TrimPrefix(msg, "[ERR]"))
	case strings.HasPrefix(msg, "[ERROR]"):
		return slog.LevelError, strings.TrimSpace(strings.TrimPrefix(msg, "[ERROR]"))
	default:
		return defaultLevel, msg
	}
}
