package storage

import (
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

// LoadJobsFile reads and normalizes the job definitions at path. The bool
// reports whether the file was there: a missing file is not an error but the
// answer to "is this file already a jobs file?", which is what the Settings tab
// needs when the user points the application at a different jobs file.
func LoadJobsFile(path string) ([]domain.Job, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var file domain.JobsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, false, err
	}
	normalizeJobs(file.Jobs)
	return file.Jobs, true, nil
}

func loadOrCreateJobs(path string) ([]domain.Job, error) {
	jobs, found, err := LoadJobsFile(path)
	if err != nil {
		return nil, err
	}
	if found {
		return jobs, nil
	}
	// Seed sample jobs so a new user can immediately see scheduled and manual
	// execution without inventing a command. The failure sample stays disabled
	// so it does not spam notifications; Run now still works for testing.
	jobs = defaultJobs()
	normalizeJobs(jobs)
	return jobs, writeJSON(path, domain.JobsFile{Jobs: jobs})
}

func normalizeJobs(jobs []domain.Job) {
	next := 1
	seen := make(map[int]bool, len(jobs))
	for index := range jobs {
		job := &jobs[index]
		if job.ID <= 0 || seen[job.ID] {
			// IDs are assigned only when absent or already claimed by an earlier job
			// in this file — a hand-edited jobs.json can carry two entries with the
			// same ID, which would otherwise share one runtime, one schedule-cache
			// entry, and one SeedStats bucket. Existing, unique IDs stay stable
			// because History and future log associations use them to identify jobs.
			job.ID = next
		}
		seen[job.ID] = true
		if job.ID >= next {
			next = job.ID + 1
		}
		if strings.TrimSpace(job.Name) == "" {
			job.Name = "Untitled job"
		}
		if strings.TrimSpace(job.Schedule) == "" {
			job.Schedule = "@every 1m"
		}
		if strings.TrimSpace(job.Command) == "" {
			// An empty command would fail in a confusing way. A safe echo command
			// gives the user something observable and harmless instead.
			job.Command = echoCommand("GoSentry job ran")
		}
		job.Arguments = strings.TrimSpace(job.Arguments)
		// Runtime state (last run, next run, status, output, activity) is no longer
		// part of Job. It is reconstructed each time the app starts via
		// domain.NewRuntime, so normalizeJobs only touches durable configuration.
	}
}

func defaultJobs() []domain.Job {
	return []domain.Job{
		{
			ID:       1,
			Name:     "Hello scheduler",
			Folder:   "Examples",
			Schedule: "@every 1m",
			Command:  echoCommand("GoSentry test job: scheduler is alive"),
			Enabled:  true,
		},
		{
			ID:       2,
			Name:     "Write timestamp",
			Folder:   "Examples",
			Schedule: "*/1 * * * *",
			Command:  echoCommand("GoSentry test job: timestamp command ran"),
			Enabled:  true,
		},
		{
			ID:       3,
			Name:     "Paused sample",
			Schedule: "@every 1m",
			Command:  echoCommand("This paused sample should not run until enabled"),
			Enabled:  false,
		},
		{
			ID:       4,
			Name:     "Failure notification test",
			Folder:   "Examples",
			Schedule: "@every 1m",
			Command:  failCommand(),
			Enabled:  false,
		},
	}
}

func failCommand() string {
	if runtime.GOOS == "windows" {
		return "exit /b 1"
	}
	return "exit 1"
}

func echoCommand(message string) string {
	if runtime.GOOS == "windows" {
		return "echo " + message
	}
	// POSIX shells need quotes for messages with spaces. Single quotes inside the
	// message are escaped using the standard close-quote/backslash/reopen pattern.
	return "echo '" + strings.ReplaceAll(message, "'", "'\\''") + "'"
}
