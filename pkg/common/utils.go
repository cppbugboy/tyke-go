// Copyright 2026 Tyke Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// uuidPattern UUID格式的正则表达式，用于验证UUID字符串。
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// GenerateUUID 生成一个符合RFC 4122标准的v4版本UUID字符串。
//
// 返回格式为 "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" 的36字符字符串，
// 例如 "550e8400-e29b-41d4-a716-446655440000"。
// UUID v4 是随机生成的，具有极低的碰撞概率。
//
// 返回值:
//   - string: 新生成的UUID字符串
//
// 示例:
//
//	uuid := GenerateUUID()
//	fmt.Println(uuid) // 输出: 550e8400-e29b-41d4-a716-446655440000
func GenerateUUID() string {
	return uuid.New().String()
}

// GenerateTimestamp 生成当前时间的时间戳字符串。
//
// 返回格式为 "YYYY-MM-DD HH:MM:SS.mmm" 的字符串，
// 精确到毫秒级，便于日志记录和调试。
//
// 返回值:
//   - string: 格式化的时间戳字符串
//
// 示例:
//
//	ts := GenerateTimestamp()
//	fmt.Println(ts) // 输出: 2026-04-17 12:30:45.123
func GenerateTimestamp() string {
	now := time.Now()
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d.%03d",
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second(),
		now.Nanosecond()/1000000)
}

// IsValidUUID 验证字符串是否为有效的UUID格式。
//
// 检查字符串是否符合标准UUID格式：
// "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
// 其中x为十六进制字符（0-9, a-f, A-F）。
//
// 参数:
//   - id: 待验证的字符串
//
// 返回值:
//   - bool: true表示是有效的UUID格式，false表示无效
//
// 示例:
//
//	IsValidUUID("550e8400-e29b-41d4-a716-446655440000") // 返回 true
//	IsValidUUID("invalid-uuid")                          // 返回 false
//	IsValidUUID("")                                      // 返回 false
func IsValidUUID(id string) bool {
	return uuidPattern.MatchString(id)
}

// GetTempDir 获取系统临时目录路径。
//
// 返回操作系统的临时目录路径：
//   - Windows: 通常为 %TEMP% 或 C:\Users\<user>\AppData\Local\Temp
//   - Linux: 通常为 /tmp 或 $TMPDIR
//   - macOS: 通常为 /tmp 或 $TMPDIR
//
// 返回值:
//   - string: 临时目录路径，获取失败时返回空字符串
//
// 示例:
//
//	dir := GetTempDir()
//	fmt.Println(dir) // Windows: C:\Users\xxx\AppData\Local\Temp
//	                 // Linux: /tmp
func GetTempDir() string {
	dir := os.TempDir()
	if dir == "" {
		return ""
	}
	return dir
}
