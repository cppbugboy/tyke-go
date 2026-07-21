// Package core implements the Tyke framework kernel.
//
// This file provides the Tyke logging system: a singleton LogConfig that wraps
// slog with file rotation via lumberjack. It supports console + file output,
// configurable log levels, and log file size/count management.
package core

import (
	"io"
	"log/slog"
	"os"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
	"tyke-go/common"
)

// LogConfig manages the framework's logging configuration and lifecycle.
type LogConfig struct {
	logger           *slog.Logger
	file             *os.File
	multiWriter      io.Writer
	lumberjackWriter *lumberjack.Logger
}

var (
	tykeLogInstance *LogConfig
	tykeLogOnce     sync.Once
)

// GetTykeLogInstance returns the singleton LogConfig instance.
func GetTykeLogInstance() *LogConfig {
	tykeLogOnce.Do(func() {
		tykeLogInstance = &LogConfig{}
	})
	return tykeLogInstance
}

// Init initializes the logging system with file rotation support. If the logger is
// already initialized, it only updates the log level. Returns an error on failure.
func (t *LogConfig) Init(logPath string, logLevel string, fileSizeMb uint32, fileCount uint32) common.BoolResult {
	if t.logger != nil {
		t.SetLogLevel(logLevel)
		return common.OkBool(true)
	}

	var writers []io.Writer
	writers = append(writers, os.Stdout)

	if logPath != "" {
		if fileSizeMb > 0 {
			t.lumberjackWriter = &lumberjack.Logger{
				Filename:   logPath,
				MaxSize:    int(fileSizeMb),
				MaxBackups: int(fileCount),
				Compress:   true,
			}
			writers = append(writers, t.lumberjackWriter)
		} else {
			f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return common.ErrBool("Failed to initialize log system: " + err.Error())
			}
			t.file = f
			writers = append(writers, f)
		}
	}

	multiWriter := io.MultiWriter(writers...)
	t.multiWriter = multiWriter
	handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{Level: slog.LevelDebug})
	t.logger = slog.New(handler)
	slog.SetDefault(t.logger)

	t.SetLogLevel(logLevel)

	common.LogInfo("Tyke log system initialized", "path", logPath, "level", logLevel)
	return common.OkBool(true)
}

// IsInitialized returns true if the logger has been successfully initialized.
func (t *LogConfig) IsInitialized() bool {
	return t.logger != nil
}

// SetLogLevel changes the log level of the running logger. Accepted values are
// "debug", "info", "warn", and "error". Unknown values default to "info".
func (t *LogConfig) SetLogLevel(logLevel string) {
	if t.logger == nil {
		return
	}
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	w := t.multiWriter
	if w == nil {
		w = os.Stdout
	}
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	t.logger = slog.New(handler)
	slog.SetDefault(t.logger)
}

// Stop gracefully shuts down the logging system, closing file handles and the
// lumberjack rotator if active.
func (t *LogConfig) Stop() {
	if t.logger != nil {
		common.LogInfo("Tyke log system shutting down")
		if t.lumberjackWriter != nil {
			t.lumberjackWriter.Close()
			t.lumberjackWriter = nil
		}
		if t.file != nil {
			err := t.file.Close()
			if err != nil {
				common.LogError("Failed to close log file", "error", err)
			}
			t.file = nil
		}
		t.logger = nil
	}
}
