package common

import (
	"crypto/rand"
	"fmt"
	"os"
	"regexp"
	"time"
)

var uuidRegex = regexp.MustCompile(`^{?[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}}?$`)

func GenerateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	return uuid
}

func GenerateTimestamp() string {
	now := time.Now()
	ms := now.UnixMilli() % 1000
	return fmt.Sprintf("%s.%03d", now.Format("2006-01-02 15:04:05"), ms)
}

func IsValidUUID(uuid string) bool {
	return uuidRegex.MatchString(uuid)
}

func GetTempDir() string {
	temp := os.TempDir()
	if temp == "" {
		LogWarn("Failed to get temp dir")
		return ""
	}
	LogDebug("temp dir", "dir", temp)
	return temp
}
