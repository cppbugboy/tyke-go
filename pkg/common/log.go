// Copyright 2026 Tyke Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

// TykeLog Tyke日志管理器。
//
// TykeLog提供统一的日志管理功能，支持：
//   - 多日志级别：debug/info/warn/error
//   - 文件输出：支持日志文件轮转
//   - 格式化输出：支持fmt风格的格式化
type TykeLog struct {
	// logger slog日志器实例
	logger *slog.Logger
	// level 当前日志级别
	level slog.Level
	// logPath 日志文件路径
	logPath string
	// handler slog文本处理器
	handler *slog.TextHandler
	// writer 日志输出目标
	writer io.Writer
}

var (
	// globalLog 全局日志实例
	globalLog *TykeLog
	// globalLogOnce 确保日志只初始化一次
	globalLogOnce sync.Once
)

// InitLog 初始化日志系统。
//
// 配置日志输出路径、级别和文件轮转参数。
// 如果logPath为空，日志将输出到标准输出。
//
// 参数:
//   - logPath: 日志文件路径，为空则输出到stdout
//   - logLevel: 日志级别（debug/info/warn/error）
//   - fileSizeMB: 单个日志文件最大大小（MB）
//   - fileCount: 保留的日志文件数量
//
// 返回值:
//   - error: 初始化失败时返回错误
//
// 示例:
//
//	err := InitLog("/var/log/tyke.log", "info", 10, 5)
//	if err != nil {
//	    panic(err)
//	}
func InitLog(logPath, logLevel string, fileSizeMB, fileCount uint32) error {
	var initErr error
	globalLogOnce.Do(func() {
		level := parseLogLevel(logLevel)
		var writer io.Writer

		if logPath != "" {
			writer = &lumberjack.Logger{
				Filename:   logPath,
				MaxSize:    int(fileSizeMB),
				MaxBackups: int(fileCount),
			}
		} else {
			writer = os.Stdout
		}

		handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
			Level: level,
		})

		globalLog = &TykeLog{
			logger:  slog.New(handler),
			level:   level,
			logPath: logPath,
			handler: handler,
			writer:  writer,
		}
	})
	return initErr
}

// getLog 获取全局日志实例。
//
// 如果日志未初始化，自动初始化为默认配置（输出到stdout，info级别）。
func getLog() *TykeLog {
	globalLogOnce.Do(func() {
		handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
		globalLog = &TykeLog{
			logger:  slog.New(handler),
			level:   slog.LevelInfo,
			handler: handler,
			writer:  os.Stdout,
		}
	})
	return globalLog
}

// SetLogLevel 设置日志级别。
//
// 动态修改日志级别，影响后续所有日志输出。
//
// 参数:
//   - level: 日志级别字符串（debug/info/warn/error）
//
// 示例:
//
//	SetLogLevel("debug") // 开启调试日志
func SetLogLevel(level string) {
	l := getLog()
	l.level = parseLogLevel(level)
	handler := slog.NewTextHandler(l.writer, &slog.HandlerOptions{
		Level: l.level,
	})
	l.handler = handler
	l.logger = slog.New(handler)
}

// LogDebug 输出调试级别日志。
//
// 仅当日志级别为debug时输出。用于详细的调试信息。
//
// 参数:
//   - format: 格式化字符串
//   - args: 格式化参数
func LogDebug(format string, args ...any) {
	l := getLog()
	if l.level <= slog.LevelDebug {
		l.logger.Debug(fmt.Sprintf(format, args...))
	}
}

// LogInfo 输出信息级别日志。
//
// 用于记录关键业务信息，如请求处理、连接状态等。
//
// 参数:
//   - format: 格式化字符串
//   - args: 格式化参数
func LogInfo(format string, args ...any) {
	l := getLog()
	if l.level <= slog.LevelInfo {
		l.logger.Info(fmt.Sprintf(format, args...))
	}
}

// LogWarn 输出警告级别日志。
//
// 用于记录可恢复的异常情况，需要关注但不影响系统运行。
//
// 参数:
//   - format: 格式化字符串
//   - args: 格式化参数
func LogWarn(format string, args ...any) {
	l := getLog()
	if l.level <= slog.LevelWarn {
		l.logger.Warn(fmt.Sprintf(format, args...))
	}
}

// LogError 输出错误级别日志。
//
// 用于记录错误情况，可能影响系统功能。
//
// 参数:
//   - format: 格式化字符串
//   - args: 格式化参数
func LogError(format string, args ...any) {
	l := getLog()
	l.logger.Error(fmt.Sprintf(format, args...))
}

// parseLogLevel 解析日志级别字符串。
//
// 将字符串转换为slog.Level枚举值。
func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug", "DEBUG":
		return slog.LevelDebug
	case "info", "INFO":
		return slog.LevelInfo
	case "warn", "WARN", "warning", "WARNING":
		return slog.LevelWarn
	case "error", "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
