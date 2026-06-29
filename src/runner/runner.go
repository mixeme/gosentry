package runner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"gitea.mixdep.ru/mix/gosentry/src/platform/winproc"
)

const commandTimeout = 30 * time.Second
const commandWaitDelay = 2 * time.Second

func RunJob(ctx context.Context, job *domain.Job, trigger string, logsDir string) (domain.RunRecord, error) {
	started := time.Now()
	// Commands can hang forever if a script waits for input or a child process
	// stalls. A fixed timeout is a conservative first guardrail for a desktop
	// scheduler; later it can become a per-job setting without changing the
	// runner contract.
	runCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	var output string
	var state string
	var detail string
	var durationMS int64
	if job.StartOnly {
		invocation := jobInvocation(ctx, *job)
		state, detail, output = startJobOnly(invocation, *job, started)
		// StartOnly jobs don't wait for process exit, so no meaningful duration.
		durationMS = 0
	} else {
		var stdoutBuf strings.Builder
		var stderrBuf strings.Builder
		invocation := jobInvocation(runCtx, *job)
		command := invocation.command
		command.WaitDelay = commandWaitDelay
		if invocation.hideWindow {
			winproc.ConfigureHiddenWindow(command)
		}
		command.Stdout = &stdoutBuf
		command.Stderr = &stderrBuf

		err := command.Run()
		duration := time.Since(started).Round(time.Millisecond)
		durationMS = duration.Milliseconds()
		output = formatOutput(stdoutBuf.String(), stderrBuf.String())
		state, detail = runStateDetail(err, runCtx.Err(), duration)
	}

	now := time.Now()
	timestamp := now.Format("2006-01-02 15:04:05")
	logFile, logErr := writeRunLog(logsDir, *job, trigger, state, detail, output, durationMS, now)

	// The runner is now pure with respect to the job: it returns a RunRecord and
	// lets the caller fold that record into the job's JobRuntime. Run state no
	// longer lives on Job, so there is nothing on the job to mutate here.
	return domain.RunRecord{
		Time:       timestamp,
		JobID:      job.ID,
		JobName:    job.Name,
		Trigger:    trigger,
		State:      state,
		Detail:     detail,
		LogFile:    logFile,
		Output:     output,
		DurationMS: durationMS,
	}, logErr
}

func startJobOnly(invocation commandInvocation, job domain.Job, started time.Time) (string, string, string) {
	command := invocation.command
	if invocation.hideWindow {
		winproc.ConfigureHiddenWindow(command)
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

func startOnlyOutput(job domain.Job, pid int) string {
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

func runStateDetail(err error, runErr error, duration time.Duration) (string, string) {
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
		return "Failed", fmt.Sprintf("Failed with exit code %d", exitError.ExitCode())
	}
	return "Failed", fmt.Sprintf("%T: %v", err, err)
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
