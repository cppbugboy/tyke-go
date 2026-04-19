package common

import (
	"log/slog"
	"runtime"
	"time"
)

func logWithSource(level slog.Level, msg string, args ...any) {
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	_ = slog.Default().Handler().Handle(nil, r)
}

func LogDebug(msg string, args ...any) {
	logWithSource(slog.LevelDebug, msg, args...)
}

func LogInfo(msg string, args ...any) {
	logWithSource(slog.LevelInfo, msg, args...)
}

func LogWarn(msg string, args ...any) {
	logWithSource(slog.LevelWarn, msg, args...)
}

func LogError(msg string, args ...any) {
	logWithSource(slog.LevelError, msg, args...)
}
