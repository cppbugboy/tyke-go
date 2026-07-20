// Package common 提供共享工具函数。
//
// 本文件定义了通用辅助函数：UUID 生成、
// 时间戳格式化、UUID 验证和临时目录获取。
package common

import (
	"crypto/rand"
	"fmt"
	"os"
	"regexp"
	"time"
)

// uuidRegex 匹配标准 UUID v4 格式（可带花括号）。
var uuidRegex = regexp.MustCompile(`^{?[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}}?$`)

// GenerateUUID 生成一个随机的 UUID v4 字符串。
// 如果 crypto/rand 失败，则回退到基于时间戳的字符串。
func GenerateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		LogError("crypto/rand.Read failed", "error", err)
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	// 设置 UUID v4 的版本和变体位。
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	return uuid
}

// GenerateTimestamp 返回当前时间的格式化字符串，精确到毫秒。
func GenerateTimestamp() string {
	now := time.Now()
	ms := now.UnixMilli() % 1000
	return fmt.Sprintf("%s.%03d", now.Format("2006-01-02 15:04:05"), ms)
}

// IsValidUUID 检查字符串是否为合法的 UUID 格式。
func IsValidUUID(uuid string) bool {
	return uuidRegex.MatchString(uuid)
}

// GetTempDir 返回操作系统临时目录路径。
func GetTempDir() string {
	temp := os.TempDir()
	if temp == "" {
		LogWarn("Failed to get temp dir")
		return ""
	}
	LogDebug("temp dir", "dir", temp)
	return temp
}
