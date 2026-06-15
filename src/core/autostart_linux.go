//go:build linux

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const autostartUnitName = "pysentry.service"

func SetAutostart(enabled bool, executablePath string) error {
	unitDir, err := userSystemdDir()
	if err != nil {
		return err
	}
	unitPath := filepath.Join(unitDir, autostartUnitName)

	if enabled {
		if err := os.MkdirAll(unitDir, 0o755); err != nil {
			return err
		}
		unit := fmt.Sprintf(`[Unit]
Description=PySentry desktop scheduler

[Service]
ExecStart=%s
Restart=on-failure

[Install]
WantedBy=default.target
`, executablePath)
		if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
			return err
		}
		if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
			return err
		}
		return exec.Command("systemctl", "--user", "enable", "--now", autostartUnitName).Run()
	}

	_ = exec.Command("systemctl", "--user", "disable", "--now", autostartUnitName).Run()
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return exec.Command("systemctl", "--user", "daemon-reload").Run()
}

func AutostartStatus(expectedEnabled bool, executablePath string) (bool, string) {
	unitDir, err := userSystemdDir()
	if err != nil {
		return false, "Cannot resolve user systemd directory"
	}
	unitPath := filepath.Join(unitDir, autostartUnitName)
	data, readErr := os.ReadFile(unitPath)
	enabledErr := exec.Command("systemctl", "--user", "is-enabled", "--quiet", autostartUnitName).Run()

	if !expectedEnabled {
		if os.IsNotExist(readErr) && enabledErr != nil {
			return true, "Autostart is off"
		}
		return false, "Autostart unit exists while setting is off"
	}
	if readErr != nil {
		return false, "Autostart unit is missing"
	}
	if !strings.Contains(string(data), executablePath) {
		return false, "Autostart unit points to another executable"
	}
	if enabledErr != nil {
		return false, "Autostart unit is not enabled"
	}
	return true, "Autostart is configured"
}

func userSystemdDir() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "systemd", "user"), nil
}
