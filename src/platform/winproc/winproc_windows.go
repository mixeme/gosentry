package winproc

import (
	"os/exec"
	"syscall"
)

// ConfigureHiddenWindow suppresses the console window that Windows would
// otherwise flash when running a child process from a GUI application.
// CREATE_NO_WINDOW keeps cmd.exe and simple console tools quiet while
// stdout/stderr are still captured through pipes.
func ConfigureHiddenWindow(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.CreationFlags |= 0x08000000
	command.SysProcAttr.HideWindow = true
}
