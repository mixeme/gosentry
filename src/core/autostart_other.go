//go:build !windows && !linux

package core

import "fmt"

func SetAutostart(enabled bool, executablePath string) error {
	if !enabled {
		return nil
	}
	return fmt.Errorf("autostart is not implemented for this platform")
}

func AutostartStatus(expectedEnabled bool, executablePath string) (bool, string) {
	if !expectedEnabled {
		return true, "Autostart is off"
	}
	return false, "Autostart is not implemented for this platform"
}
