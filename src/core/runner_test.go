package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunJobWritesLogFile(t *testing.T) {
	logsDir := t.TempDir()
	job := Job{
		ID:      42,
		Name:    "Hello Test",
		Command: echoCommand("hello from test"),
	}

	record := RunJob(context.Background(), &job, "Manual", logsDir)
	if record.LogFile == "" {
		t.Fatal("expected log file path")
	}
	if filepath.Dir(record.LogFile) != logsDir {
		t.Fatalf("expected log in %q, got %q", logsDir, record.LogFile)
	}
	if !strings.Contains(filepath.Base(record.LogFile), "Hello_Test") {
		t.Fatalf("expected job name in log filename, got %q", record.LogFile)
	}

	data, err := os.ReadFile(record.LogFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"trigger: Manual", "job_name: Hello Test", "hello from test"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected log content to contain %q, got:\n%s", want, content)
		}
	}
}

func TestRunJobRunsQuotedWindowsExecutable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows cmd.exe quoting only")
	}

	logsDir := t.TempDir()
	job := Job{
		ID:      43,
		Name:    "Quoted Windows Command",
		Command: `"C:\Windows\System32\cmd.exe" /C echo quoted command ok`,
	}

	record := RunJob(context.Background(), &job, "Manual", logsDir)
	if record.State != "OK" {
		t.Fatalf("expected quoted command to run, got state %q detail %q output:\n%s", record.State, record.Detail, record.Output)
	}
	if !strings.Contains(record.Output, "quoted command ok") {
		t.Fatalf("expected command output, got:\n%s", record.Output)
	}
}

func TestRunJobRunsUnquotedWindowsProgramPathWithSpaces(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows cmd.exe quoting only")
	}

	logsDir := t.TempDir()
	scriptDir := filepath.Join(t.TempDir(), "Program Files", "GoSentry Test")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(scriptDir, "hello.cmd")
	if err := os.WriteFile(scriptPath, []byte("@echo off\r\necho unquoted command ok\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	job := Job{
		ID:      44,
		Name:    "Unquoted Windows Command",
		Command: scriptPath,
	}

	record := RunJob(context.Background(), &job, "Manual", logsDir)
	if record.State != "OK" {
		t.Fatalf("expected unquoted command path to run, got state %q detail %q output:\n%s", record.State, record.Detail, record.Output)
	}
	if !strings.Contains(record.Output, "unquoted command ok") {
		t.Fatalf("expected command output, got:\n%s", record.Output)
	}
}

func TestRunJobRunsWindowsCommandWithSeparateArguments(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows command arguments only")
	}

	logsDir := t.TempDir()
	job := Job{
		ID:        45,
		Name:      "Separate Arguments",
		Command:   `C:\Windows\System32\cmd.exe`,
		Arguments: "/C\necho separate arguments ok",
	}

	record := RunJob(context.Background(), &job, "Manual", logsDir)
	if record.State != "OK" {
		t.Fatalf("expected separate arguments to run, got state %q detail %q output:\n%s", record.State, record.Detail, record.Output)
	}
	if !strings.Contains(record.Output, "separate arguments ok") {
		t.Fatalf("expected command output, got:\n%s", record.Output)
	}
}

func TestRunJobAcceptsConfiguredExitCode(t *testing.T) {
	command := `sh -c 'exit 1'`
	if runtime.GOOS == "windows" {
		command = `C:\Windows\System32\cmd.exe`
	}
	job := Job{
		ID:               46,
		Name:             "Accepted Exit Code",
		Command:          command,
		SuccessExitCodes: "0,1",
	}
	if runtime.GOOS == "windows" {
		job.Arguments = "/C\nexit /b 1"
	}

	record := RunJob(context.Background(), &job, "Manual", t.TempDir())
	if record.State != "OK" {
		t.Fatalf("expected accepted exit code to be OK, got state %q detail %q", record.State, record.Detail)
	}
	if !strings.Contains(record.Detail, "accepted exit code 1") {
		t.Fatalf("expected accepted exit code detail, got %q", record.Detail)
	}
}

func TestRunJobRejectsUnconfiguredExitCode(t *testing.T) {
	command := `sh -c 'exit 1'`
	if runtime.GOOS == "windows" {
		command = `C:\Windows\System32\cmd.exe`
	}
	job := Job{
		ID:               47,
		Name:             "Rejected Exit Code",
		Command:          command,
		SuccessExitCodes: "0",
	}
	if runtime.GOOS == "windows" {
		job.Arguments = "/C\nexit /b 1"
	}

	record := RunJob(context.Background(), &job, "Manual", t.TempDir())
	if record.State != "Failed" {
		t.Fatalf("expected rejected exit code to fail, got state %q detail %q", record.State, record.Detail)
	}
	if !strings.Contains(record.Detail, "Exit code 1") {
		t.Fatalf("expected exit code detail, got %q", record.Detail)
	}
}

func TestRunJobStartOnlyDoesNotWaitForExitCode(t *testing.T) {
	command := "sh"
	arguments := "-c\nexit 7"
	if runtime.GOOS == "windows" {
		command = `C:\Windows\System32\cmd.exe`
		arguments = "/C\nexit /b 7"
	}
	job := Job{
		ID:        48,
		Name:      "Start Only",
		Command:   command,
		Arguments: arguments,
		StartOnly: true,
	}

	record := RunJob(context.Background(), &job, "Manual", t.TempDir())
	if record.State != "OK" {
		t.Fatalf("expected start-only job to be OK after launch, got state %q detail %q", record.State, record.Detail)
	}
	if !strings.Contains(record.Detail, "not waiting for process exit") {
		t.Fatalf("expected start-only detail, got %q", record.Detail)
	}
	if !strings.Contains(record.Output, "start_only:\ntrue") {
		t.Fatalf("expected start-only output, got:\n%s", record.Output)
	}
}

func TestRunJobStartOnlyReportsStartFailure(t *testing.T) {
	job := Job{
		ID:        49,
		Name:      "Missing Start Only",
		Command:   "definitely-missing-gosentry-command",
		Arguments: "--force-direct-start",
		StartOnly: true,
	}

	record := RunJob(context.Background(), &job, "Manual", t.TempDir())
	if record.State != "Failed" {
		t.Fatalf("expected missing start-only command to fail, got state %q detail %q", record.State, record.Detail)
	}
	if !strings.Contains(record.Output, "Process did not start") {
		t.Fatalf("expected start failure output, got:\n%s", record.Output)
	}
}

func TestParseExitCodes(t *testing.T) {
	got := parseExitCodes("0, 1;2\n3")
	want := []int{0, 1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestDirectCommandDoesNotHideWindow(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows window visibility only")
	}

	invocation := jobInvocation(context.Background(), Job{
		Command:   `C:\Windows\System32\cmd.exe`,
		Arguments: "/C\necho visible direct process",
	})
	if invocation.hideWindow {
		t.Fatal("direct command should not request hidden startup window")
	}
}

func TestShellCommandHidesWindow(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows window visibility only")
	}

	invocation := jobInvocation(context.Background(), Job{Command: "echo hidden shell process"})
	if !invocation.hideWindow {
		t.Fatal("shell command should request hidden startup window")
	}
	configureHiddenWindow(invocation.command)
	if invocation.command.SysProcAttr == nil || !invocation.command.SysProcAttr.HideWindow {
		t.Fatal("expected shell command to be hidden")
	}
}

func TestShellCommandUsesWindowsSafeQuoting(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows cmd.exe quoting only")
	}

	command := shellCommand(context.Background(), `"C:\Program Files\FreeFileSync\FreeFileSync.exe" "D:\Local\Programs\FreeFileSync\Jobs\Auto.ffs_batch"`)
	configureHiddenWindow(command)

	want := `cmd.exe /S /C ""C:\Program Files\FreeFileSync\FreeFileSync.exe" "D:\Local\Programs\FreeFileSync\Jobs\Auto.ffs_batch""`
	if command.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr")
	}
	if command.SysProcAttr.CmdLine != want {
		t.Fatalf("expected command line %q, got %q", want, command.SysProcAttr.CmdLine)
	}
}

func TestWindowsShellCommandLineQuotesUnquotedProgramPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows cmd.exe quoting only")
	}

	got := windowsShellCommandLine(`C:\Program Files\Joplin\Joplin.exe --profile "D:\Joplin Profile"`)
	want := `cmd.exe /S /C ""C:\Program Files\Joplin\Joplin.exe" --profile "D:\Joplin Profile""`
	if got != want {
		t.Fatalf("expected command line %q, got %q", want, got)
	}
}
