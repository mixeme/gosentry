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

// TestQuoteLeadingWindowsProgramPathPicksEarliestBoundedExtension pins the
// fix for the quoting bug found in the whole-project review: the program
// path must end at the *earliest* extension match that sits at a token
// boundary, not the first extension in the .exe/.cmd/.bat/.com list order,
// and not a substring match inside another word.
func TestQuoteLeadingWindowsProgramPathPicksEarliestBoundedExtension(t *testing.T) {
	cases := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "bat with unquoted argument",
			command: `C:\My Tools\run.bat D:\in.txt`,
			want:    `"C:\My Tools\run.bat" D:\in.txt`,
		},
		{
			name:    "bat program with exe argument",
			command: `C:\My Tools\run.bat C:\Windows\System32\notepad.exe`,
			want:    `"C:\My Tools\run.bat" C:\Windows\System32\notepad.exe`,
		},
		{
			name:    "cmd program with exe argument",
			command: `C:\Program Files\App\deploy.cmd D:\stage\setup.exe`,
			want:    `"C:\Program Files\App\deploy.cmd" D:\stage\setup.exe`,
		},
		{
			name:    "exe substring inside directory name",
			command: `C:\dir.exexample\My Tool\run.bat`,
			want:    `"C:\dir.exexample\My Tool\run.bat"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := quoteLeadingWindowsProgramPath(tc.command); got != tc.want {
				t.Fatalf("quoteLeadingWindowsProgramPath(%q) = %q, want %q", tc.command, got, tc.want)
			}
		})
	}
}
