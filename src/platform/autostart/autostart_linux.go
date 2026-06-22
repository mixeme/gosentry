//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

type linuxManager struct{}

// New returns the Linux autostart Manager.
func New() Manager { return linuxManager{} }

func (linuxManager) Set(enabled bool, executablePath, iconPath string) error {
	return SetAutostart(enabled, executablePath, iconPath)
}

func (linuxManager) Status(expectedEnabled bool, executablePath string) (bool, string) {
	return AutostartStatus(expectedEnabled, executablePath)
}

const autostartDesktopFileName = "gosentry.desktop"

func SetAutostart(enabled bool, executablePath string, iconPath string) error {
	desktopPath, err := autostartDesktopPath()
	if err != nil {
		return err
	}
	if enabled {
		if err := os.MkdirAll(filepath.Dir(desktopPath), 0o755); err != nil {
			return err
		}
		desktopFile := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=GoSentry
Comment=GoSentry desktop scheduler
Exec=%s %s
%s
Terminal=false
X-GNOME-Autostart-enabled=true
`, quoteDesktopExec(executablePath), domain.StartInTrayArgument, desktopIconLine(iconPath))
		return os.WriteFile(desktopPath, []byte(desktopFile), 0o644)
	}

	if err := os.Remove(desktopPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func AutostartStatus(expectedEnabled bool, executablePath string) (bool, string) {
	desktopPath, err := autostartDesktopPath()
	if err != nil {
		return false, "Cannot resolve XDG autostart directory"
	}
	data, readErr := os.ReadFile(desktopPath)

	if !expectedEnabled {
		if os.IsNotExist(readErr) {
			return true, "Autostart is off"
		}
		return false, "Autostart desktop entry exists while setting is off"
	}
	if readErr != nil {
		return false, "Autostart desktop entry is missing"
	}
	expectedExec := "Exec=" + quoteDesktopExec(executablePath) + " " + domain.StartInTrayArgument
	if !strings.Contains(string(data), expectedExec) {
		return false, "Autostart desktop entry points to another executable"
	}
	return true, "Autostart is configured"
}

func autostartDesktopPath() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "autostart", autostartDesktopFileName), nil
}

func quoteDesktopExec(path string) string {
	return strconv.Quote(path)
}

func desktopIconLine(iconPath string) string {
	if strings.TrimSpace(iconPath) == "" {
		return ""
	}
	return "Icon=" + iconPath
}

