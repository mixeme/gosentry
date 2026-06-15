//go:build linux

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const autostartDesktopFileName = "pysentry.desktop"

func SetAutostart(enabled bool, executablePath string) error {
	desktopPath, err := autostartDesktopPath()
	if err != nil {
		return err
	}
	if err := cleanupLegacySystemdAutostart(); err != nil {
		return err
	}

	if enabled {
		if err := os.MkdirAll(filepath.Dir(desktopPath), 0o755); err != nil {
			return err
		}
		desktopFile := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=PySentry
Comment=PySentry desktop scheduler
Exec=%s
Terminal=false
X-GNOME-Autostart-enabled=true
`, quoteDesktopExec(executablePath))
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
	if legacySystemdAutostartExists() {
		return false, "Legacy systemd autostart entry still exists"
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
	if !strings.Contains(string(data), "Exec="+quoteDesktopExec(executablePath)) {
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

func cleanupLegacySystemdAutostart() error {
	unitPath, err := legacySystemdUnitPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(unitPath); os.IsNotExist(err) {
		return nil
	}

	// Older PySentry builds used a systemd user unit for autostart. The current
	// Linux implementation uses XDG Autostart because PySentry is a GUI/tray
	// application and should be launched by the desktop session. Disable and
	// remove the old unit so the two mechanisms do not fight or start duplicates.
	_ = exec.Command("systemctl", "--user", "disable", "pysentry.service").Run()
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

func legacySystemdAutostartExists() bool {
	unitPath, err := legacySystemdUnitPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(unitPath)
	return err == nil
}

func legacySystemdUnitPath() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "systemd", "user", "pysentry.service"), nil
}
