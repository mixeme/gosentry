package core

import (
	"fmt"
	"os/exec"
	"strings"
)

const autostartName = "PySentry"

func SetAutostart(enabled bool, executablePath string) error {
	if enabled {
		// Remove any stale entry first. This makes "uncheck, save, check, save"
		// and even a plain "check, save" repair an old path after the executable
		// was moved or renamed for a new version.
		deleteCommand := exec.Command("reg.exe", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", autostartName, "/f")
		configureHiddenWindow(deleteCommand)
		_ = deleteCommand.Run()

		command := exec.Command("reg.exe", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", autostartName, "/t", "REG_SZ", "/d", fmt.Sprintf("%q", executablePath), "/f")
		configureHiddenWindow(command)
		return command.Run()
	}
	command := exec.Command("reg.exe", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", autostartName, "/f")
	configureHiddenWindow(command)
	_ = command.Run()
	return nil
}

func AutostartStatus(expectedEnabled bool, executablePath string) (bool, string) {
	command := exec.Command("reg.exe", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", autostartName)
	configureHiddenWindow(command)
	output, err := command.Output()
	if !expectedEnabled {
		if err != nil {
			return true, "Autostart is off"
		}
		return false, "Autostart entry exists while setting is off"
	}
	if err != nil {
		return false, "Autostart entry is missing"
	}

	text := strings.ReplaceAll(string(output), `"`, "")
	if !strings.Contains(text, executablePath) {
		return false, "Autostart points to another executable"
	}
	return true, "Autostart is configured"
}
