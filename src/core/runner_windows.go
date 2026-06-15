package core

import (
	"os/exec"
	"syscall"
)

func configureHiddenWindow(command *exec.Cmd) {
	// PySentry is a GUI scheduler, so child commands should not flash a console
	// window on Windows. CREATE_NO_WINDOW keeps cmd.exe and simple console tools
	// quiet while stdout/stderr are still captured through pipes.
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000,
		HideWindow:    true,
	}
}
