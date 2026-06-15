//go:build !windows

package core

import "os/exec"

func configureHiddenWindow(command *exec.Cmd) {
	// Non-Windows platforms do not create a new console window for sh -c from a
	// desktop process in the same way Windows does, so no extra process attribute
	// is required here.
}
