// Package common 提供共享日志工具函数。
//
// 本文件定义了 Go slog 包的便捷包装器：LogDebug、
// LogInfo、LogWarn 和 LogError。每个函数都会捕获调用者的源码
// 位置并将其传递给默认的 slog 处理器。
package common

import (
	"log/slog"
	"runtime"
	"time"
)

// logWithSource 创建一条包含调用者源码位置的 slog 记录。
// skip 参数 (2) 跳过了 logWithSource 帧和公共包装器帧。
func logWithSource(level slog.Level, msg string, args ...any) {
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	_ = slog.Default().Handler().Handle(nil, r)
}

// LogDebug 以 debug 级别记录日志消息。如果 debug 未启用则为空操作。
func LogDebug(msg string, args ...any) {
	if !slog.Default().Enabled(nil, slog.LevelDebug) {
		return
	}
	logWithSource(slog.LevelDebug, msg, args...)
}

// LogInfo 以 info 级别记录日志消息。如果 info 未启用则为空操作。
func LogInfo(msg string, args ...any) {
	if !slog.Default().Enabled(nil, slog.LevelInfo) {
		return
	}
	logWithSource(slog.LevelInfo, msg, args...)
}

// LogWarn 以 warn 级别记录日志消息。如果 warn 未启用则为空操作。
func LogWarn(msg string, args ...any) {
	if !slog.Default().Enabled(nil, slog.LevelWarn) {
		return
	}
	logWithSource(slog.LevelWarn, msg, args...)
}

// LogError 以 error 级别记录日志消息。无论级别设置如何始终会记录。
func LogError(msg string, args ...any) {
	logWithSource(slog.LevelError, msg, args...)
}
