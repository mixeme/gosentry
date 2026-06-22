//go:build !windows

package runner

import (
	"context"
	"os/exec"
)

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	// sh -c is the portable baseline for Linux builds. It keeps the runner small
	// and avoids a hard dependency on a larger shell such as bash.
	return exec.CommandContext(ctx, "sh", "-c", command)
}
