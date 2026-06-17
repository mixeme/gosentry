package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const commandTimeout = 30 * time.Second
const commandWaitDelay = 2 * time.Second

func RunJob(ctx context.Context, job *Job, trigger string, logsDir string) RunRecord {
	started := time.Now()
	// Commands can hang forever if a script waits for input or a child process
	// stalls. A fixed timeout is a conservative first guardrail for a desktop
	// scheduler; later it can become a per-job setting without changing the
	// runner contract.
	runCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var output string
	var state string
	var detail string
	if job.StartOnly {
		invocation := jobInvocation(context.Background(), *job)
		state, detail, output = startJobOnly(invocation, *job, started)
	} else {
		invocation := jobInvocation(runCtx, *job)
		command := invocation.command
		command.WaitDelay = commandWaitDelay
		if invocation.hideWindow {
			configureHiddenWindow(command)
		}
		command.Stdout = &stdout
		command.Stderr = &stderr

		err := command.Run()
		duration := time.Since(started).Round(time.Millisecond)
		output = formatOutput(stdout.String(), stderr.String())
		state, detail = runStateDetail(err, runCtx.Err(), duration, *job)
	}

	now := time.Now()
	job.LastRun = now.Format("2006-01-02 15:04:05")
	job.LastState = state
	job.Output = output
	logFile := writeRunLog(logsDir, *job, trigger, state, detail, output, now)

	record := RunRecord{
		Time:    job.LastRun,
		JobID:   job.ID,
		JobName: job.Name,
		Trigger: trigger,
		State:   state,
		Detail:  detail,
		LogFile: logFile,
		Output:  output,
	}
	// Keep a small in-memory history for the currently running GUI. Full command
	// output is persisted to files, so retaining every past record in RAM would
	// only duplicate data and make long sessions grow without bound.
	job.Logs = append([]RunRecord{record}, job.Logs...)
	if len(job.Logs) > 50 {
		job.Logs = job.Logs[:50]
	}
	return record
}

func CleanupLogs(logsDir string, maxFiles int, maxAgeDays int) error {
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	type logFile struct {
		path    string
		modTime time.Time
	}
	var logs []logFile
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	for _, entry := range entries {
		// Only GoSentry run logs are managed here. Directories and non-.log files
		// are intentionally ignored so the user can keep notes or other artifacts
		// in the same folder without the cleanup policy deleting them.
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".log") {
			continue
		}
		path := filepath.Join(logsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if maxAgeDays > 0 && info.ModTime().Before(cutoff) {
			// Cleanup is best-effort: failing to delete one file should not block
			// the scheduler from running future jobs.
			_ = os.Remove(path)
			continue
		}
		logs = append(logs, logFile{path: path, modTime: info.ModTime()})
	}

	if maxFiles <= 0 || len(logs) <= maxFiles {
		return nil
	}
	sort.Slice(logs, func(i int, j int) bool {
		// Newest files are kept first, then everything after maxFiles is removed.
		// This matches the user's expectation that the most recent failures and
		// command output remain available for investigation.
		return logs[i].modTime.After(logs[j].modTime)
	})
	for _, old := range logs[maxFiles:] {
		_ = os.Remove(old.path)
	}
	return nil
}

func writeRunLog(logsDir string, job Job, trigger string, state string, detail string, output string, started time.Time) string {
	if strings.TrimSpace(logsDir) == "" {
		return ""
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return ""
	}
	// The timestamp comes first so a plain directory listing is naturally sorted
	// by run time. The job name is included for human scanning, but sanitized to
	// avoid characters that are invalid on Windows or awkward on shells.
	fileName := started.Format("20060102-150405") + "_" + sanitizeFileName(job.Name) + ".log"
	path := filepath.Join(logsDir, fileName)
	content := fmt.Sprintf("time: %s\njob_id: %d\njob_name: %s\ntrigger: %s\nstate: %s\ndetail: %s\ncommand: %s\narguments: %s\nsuccess_exit_codes: %s\nstart_only: %t\n\n%s\n",
		started.Format("2006-01-02 15:04:05"), job.ID, job.Name, trigger, state, detail, job.Command, logArguments(job.Arguments), successExitCodesText(job), job.StartOnly, output)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return ""
	}
	return path
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "job"
	}
	var builder strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
		case r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	result := strings.Trim(builder.String(), "_")
	if result == "" {
		return "job"
	}
	return result
}

func startJobOnly(invocation commandInvocation, job Job, started time.Time) (string, string, string) {
	command := invocation.command
	if invocation.hideWindow {
		configureHiddenWindow(command)
	}
	err := command.Start()
	duration := time.Since(started).Round(time.Millisecond)
	if err != nil {
		return "Failed", fmt.Sprintf("%T: %v", err, err), startOnlyOutput(job, 0)
	}
	pid := command.Process.Pid
	if releaseErr := command.Process.Release(); releaseErr != nil {
		return "Failed", fmt.Sprintf("process started with pid %d, but release failed: %T: %v", pid, releaseErr, releaseErr), startOnlyOutput(job, pid)
	}
	return "OK", fmt.Sprintf("Started in %s (pid %d); not waiting for process exit", duration, pid), startOnlyOutput(job, pid)
}

func startOnlyOutput(job Job, pid int) string {
	var builder strings.Builder
	builder.WriteString("status:\n")
	if pid > 0 {
		builder.WriteString(fmt.Sprintf("Started process pid %d. GoSentry is not waiting for it to exit.\n\n", pid))
	} else {
		builder.WriteString("Process did not start.\n\n")
	}
	builder.WriteString("command:\n")
	builder.WriteString(job.Command + "\n\n")
	builder.WriteString("arguments:\n")
	builder.WriteString(logArguments(job.Arguments))
	builder.WriteString("\n\nstart_only:\ntrue")
	return builder.String()
}

func runStateDetail(err error, runErr error, duration time.Duration, job Job) (string, string) {
	if err == nil {
		return "OK", fmt.Sprintf("Completed in %s (exit code 0)", duration)
	}
	if errors.Is(runErr, context.DeadlineExceeded) {
		return "Failed", fmt.Sprintf("Timed out after %s", commandTimeout)
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		return "OK", fmt.Sprintf("Completed; output capture stopped after %s because a child process kept the stream open", commandWaitDelay)
	}

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		exitCode := exitError.ExitCode()
		if acceptedExitCode(exitCode, job.SuccessExitCodes) {
			return "OK", fmt.Sprintf("Completed in %s with accepted exit code %d", duration, exitCode)
		}
		return "Failed", fmt.Sprintf("Exit code %d is not in success_exit_codes (%s)", exitCode, successExitCodesText(job))
	}
	return "Failed", fmt.Sprintf("%T: %v", err, err)
}

func acceptedExitCode(exitCode int, successExitCodes string) bool {
	for _, accepted := range parseExitCodes(successExitCodes) {
		if exitCode == accepted {
			return true
		}
	}
	return false
}

func parseExitCodes(value string) []int {
	value = strings.TrimSpace(value)
	if value == "" {
		return []int{0}
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	result := make([]int, 0, len(fields))
	seen := map[int]bool{}
	for _, field := range fields {
		code, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || seen[code] {
			continue
		}
		seen[code] = true
		result = append(result, code)
	}
	if len(result) == 0 {
		return []int{0}
	}
	return result
}

func successExitCodesText(job Job) string {
	codes := parseExitCodes(job.SuccessExitCodes)
	parts := make([]string, 0, len(codes))
	for _, code := range codes {
		parts = append(parts, strconv.Itoa(code))
	}
	return strings.Join(parts, ",")
}

type commandInvocation struct {
	command    *exec.Cmd
	hideWindow bool
}

func jobInvocation(ctx context.Context, job Job) commandInvocation {
	command := strings.TrimSpace(job.Command)
	arguments := commandArguments(job.Arguments)
	if len(arguments) > 0 || commandPathExists(command) {
		return commandInvocation{
			command:    exec.CommandContext(ctx, unquoteCommandPath(command), arguments...),
			hideWindow: false,
		}
	}

	// Shell mode remains for existing jobs and for commands that intentionally
	// use builtins, redirection, variables, or chained command syntax.
	return commandInvocation{
		command:    shellCommand(ctx, command),
		hideWindow: true,
	}
}

func commandArguments(arguments string) []string {
	var result []string
	for _, line := range strings.FieldsFunc(arguments, func(r rune) bool {
		return r == '\n' || r == '\r'
	}) {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func commandPathExists(command string) bool {
	command = unquoteCommandPath(strings.TrimSpace(command))
	if command == "" {
		return false
	}
	info, err := os.Stat(command)
	return err == nil && !info.IsDir()
}

func unquoteCommandPath(command string) string {
	return strings.Trim(strings.TrimSpace(command), `"`)
}

func logArguments(arguments string) string {
	if strings.TrimSpace(arguments) == "" {
		return "<empty>"
	}
	return strings.ReplaceAll(strings.TrimSpace(arguments), "\r\n", "\n")
}

func formatOutput(stdout string, stderr string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	if stdout == "" {
		// Showing an explicit placeholder is clearer than an empty panel in the
		// GUI: the user can tell that the command ran but produced no stream data.
		stdout = "<empty>"
	}
	if stderr == "" {
		stderr = "<empty>"
	}
	return "stdout:\n" + stdout + "\n\nstderr:\n" + stderr
}
