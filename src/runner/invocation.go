package runner

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

type commandInvocation struct {
	command    *exec.Cmd
	hideWindow bool
}

func jobInvocation(ctx context.Context, job domain.Job) commandInvocation {
	command := strings.TrimSpace(job.Command)
	arguments := commandArguments(job.Arguments)
	if len(arguments) > 0 || commandPathExists(command) {
		return commandInvocation{
			command:    exec.CommandContext(ctx, unquoteCommandPath(command), arguments...),
			hideWindow: false,
		}
	}

	// Shell mode remains for existing jobs and for commands that intentionally
	// use builtins, redirection, variables, or chained command syntax.
	return commandInvocation{
		command:    shellCommand(ctx, command),
		hideWindow: true,
	}
}

func commandArguments(arguments string) []string {
	var result []string
	for _, line := range strings.FieldsFunc(arguments, func(r rune) bool {
		return r == '\n' || r == '\r'
	}) {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func commandPathExists(command string) bool {
	command = unquoteCommandPath(strings.TrimSpace(command))
	if command == "" {
		return false
	}
	info, err := os.Stat(command)
	return err == nil && !info.IsDir()
}

func unquoteCommandPath(command string) string {
	return strings.Trim(strings.TrimSpace(command), `"`)
}

func LogArguments(arguments string) string {
	if strings.TrimSpace(arguments) == "" {
		return "<empty>"
	}
	return strings.ReplaceAll(strings.TrimSpace(arguments), "\r\n", "\n")
}

func logArguments(arguments string) string { return LogArguments(arguments) }
