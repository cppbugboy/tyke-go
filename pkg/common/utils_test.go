package common

import (
	"strings"
	"testing"
)

func TestGenerateUUID(t *testing.T) {
	id := GenerateUUID()
	if !IsValidUUID(id) {
		t.Errorf("GenerateUUID() = %q, not a valid UUID", id)
	}
}

func TestGenerateUUIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := GenerateUUID()
		if ids[id] {
			t.Errorf("duplicate UUID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestGenerateTimestamp(t *testing.T) {
	ts := GenerateTimestamp()
	if len(ts) == 0 {
		t.Error("GenerateTimestamp() returned empty string")
	}
	if !strings.Contains(ts, "-") || !strings.Contains(ts, ":") {
		t.Errorf("GenerateTimestamp() = %q, unexpected format", ts)
	}
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"6ba7b810-9dad-11d1-80b4-00c04fd430c8", true},
		{"", false},
		{"not-a-uuid", false},
		{"550e8400-e29b-41d4-a716", false},
		{"550e8400-e29b-41d4-a716-44665544000", false},
	}
	for _, tt := range tests {
		got := IsValidUUID(tt.input)
		if got != tt.want {
			t.Errorf("IsValidUUID(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestGetTempDir(t *testing.T) {
	dir := GetTempDir()
	if dir == "" {
		t.Error("GetTempDir() returned empty string")
	}
}
