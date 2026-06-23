//go:build windows

package runner

import (
	"context"
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"gitea.mixdep.ru/mix/gosentry/src/platform/winproc"
)

func TestDirectCommandDoesNotHideWindow(t *testing.T) {
	invocation := jobInvocation(context.Background(), domain.Job{
		Command:   `C:\Windows\System32\cmd.exe`,
		Arguments: "/C\necho visible direct process",
	})
	if invocation.hideWindow {
		t.Fatal("direct command should not request hidden startup window")
	}
}

func TestShellCommandHidesWindow(t *testing.T) {
	invocation := jobInvocation(context.Background(), domain.Job{Command: "echo hidden shell process"})
	if !invocation.hideWindow {
		t.Fatal("shell command should request hidden startup window")
	}
	winproc.ConfigureHiddenWindow(invocation.command)
	if invocation.command.SysProcAttr == nil || !invocation.command.SysProcAttr.HideWindow {
		t.Fatal("expected shell command to be hidden")
	}
}

func TestShellCommandUsesWindowsSafeQuoting(t *testing.T) {
	command := shellCommand(context.Background(), `"C:\Program Files\FreeFileSync\FreeFileSync.exe" "D:\Local\Programs\FreeFileSync\Jobs\Auto.ffs_batch"`)
	winproc.ConfigureHiddenWindow(command)

	want := `cmd.exe /S /C ""C:\Program Files\FreeFileSync\FreeFileSync.exe" "D:\Local\Programs\FreeFileSync\Jobs\Auto.ffs_batch""`
	if command.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr")
	}
	if command.SysProcAttr.CmdLine != want {
		t.Fatalf("expected command line %q, got %q", want, command.SysProcAttr.CmdLine)
	}
}

func TestWindowsShellCommandLineQuotesUnquotedProgramPath(t *testing.T) {
	got := windowsShellCommandLine(`C:\Program Files\Joplin\Joplin.exe --profile "D:\Joplin Profile"`)
	want := `cmd.exe /S /C ""C:\Program Files\Joplin\Joplin.exe" --profile "D:\Joplin Profile""`
	if got != want {
		t.Fatalf("expected command line %q, got %q", want, got)
	}
}
