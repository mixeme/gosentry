package app

import (
	"errors"
	"path/filepath"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

// normalizeJob trims user-entered fields and applies the same defaults the job
// dialog used, so callers do not have to.
func normalizeJob(job *domain.Job) {
	job.Name = strings.TrimSpace(job.Name)
	job.Folder = strings.TrimSpace(job.Folder)
	job.Schedule = strings.TrimSpace(job.Schedule)
	job.Command = strings.TrimSpace(job.Command)
	job.Arguments = strings.TrimSpace(job.Arguments)
}

// validateJob enforces the minimum executable definition: name, schedule, and
// command must be present. Folder is optional. The schedule string itself is not
// rejected for being unparseable — that surfaces later as an "Invalid schedule"
// next-run, matching the prior behavior.
func validateJob(job domain.Job) error {
	if job.Name == "" || job.Schedule == "" || job.Command == "" {
		return errors.New("name, schedule, and command are required")
	}
	policy := strings.TrimSpace(job.OverlapPolicy)
	if policy != "" && policy != string(domain.OverlapPolicySkip) && policy != string(domain.OverlapPolicyQueue) {
		return errors.New("overlap policy must be 'skip', 'queue', or empty")
	}
	if job.TimeoutSeconds != nil && *job.TimeoutSeconds < 0 {
		return errors.New("timeout must be zero (no timeout) or a positive number of seconds, or unset to inherit the global default")
	}
	return nil
}

// hasFileName reports whether a path ends in something that can be a file name.
// It is a syntax check only — an existing directory whose name looks like a file
// name still passes, and fails at write time — but it catches the shapes a user
// types when they mean a folder: a trailing separator, "." and "..".
func hasFileName(path string) bool {
	if strings.HasSuffix(path, "/") || strings.HasSuffix(path, string(filepath.Separator)) {
		return false
	}
	switch filepath.Base(path) {
	case ".", "..", string(filepath.Separator):
		return false
	}
	return true
}

// validateConfig rejects settings that would break persistence or cleanup.
func validateConfig(config domain.Config) error {
	jobsFile := strings.TrimSpace(config.JobsFile)
	if jobsFile == "" {
		return errors.New("jobs file is required")
	}
	// A path that names only a folder would be written to as if it were a file
	// and fail later with an opaque OS error, so require a file name here.
	if !hasFileName(jobsFile) {
		return errors.New("jobs file must include a file name")
	}
	if strings.TrimSpace(config.LogsDir) == "" {
		return errors.New("logs directory is required")
	}
	// 0 means "keep everything" (see runner.CleanupLogs); only a negative count
	// is rejected, the same three-state shape as DefaultTimeoutSeconds below.
	if config.MaxLogFiles < 0 {
		return errors.New("max log files must be zero (unlimited) or a positive number")
	}
	if config.MaxLogAgeDays < 0 {
		return errors.New("max log age days must be zero (unlimited) or a positive number")
	}
	if config.ExecutionMode != domain.ExecutionModeParallel && config.ExecutionMode != domain.ExecutionModeSequential {
		return errors.New("execution mode must be 'parallel' or 'sequential'")
	}
	if config.OverlapPolicy != domain.OverlapPolicySkip && config.OverlapPolicy != domain.OverlapPolicyQueue {
		return errors.New("overlap policy must be 'skip' or 'queue'")
	}
	if config.DefaultTimeoutSeconds < 0 {
		return errors.New("default timeout must not be negative (0 means no timeout)")
	}
	// Empty Theme is accepted and normalized to the branded theme on load, so
	// older configs (and hand-built ones) stay valid without an explicit theme.
	if config.Theme != "" && config.Theme != domain.ThemeSystem && config.Theme != domain.ThemeGoSentry {
		return errors.New("theme must be 'system' or 'gosentry'")
	}
	return nil
}
