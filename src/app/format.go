package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

// StatusText formats a job's current state for display: "Paused" if disabled,
// else its runtime LastState (Ready, Running, Success, etc).
func StatusText(j domain.Job, runtime *domain.JobRuntime) string {
	if !j.Enabled {
		return "Paused"
	}
	if runtime == nil {
		return ""
	}
	return runtime.LastState
}

// EventText formats a run record for the History table, showing time, trigger,
// job name, outcome state, detail, and log file (if any).
func EventText(e domain.RunRecord) string {
	trigger := e.Trigger
	if trigger == "" {
		trigger = "Unknown"
	}
	if e.LogFile != "" {
		return fmt.Sprintf("%s  %s  %s  %s  %s  %s", e.Time, trigger, e.JobName, e.State, e.Detail, e.LogFile)
	}
	return fmt.Sprintf("%s  %s  %s  %s  %s", e.Time, trigger, e.JobName, e.State, e.Detail)
}

// EventLine formats a run record as a compact single line for the jobs log
// view, using only the base name of the log file instead of the full path.
func EventLine(e domain.RunRecord) string {
	trigger := e.Trigger
	if trigger == "" {
		trigger = "Unknown"
	}
	if e.LogFile != "" {
		return fmt.Sprintf("%s  %s  %s  %s  %s  %s", e.Time, trigger, e.JobName, e.State, e.Detail, filepath.Base(e.LogFile))
	}
	return fmt.Sprintf("%s  %s  %s  %s  %s", e.Time, trigger, e.JobName, e.State, e.Detail)
}

// DisplayFolder formats a job's folder for display: "(No folder)" if empty,
// else the trimmed folder name.
func DisplayFolder(folder string) string {
	if strings.TrimSpace(folder) == "" {
		return "(No folder)"
	}
	return strings.TrimSpace(folder)
}

// DisplayArguments formats a job's arguments for display: "(none)" if empty,
// else the trimmed arguments.
func DisplayArguments(arguments string) string {
	if strings.TrimSpace(arguments) == "" {
		return "(none)"
	}
	return strings.TrimSpace(arguments)
}

// DisplayRunMode formats a job's execution mode: "Start only" or
// "Wait for completion".
func DisplayRunMode(job domain.Job) string {
	if job.StartOnly {
		return "Start only"
	}
	return "Wait for completion"
}

// DisplayInvocation formats a job's command and arguments for the jobs list,
// joining them with spacing and collapsing newlines in arguments to spaces.
func DisplayInvocation(job domain.Job) string {
	if strings.TrimSpace(job.Arguments) == "" {
		return job.Command
	}
	return job.Command + "    " + strings.ReplaceAll(strings.TrimSpace(job.Arguments), "\n", " ")
}

// DisplayStats returns a one-line execution-time summary for a job runtime.
// Returns "No runs recorded" when no runs have been counted yet.
func DisplayStats(rt *domain.JobRuntime) string {
	if rt == nil || rt.RunCount == 0 {
		return "No runs recorded"
	}
	return fmt.Sprintf("%d runs, %d failed, last %d ms, avg %d ms, max %d ms",
		rt.RunCount, rt.FailCount, rt.LastDurationMS, rt.AvgDurationMS, rt.MaxDurationMS)
}

// DisplayIndex returns the position of jobIndex in the given slice of indexes,
// or 0 if not found.
func DisplayIndex(indexes []int, jobIndex int) int {
	for display, index := range indexes {
		if index == jobIndex {
			return display
		}
	}
	return 0
}
