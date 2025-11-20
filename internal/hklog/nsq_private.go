package hklog

import (
	"log/slog"
	"strings"
)

func parseNSQLevel(s string) slog.Level {
	switch {
	case strings.HasPrefix(s, "DBG"):
		return slog.LevelDebug
	case strings.HasPrefix(s, "INF"):
		return slog.LevelInfo
	case strings.HasPrefix(s, "WRN"):
		return slog.LevelWarn
	case strings.HasPrefix(s, "ERR"):
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
