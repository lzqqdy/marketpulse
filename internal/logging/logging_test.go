package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestSetupWritesLogsByDateAndLevel(t *testing.T) {
	dir := t.TempDir()
	if err := Setup(dir); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	slog.Info("info message", "symbol", "hsi")
	slog.Warn("warn message", "provider", "sina")
	slog.Error("error message", "provider", "eastmoney")

	tests := []struct {
		level logrus.Level
		want  string
	}{
		{logrus.InfoLevel, "info message"},
		{logrus.WarnLevel, "warn message"},
		{logrus.ErrorLevel, "error message"},
	}
	for _, tt := range tests {
		path := PathFor(dir, time.Now(), tt.level)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		if !strings.Contains(string(data), tt.want) {
			t.Fatalf("%s does not contain %q: %s", filepath.Base(path), tt.want, data)
		}
	}
}

func TestCleanupOldLogsKeepsAtMostSevenDateDirs(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.Local)
	names := []string{
		"2026-05-10",
		"2026-05-11",
		"2026-05-12",
		"2026-05-18",
		"not-a-date",
	}
	for _, name := range names {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", name, err)
		}
	}

	if err := cleanupOldLogs(dir, now); err != nil {
		t.Fatalf("cleanupOldLogs() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026-05-11")); !os.IsNotExist(err) {
		t.Fatalf("expected 2026-05-11 to be removed, stat error = %v", err)
	}
	for _, name := range []string{"2026-05-12", "2026-05-18", "not-a-date"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to remain, stat error = %v", name, err)
		}
	}
}
