//go:build !windows

package winproc

import "os/exec"

// ConfigureHiddenWindow is a no-op on non-Windows platforms: launching sh -c
// from a desktop process does not create a new console window in the same way
// Windows does.
func ConfigureHiddenWindow(command *exec.Cmd) {}
