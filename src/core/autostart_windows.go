package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const autostartName = "GoSentry"
const legacyAutostartName = "PySentry"
const startupShortcutFile = autostartName + ".lnk"

func SetAutostart(enabled bool, executablePath string, iconPath string) error {
	if err := cleanupLegacyRegistryAutostart(); err != nil {
		return err
	}

	shortcutPath, err := startupShortcutPath()
	if err != nil {
		return err
	}

	if enabled {
		return createStartupShortcut(shortcutPath, executablePath, iconPath)
	}
	return removeIfExists(shortcutPath)
}

func AutostartStatus(expectedEnabled bool, executablePath string) (bool, string) {
	shortcutPath, err := startupShortcutPath()
	if err != nil {
		return false, "Startup folder cannot be resolved"
	}
	_, statErr := os.Stat(shortcutPath)
	if !expectedEnabled {
		if os.IsNotExist(statErr) {
			if legacyRegistryAutostartExists() {
				return false, "Legacy registry autostart exists; save settings to repair"
			}
			return true, "Autostart is off"
		}
		if statErr != nil {
			return false, "Autostart shortcut cannot be checked"
		}
		return false, "Autostart shortcut exists while setting is off"
	}

	if os.IsNotExist(statErr) {
		if legacyRegistryAutostartExists() {
			return false, "Legacy registry autostart exists; save settings to repair"
		}
		return false, "Autostart shortcut is missing"
	}
	if statErr != nil {
		return false, "Autostart shortcut cannot be checked"
	}

	actual, arguments, err := readShortcut(shortcutPath)
	if err != nil {
		return false, "Autostart shortcut cannot be read"
	}
	if !sameWindowsPath(actual, executablePath) {
		return false, "Autostart shortcut points to another executable"
	}
	if strings.TrimSpace(arguments) != StartInTrayArgument {
		return false, "Autostart shortcut does not start in tray"
	}
	return true, "Autostart is configured"
}

func startupShortcutPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA is not set")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", startupShortcutFile), nil
}

func createStartupShortcut(shortcutPath string, executablePath string, iconPath string) error {
	if err := os.MkdirAll(filepath.Dir(shortcutPath), 0755); err != nil {
		return err
	}

	workingDirectory := filepath.Dir(executablePath)
	if iconPath == "" {
		iconPath = executablePath
	}
	script := `$shell = New-Object -ComObject WScript.Shell; $shortcut = $shell.CreateShortcut($env:GOSENTRY_SHORTCUT_PATH); $shortcut.TargetPath = $env:GOSENTRY_TARGET_PATH; $shortcut.Arguments = $env:GOSENTRY_ARGUMENTS; $shortcut.WorkingDirectory = $env:GOSENTRY_WORKING_DIRECTORY; $shortcut.IconLocation = $env:GOSENTRY_ICON_PATH; $shortcut.Save()`
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	command.Env = append(os.Environ(),
		"GOSENTRY_SHORTCUT_PATH="+shortcutPath,
		"GOSENTRY_TARGET_PATH="+executablePath,
		"GOSENTRY_ARGUMENTS="+StartInTrayArgument,
		"GOSENTRY_WORKING_DIRECTORY="+workingDirectory,
		"GOSENTRY_ICON_PATH="+iconPath,
	)
	configureHiddenWindow(command)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("create startup shortcut: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func readShortcut(shortcutPath string) (string, string, error) {
	script := `$shell = New-Object -ComObject WScript.Shell; $shortcut = $shell.CreateShortcut($env:GOSENTRY_SHORTCUT_PATH); [Console]::Out.Write($shortcut.TargetPath + [Environment]::NewLine + $shortcut.Arguments)`
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	command.Env = append(os.Environ(), "GOSENTRY_SHORTCUT_PATH="+shortcutPath)
	configureHiddenWindow(command)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("read startup shortcut: %w: %s", err, strings.TrimSpace(string(output)))
	}
	lines := strings.SplitN(string(output), "\n", 2)
	target := strings.TrimSpace(lines[0])
	arguments := ""
	if len(lines) > 1 {
		arguments = strings.TrimSpace(lines[1])
	}
	return target, arguments, nil
}

func readShortcutTarget(shortcutPath string) (string, error) {
	target, _, err := readShortcut(shortcutPath)
	return target, err
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func cleanupLegacyRegistryAutostart() error {
	for _, name := range []string{legacyAutostartName, autostartName} {
		command := exec.Command("reg.exe", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", name, "/f")
		configureHiddenWindow(command)
		_ = command.Run()
	}
	return nil
}

func legacyRegistryAutostartExists() bool {
	for _, name := range []string{legacyAutostartName, autostartName} {
		command := exec.Command("reg.exe", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", name)
		configureHiddenWindow(command)
		if command.Run() == nil {
			return true
		}
	}
	return false
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
