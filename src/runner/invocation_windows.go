package runner

import (
	"context"
	"os/exec"
	"strings"
	"syscall"
	"unicode"
)

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	// cmd.exe keeps Windows users' expectations for commands such as "dir",
	// "copy", variable expansion, redirection, and .bat/.cmd wrappers.
	//
	// Go's normal Windows argument escaping turns embedded quotes into literal
	// backslash-quote sequences for cmd.exe. Supplying the raw command line keeps
	// commands like `"C:\Program Files\App\App.exe" "D:\file.txt"` executable.
	result := exec.CommandContext(ctx, "cmd.exe")
	result.SysProcAttr = &syscall.SysProcAttr{CmdLine: windowsShellCommandLine(command)}
	return result
}

func windowsShellCommandLine(command string) string {
	return `cmd.exe /S /C "` + quoteLeadingWindowsProgramPath(command) + `"`
}

func quoteLeadingWindowsProgramPath(command string) string {
	trimmed := strings.TrimLeftFunc(command, unicode.IsSpace)
	leadingWhitespace := command[:len(command)-len(trimmed)]
	if trimmed == "" || strings.HasPrefix(trimmed, `"`) || !startsWithWindowsRootedPath(trimmed) {
		return command
	}

	lower := strings.ToLower(trimmed)
	for _, extension := range []string{".exe", ".cmd", ".bat", ".com"} {
		index := strings.Index(lower, extension)
		if index < 0 {
			continue
		}
		pathEnd := index + len(extension)
		programPath := trimmed[:pathEnd]
		if !strings.ContainsFunc(programPath, unicode.IsSpace) {
			return command
		}
		return leadingWhitespace + `"` + programPath + `"` + trimmed[pathEnd:]
	}
	return command
}

func startsWithWindowsRootedPath(command string) bool {
	if strings.HasPrefix(command, `\\`) {
		return true
	}
	return len(command) >= 3 &&
		((command[0] >= 'A' && command[0] <= 'Z') || (command[0] >= 'a' && command[0] <= 'z')) &&
		command[1] == ':' &&
		(command[2] == '\\' || command[2] == '/')
}
