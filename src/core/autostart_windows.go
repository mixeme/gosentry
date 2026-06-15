package core

import (
	"os/exec"
	"path/filepath"
	"strconv"
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

		command := exec.Command("reg.exe", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", autostartName, "/t", "REG_SZ", "/d", strconv.Quote(executablePath), "/f")
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

	actual, ok := parseRegistryRunValue(string(output))
	if !ok {
		return false, "Autostart entry cannot be read"
	}
	if !sameWindowsPath(actual, executablePath) {
		return false, "Autostart points to another executable"
	}
	return true, "Autostart is configured"
}

func parseRegistryRunValue(output string) (string, bool) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		for index, field := range fields {
			if field == "REG_SZ" && index+1 < len(fields) {
				value := strings.Join(fields[index+1:], " ")
				value = strings.Trim(value, `"`)
				return value, value != ""
			}
		}
	}
	return "", false
}

func sameWindowsPath(left string, right string) bool {
	left = filepath.Clean(strings.Trim(left, `"`))
	right = filepath.Clean(strings.Trim(right, `"`))
	return strings.EqualFold(left, right)
}
