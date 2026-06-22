package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"gitea.mixdep.ru/mix/gosentry/src/platform/winproc"
)

type windowsManager struct{}

// New returns the Windows autostart Manager.
func New() Manager { return windowsManager{} }

func (windowsManager) Set(enabled bool, executablePath, iconPath string) error {
	return SetAutostart(enabled, executablePath, iconPath)
}

func (windowsManager) Status(expectedEnabled bool, executablePath string) (bool, string) {
	return AutostartStatus(expectedEnabled, executablePath)
}

const autostartName = "GoSentry"
const legacyAutostartName = "PySentry"
const startupShortcutFile = autostartName + ".lnk"

func SetAutostart(enabled bool, executablePath string, iconPath string) error {
	// Windows autostart used to write HKCU\Run values, but that approach became
	// brittle once paths with spaces and the "--start-in-tray" argument entered
	// the picture. A Startup-folder shortcut stores target path and arguments as
	// separate structured fields, so it avoids quoting bugs and more closely
	// matches how a user would configure a GUI app by hand.
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
	if strings.TrimSpace(arguments) != domain.StartInTrayArgument {
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
	// WScript.Shell is used here deliberately instead of a third-party Go COM
	// wrapper. The PowerShell bridge is not glamorous, but it is already present
	// on supported Windows systems and keeps the dependency surface much smaller
	// for a project that otherwise aims to stay light.
	script := `$shell = New-Object -ComObject WScript.Shell; $shortcut = $shell.CreateShortcut($env:GOSENTRY_SHORTCUT_PATH); $shortcut.TargetPath = $env:GOSENTRY_TARGET_PATH; $shortcut.Arguments = $env:GOSENTRY_ARGUMENTS; $shortcut.WorkingDirectory = $env:GOSENTRY_WORKING_DIRECTORY; $shortcut.IconLocation = $env:GOSENTRY_ICON_PATH; $shortcut.Save()`
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	command.Env = append(os.Environ(),
		"GOSENTRY_SHORTCUT_PATH="+shortcutPath,
		"GOSENTRY_TARGET_PATH="+executablePath,
		"GOSENTRY_ARGUMENTS="+domain.StartInTrayArgument,
		"GOSENTRY_WORKING_DIRECTORY="+workingDirectory,
		"GOSENTRY_ICON_PATH="+iconPath,
	)
	winproc.ConfigureHiddenWindow(command)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("create startup shortcut: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func readShortcut(shortcutPath string) (string, string, error) {
	// Force UTF-8 before writing the path. PowerShell defaults to the system
	// OEM code page (e.g. CP866 on Russian Windows). Without this override,
	// [Console]::Out.Write encodes Cyrillic and other non-ASCII characters as
	// OEM bytes; Go then reads them as UTF-8 and gets a different string from
	// os.Executable, causing AutostartStatus to report "shortcut points to
	// another executable" for any install path that contains non-ASCII chars.
	// New-Object System.Text.UTF8Encoding($false) is UTF-8 without BOM.
	script := `[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false); $shell = New-Object -ComObject WScript.Shell; $shortcut = $shell.CreateShortcut($env:GOSENTRY_SHORTCUT_PATH); [Console]::Out.Write($shortcut.TargetPath + [Environment]::NewLine + $shortcut.Arguments)`
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	command.Env = append(os.Environ(), "GOSENTRY_SHORTCUT_PATH="+shortcutPath)
	winproc.ConfigureHiddenWindow(command)
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
		winproc.ConfigureHiddenWindow(command)
		_ = command.Run()
	}
	return nil
}

func legacyRegistryAutostartExists() bool {
	for _, name := range []string{legacyAutostartName, autostartName} {
		command := exec.Command("reg.exe", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", name)
		winproc.ConfigureHiddenWindow(command)
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
	left = normalizeWindowsPath(left)
	right = normalizeWindowsPath(right)
	if strings.EqualFold(left, right) {
		return true
	}
	// If the string comparison fails, compare by filesystem object identity.
	// os.SameFile uses the volume serial number and file index on Windows, so
	// it correctly handles cases where one path uses an NTFS 8.3 short name
	// while the other uses the long name. Windows generates 8.3 names for
	// directory entries that contain spaces; when the process is launched via
	// a Startup-folder shortcut the OS may resolve the PIDL to the short-name
	// form, so os.Executable can return a different string than WScript reads
	// back from TargetPath even though both point to the same file. The same
	// fallback also covers directory junction points.
	leftInfo, leftErr := os.Lstat(left)
	rightInfo, rightErr := os.Lstat(right)
	if leftErr == nil && rightErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}
	return false
}

func normalizeWindowsPath(p string) string {
	p = strings.Trim(p, `"`)
	// filepath.Clean preserves the \\?\ extended-length device path prefix that
	// Windows adds for paths exceeding MAX_PATH. Strip it so the cleaned result
	// compares equal to the same path without the prefix.
	p = strings.TrimPrefix(p, `\\?\`)
	return filepath.Clean(p)
}
