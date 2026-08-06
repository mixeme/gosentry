//go:build linux

package autostart

import (
	"os"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

func TestLinuxAutostartStartsInTray(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	executablePath := "/opt/Go Sentry/gosentry"
	if err := setAutostart(true, true, executablePath, "/opt/Go Sentry/gosentry.png"); err != nil {
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

	expectedExec := "Exec=" + quoteDesktopExec(executablePath) + " " + domain.StartInTrayArgument
	if !strings.Contains(string(data), expectedExec) {
		t.Fatalf("desktop entry does not start in tray: %s", data)
	}
}

func TestLinuxAutostartWithoutTrayFlag(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	executablePath := "/opt/Go Sentry/gosentry"
	if err := setAutostart(true, false, executablePath, ""); err != nil {
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

	expectedExec := "Exec=" + quoteDesktopExec(executablePath)
	if !strings.Contains(string(data), expectedExec) {
		t.Fatalf("desktop entry should not include tray flag: %s", data)
	}
	if strings.Contains(string(data), domain.StartInTrayArgument) {
		t.Fatalf("desktop entry must not pass --start-in-tray: %s", data)
	}
}

