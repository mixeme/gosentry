//go:build linux

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxAutostartStartsInTray(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	executablePath := "/opt/Go Sentry/gosentry"
	if err := SetAutostart(true, executablePath, "/opt/Go Sentry/gosentry.png"); err != nil {
		t.Fatalf("enable autostart: %v", err)
	}

	desktopPath, err := autostartDesktopPath()
	if err != nil {
		t.Fatalf("resolve desktop path: %v", err)
	}
	data, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatalf("read desktop entry: %v", err)
	}

	expectedExec := "Exec=" + quoteDesktopExec(executablePath) + " " + StartInTrayArgument
	if !strings.Contains(string(data), expectedExec) {
		t.Fatalf("desktop entry does not start in tray: %s", data)
	}
}

func TestLinuxAutostartRemovesLegacyDesktopEntry(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	legacyPath, err := legacyAutostartDesktopPath()
	if err != nil {
		t.Fatalf("resolve legacy desktop path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("create legacy desktop directory: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("[Desktop Entry]\nName=PySentry\n"), 0o644); err != nil {
		t.Fatalf("write legacy desktop entry: %v", err)
	}

	if err := SetAutostart(true, "/opt/gosentry/gosentry", ""); err != nil {
		t.Fatalf("enable autostart: %v", err)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy desktop entry still exists or cannot be checked: %v", err)
	}
}
