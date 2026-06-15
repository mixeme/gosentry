package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"
)

const commandTimeout = 30 * time.Second

func RunJob(ctx context.Context, job *Job, trigger string, logsDir string) RunRecord {
	started := time.Now()
	// Commands can hang forever if a script waits for input or a child process
	// stalls. A fixed timeout is a conservative first guardrail for a desktop
	// scheduler; later it can become a per-job setting without changing the
	// runner contract.
	runCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	// The command is executed through the platform shell so users can type the
	// same command they would test manually in cmd.exe or sh. This is less strict
	// than argv-based execution, but it is the expected behavior for a cron-like
	// tool that supports redirection, environment expansion, and shell builtins.
	command := shellCommand(runCtx, job.Command)
	configureHiddenWindow(command)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	duration := time.Since(started).Round(time.Millisecond)
	output := formatOutput(stdout.String(), stderr.String())

	state := "OK"
	detail := fmt.Sprintf("Completed in %s", duration)
	if err != nil {
		state = "Failed"
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			detail = fmt.Sprintf("Timed out after %s", commandTimeout)
		} else {
			detail = err.Error()
		}
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
		// Only PySentry run logs are managed here. Directories and non-.log files
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
	content := fmt.Sprintf("time: %s\njob_id: %d\njob_name: %s\ntrigger: %s\nstate: %s\ndetail: %s\ncommand: %s\n\n%s\n",
		started.Format("2006-01-02 15:04:05"), job.ID, job.Name, trigger, state, detail, job.Command, output)
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

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		// cmd.exe /C preserves Windows users' expectations for commands such as
		// "dir", "copy", variable expansion, and .bat/.cmd wrappers.
		return exec.CommandContext(ctx, "cmd.exe", "/C", command)
	}
	// sh -c is the portable baseline for Linux builds. It keeps the runner small
	// and avoids a hard dependency on a larger shell such as bash.
	return exec.CommandContext(ctx, "sh", "-c", command)
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
