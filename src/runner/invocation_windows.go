package runner

import (
	"context"
	"os/exec"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
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
	pathEnd := -1
	for _, extension := range []string{".exe", ".cmd", ".bat", ".com"} {
		end := earliestBoundedExtensionEnd(lower, extension)
		if end >= 0 && (pathEnd < 0 || end < pathEnd) {
			pathEnd = end
		}
	}
	if pathEnd < 0 {
		return command
	}
	programPath := trimmed[:pathEnd]
	if !strings.ContainsFunc(programPath, unicode.IsSpace) {
		return command
	}
	return leadingWhitespace + `"` + programPath + `"` + trimmed[pathEnd:]
}

// earliestBoundedExtensionEnd returns the offset just past the first
// occurrence of extension in s that ends at a token boundary (end of string
// or whitespace), or -1 if none does. Scanning left to right and rejecting
// unbounded matches keeps a trailing "...\App.exe" inside an argument, such
// as "run.bat C:\tool.exe", from being mistaken for the program path.
func earliestBoundedExtensionEnd(s, extension string) int {
	offset := 0
	for {
		index := strings.Index(s[offset:], extension)
		if index < 0 {
			return -1
		}
		end := offset + index + len(extension)
		if end == len(s) {
			return end
		}
		r, _ := utf8.DecodeRuneInString(s[end:])
		if unicode.IsSpace(r) {
			return end
		}
		offset += index + 1
	}
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
