package core

import (
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/tyke/tyke/tyke/common"
)

type TykeLog struct {
	logger      *slog.Logger
	file        *os.File
	multiWriter io.Writer
}

var (
	tykeLogInstance *TykeLog
	tykeLogOnce     sync.Once
)

func GetTykeLogInstance() *TykeLog {
	tykeLogOnce.Do(func() {
		tykeLogInstance = &TykeLog{}
	})
	return tykeLogInstance
}

func (t *TykeLog) Init(logPath string, logLevel string, fileSizeMb uint32, fileCount uint32) common.BoolResult {
	if t.logger != nil {
		t.SetLogLevel(logLevel)
		return common.OkBool(true)
	}

	var writers []io.Writer
	writers = append(writers, os.Stdout)

	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return common.ErrBool("Failed to initialize log system: " + err.Error())
		}
		t.file = f
		writers = append(writers, f)
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

func (t *TykeLog) IsInitialized() bool {
	return t.logger != nil
}

func (t *TykeLog) SetLogLevel(logLevel string) {
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

func (t *TykeLog) Stop() {
	if t.logger != nil {
		common.LogInfo("Tyke log system shutting down")
		if t.file != nil {
			t.file.Close()
			t.file = nil
		}
		t.logger = nil
	}
}
