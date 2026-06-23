package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"gitea.mixdep.ru/mix/gosentry/src/runner"
)

// RunNow starts a manual run of a job. It refuses to run while globally paused —
// the pause is an emergency stop for all execution — and will not start a job
// that is already running. The run itself happens on a background goroutine that
// records the result through the Service, so RunNow returns as soon as the run
// is started. The error reports why a run could not be started (or a failure to
// persist the "Running" status), not the run's own outcome.
func (s *Service) RunNow(id int) error {
	s.mu.Lock()
	if s.paused {
		s.mu.Unlock()
		return errors.New("scheduler is paused")
	}
	job := s.findByIDLocked(id)
	if job == nil {
		s.mu.Unlock()
		return fmt.Errorf("run job %d: %w", id, errJobNotFound)
	}
	runtime := s.runtimeForLocked(job)
	if runtime.LastState == "Running" {
		s.mu.Unlock()
		return fmt.Errorf("job %d is already running", id)
	}
	err := s.startRunLocked(job, runtime, "Manual")
	s.mu.Unlock()

	// Reflect the "Running" transition; the run's completion emits again later.
	s.emit(JobChanged{JobID: id})
	return err
}

// RunDue is the scheduler's per-tick entry point: it starts whatever is due at
// the given time. It is a no-op while globally paused. At most one job is started
// per call so scheduled shell commands in this single process do not overlap; a
// job already running is skipped. Run results are recorded back through the
// Service, so the Service stays the sole writer of job and runtime state. The
// time is supplied by the scheduler's clock, which lets tests drive
// due-evaluation deterministically.
func (s *Service) RunDue(now time.Time) {
	s.mu.Lock()
	var startedID int
	var startErr error
	if !s.paused {
		for index := range s.jobs {
			job := &s.jobs[index]
			runtime := s.runtimeForLocked(job)
			if !job.Enabled || runtime.NextDue.IsZero() || now.Before(runtime.NextDue) {
				continue
			}
			if runtime.LastState == "Running" {
				continue
			}
			startErr = s.startRunLocked(job, runtime, "Schedule")
			startedID = job.ID
			break
		}
	}
	s.mu.Unlock()

	if startErr != nil {
		s.emit(ErrorOccurred{Err: fmt.Errorf("save jobs before scheduled run: %w", startErr)})
	}
	if startedID != 0 {
		s.emit(JobChanged{JobID: startedID})
	}
}

// startRunLocked transitions a job to "Running", persists that, and launches the
// run on a background goroutine. The caller must hold mu.
func (s *Service) startRunLocked(job *domain.Job, runtime *domain.JobRuntime, trigger string) error {
	jobCopy := *job
	runtime.LastState = "Running"
	runtime.NextRun = "Running"
	runtime.Output = runningOutput(jobCopy, trigger, time.Now())
	runtime.NextDue = time.Time{}
	err := s.store.SaveJobs(s.jobs)
	// Capture ctx under the lock so a concurrent Start/Stop cannot swap it out
	// from under the goroutine after we release mu.
	go s.executeRun(s.ctx, jobCopy, trigger)
	return err
}

// executeRun runs the job off the lock, then records the result back through the
// Service under the lock and announces it. It runs on its own goroutine.
func (s *Service) executeRun(ctx context.Context, jobCopy domain.Job, trigger string) {
	record := s.runJob(ctx, &jobCopy, trigger, s.store.Paths.LogsDir)

	s.mu.Lock()
	var cleanupErr, saveErr error
	if current := s.findByIDLocked(jobCopy.ID); current != nil {
		runtime := s.runtimeForLocked(current)
		runtime.LastRun = record.Time
		runtime.LastState = record.State
		runtime.Output = record.Output
		prependLog(runtime, record)
		s.refreshNextRunLocked(current, runtime)
		cleanupErr = runner.CleanupLogs(s.store.Paths.LogsDir, s.store.Config.MaxLogFiles, s.store.Config.MaxLogAgeDays)
		saveErr = s.store.SaveJobs(s.jobs)
	}
	s.mu.Unlock()

	if cleanupErr != nil {
		s.emit(ErrorOccurred{Err: fmt.Errorf("log cleanup after run %q: %w", jobCopy.Name, cleanupErr)})
	}
	if saveErr != nil {
		s.emit(ErrorOccurred{Err: fmt.Errorf("save jobs after run %q: %w", jobCopy.Name, saveErr)})
	}
	s.emit(RunRecorded{Record: record})
	s.emit(JobChanged{JobID: jobCopy.ID})
}

// runningOutput is the placeholder output shown while a job is running, before
// the real command output replaces it.
func runningOutput(job domain.Job, trigger string, started time.Time) string {
	var builder strings.Builder
	builder.WriteString("status:\n")
	builder.WriteString("Running since " + started.Format(timestampLayout) + "\n\n")
	builder.WriteString("trigger:\n")
	builder.WriteString(trigger + "\n\n")
	builder.WriteString("command:\n")
	builder.WriteString(job.Command + "\n\n")
	builder.WriteString("arguments:\n")
	builder.WriteString(runner.LogArguments(job.Arguments))
	builder.WriteString("\n\nstart_only:\n")
	builder.WriteString(fmt.Sprintf("%t", job.StartOnly))
	return builder.String()
}
