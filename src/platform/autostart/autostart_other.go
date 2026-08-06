//go:build !windows && !linux

package autostart

import "fmt"

type otherManager struct{}

// New returns the stub autostart Manager for unsupported platforms.
func New() Manager { return otherManager{} }

func (otherManager) Set(enabled, startInTray bool, executablePath, iconPath string) error {
	return setAutostart(enabled, startInTray, executablePath, iconPath)
}

func (otherManager) Status(expectedEnabled, startInTray bool, executablePath string) (bool, string) {
	return autostartStatus(expectedEnabled, startInTray, executablePath)
}

func setAutostart(enabled bool, startInTray bool, executablePath string, iconPath string) error {
	if !enabled {
		return nil
	}
	return fmt.Errorf("autostart is not implemented for this platform")
}

func autostartStatus(expectedEnabled bool, startInTray bool, executablePath string) (bool, string) {
	if !expectedEnabled {
		return true, "Autostart is off"
	}
	return false, "Autostart is not implemented for this platform"
}
