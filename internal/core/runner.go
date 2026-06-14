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
	runCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	command := shellCommand(runCtx, job.Command)
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
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".log") {
			continue
		}
		path := filepath.Join(logsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if maxAgeDays > 0 && info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
			continue
		}
		logs = append(logs, logFile{path: path, modTime: info.ModTime()})
	}

	if maxFiles <= 0 || len(logs) <= maxFiles {
		return nil
	}
	sort.Slice(logs, func(i int, j int) bool {
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
		return exec.CommandContext(ctx, "cmd.exe", "/C", command)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}

func formatOutput(stdout string, stderr string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	if stdout == "" {
		stdout = "<empty>"
	}
	if stderr == "" {
		stderr = "<empty>"
	}
	return "stdout:\n" + stdout + "\n\nstderr:\n" + stderr
}
