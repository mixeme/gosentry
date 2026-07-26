// Package filemanager opens a directory in the desktop file manager, so the
// UI can reveal a configured folder (logs, jobs) without knowing which handler
// the platform uses.
package filemanager

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// Open shows dir in the platform file manager. A missing path, a path that is
// not a directory, and a handler that fails to start are all returned as
// errors so the caller can surface them instead of appearing to do nothing.
func Open(dir string) error {
	info, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("folder does not exist: %s", dir)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a folder: %s", dir)
	}
	name, args := openCommand(dir)
	if name == "" {
		return fmt.Errorf("opening a folder is not supported on %s", runtime.GOOS)
	}
	command := exec.Command(name, args...)
	if err := command.Start(); err != nil {
		return err
	}
	// The handler hands the request to the desktop shell and exits on its own —
	// Windows Explorer even exits non-zero after opening the window — so its
	// status carries no information. Wait runs only to release the process
	// handle, and never blocks the caller.
	go func() { _ = command.Wait() }()
	return nil
}
