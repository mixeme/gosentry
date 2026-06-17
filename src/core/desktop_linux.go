//go:build linux

package core

import (
	"fmt"
	"os"
	"path/filepath"
)

func InstallDesktopIntegration(appID string, executablePath string, icon []byte) (string, error) {
	dataHome, err := xdgDataHome()
	if err != nil {
		return "", err
	}

	// The taskbar can only show the application icon reliably when the desktop
	// environment can match the window app id to an installed .desktop file and
	// icon. Use the user's XDG data directory so portable builds do not need root
	// access or a package manager install step.
	iconPath := filepath.Join(dataHome, "icons", "hicolor", "256x256", "apps", "gosentry.png")
	if err := writeUserFile(iconPath, icon, 0o644); err != nil {
		return "", err
	}

	desktopPath := filepath.Join(dataHome, "applications", appID+".desktop")
	desktopFile := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=GoSentry
Comment=GoSentry desktop scheduler
Exec=%s
Icon=%s
Terminal=false
Categories=Utility;
StartupWMClass=%s
`, quoteDesktopExec(executablePath), iconPath, appID)
	if err := writeUserFile(desktopPath, []byte(desktopFile), 0o644); err != nil {
		return "", err
	}
	return iconPath, nil
}

func xdgDataHome() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return dataHome, nil
}

func writeUserFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}
