//go:build !windows

package core

import (
	"context"
	"os/exec"
)

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	// sh -c is the portable baseline for Linux builds. It keeps the runner small
	// and avoids a hard dependency on a larger shell such as bash.
	return exec.CommandContext(ctx, "sh", "-c", command)
}

func configureHiddenWindow(command *exec.Cmd) {
	// Non-Windows platforms do not create a new console window for sh -c from a
	// desktop process in the same way Windows does, so no extra process attribute
	// is required here.
}
