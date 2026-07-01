//go:build linux

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallDesktopIntegrationWritesDesktopAndIcon(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)

	appID := "ru.mixeme.gosentry.desktop"
	executable := filepath.Join(dataHome, "bin", "gosentry")
	icon := []byte{0x89, 0x50, 0x4e, 0x47} // PNG magic prefix is enough for file presence

	iconPath, err := InstallDesktopIntegration(appID, executable, icon)
	if err != nil {
		t.Fatalf("InstallDesktopIntegration: %v", err)
	}

	if _, err := os.Stat(iconPath); err != nil {
		t.Fatalf("icon file: %v", err)
	}
	iconData, err := os.ReadFile(iconPath)
	if err != nil {
		t.Fatalf("read icon: %v", err)
	}
	if string(iconData) != string(icon) {
		t.Fatalf("icon bytes mismatch")
	}

	desktopPath := filepath.Join(dataHome, "applications", appID+".desktop")
	data, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatalf("read desktop entry: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "Name=GoSentry") {
		t.Fatalf("desktop entry missing Name: %s", text)
	}
	if !strings.Contains(text, "StartupWMClass="+appID) {
		t.Fatalf("desktop entry missing WM class: %s", text)
	}
	wantExec := "Exec=" + quoteDesktopExec(executable)
	if !strings.Contains(text, wantExec) {
		t.Fatalf("desktop entry exec = %s, want substring %q", text, wantExec)
	}
	if !strings.Contains(text, "Icon="+iconPath) {
		t.Fatalf("desktop entry missing Icon path: %s", text)
	}
}

func TestQuoteDesktopExecQuotesPath(t *testing.T) {
	got := quoteDesktopExec("/opt/Go Sentry/gosentry")
	if got != `"/opt/Go Sentry/gosentry"` {
		t.Errorf("quoteDesktopExec = %q", got)
	}
}
