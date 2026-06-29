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

// RunNow starts a manual run of a job. Global pause stops only the scheduler's
// automatic runs (see RunDue), so a manual "Run now" is allowed even while
// paused — it is the user's explicit, one-off action. It will not start a job
// that is already running. In sequential execution mode it also refuses while
// any other job is running, so a manual run never breaks the one-at-a-time
// guarantee. The run itself happens on a background goroutine that records the
// result through the Service, so RunNow returns as soon as the run is started.
// The error reports why a run could not be started (or a failure to persist the
// "Running" status), not the run's own outcome.
func (s *Service) RunNow(id int) error {
	s.mu.Lock()
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
	if s.store.Config.ExecutionMode == domain.ExecutionModeSequential && s.anyRunningLocked() {
		s.mu.Unlock()
		return errors.New("another job is already running (sequential mode)")
	}
	err := s.startRunLocked(job, runtime, "Manual", time.Now())
	s.mu.Unlock()

	if err == nil {
		// Reflect the "Running" transition; the run's completion emits again later.
		s.emit(JobChanged{JobID: id})
	}
	return err
}

// RunDue is the scheduler's per-tick entry point: it starts whatever is due at
// the given time. It is a no-op while globally paused. Run results are recorded
// back through the Service, so the Service stays the sole writer of job and
// runtime state. The time is supplied by the scheduler's clock, which lets tests
// drive due-evaluation deterministically.
//
// Dispatch obeys two configured knobs. The execution mode decides whether
// distinct due jobs run together (parallel) or one at a time (sequential): in
// sequential mode a due job is left for a later tick while any other job is
// running. The overlap policy decides what happens when a job comes due again
// while its own previous run is still in flight: "skip" drops the new run,
// "queue" increments PendingRuns so executeRun drains missed occurrences after
// the current run finishes. Either way NextDue is advanced past the fired occurrence so the same
// moment is not re-evaluated on every tick.
func (s *Service) RunDue(now time.Time) {
	s.mu.Lock()
	var started []int
	var startErr error
	if !s.paused {
		sequential := s.store.Config.ExecutionMode == domain.ExecutionModeSequential
		running := s.anyRunningLocked()
		for index := range s.jobs {
			job := &s.jobs[index]
			runtime := s.runtimeForLocked(job)
			if !job.Enabled || runtime.NextDue.IsZero() || now.Before(runtime.NextDue) {
				continue
			}
			if runtime.LastState == "Running" {
				// The job came due again while its own run is still in flight.
				// Apply the effective overlap policy and step past this
				// occurrence.
				if s.effectiveOverlapPolicy(job) == domain.OverlapPolicyQueue {
					runtime.PendingRuns++
				}
				s.advanceNextDueLocked(job, runtime, now)
				continue
			}
			if sequential && running {
				// One-at-a-time: leave this job due and pick it up on a later
				// tick once the in-flight run has finished.
				continue
			}
			if err := s.startRunLocked(job, runtime, "Schedule", now); err != nil {
				startErr = err
				continue
			}
			started = append(started, job.ID)
			running = true
		}
	}
	s.mu.Unlock()

	if startErr != nil {
		s.emit(ErrorOccurred{Err: fmt.Errorf("save jobs before scheduled run: %w", startErr)})
	}
	for _, id := range started {
		s.emit(JobChanged{JobID: id})
	}
}

// runEnv snapshots path and retention settings for one background run so
// executeRun does not read store.Paths or store.Config without holding mu.
type runEnv struct {
	logsDir  string
	maxFiles int
	maxAge   int
}

// startRunLocked transitions a job to "Running", advances its NextDue to the next
// scheduled occurrence, persists that, and launches the run on a background
// goroutine. Advancing (rather than zeroing) NextDue keeps the schedule marching
// while the run is in flight, which is what lets RunDue notice a fresh occurrence
// firing during a long run and apply the overlap policy. The caller must hold mu.
// now is the reference time for next-due advancement and the running placeholder.
func (s *Service) startRunLocked(job *domain.Job, runtime *domain.JobRuntime, trigger string, now time.Time) error {
	jobCopy := *job
	prevState := runtime.LastState
	prevNextRun := runtime.NextRun
	prevOutput := runtime.Output
	prevNextDue := runtime.NextDue

	runtime.LastState = "Running"
	runtime.NextRun = "Running"
	runtime.Output = runningOutput(jobCopy, trigger, now)
	s.advanceNextDueLocked(job, runtime, now)
	if err := s.store.SaveJobs(s.jobs); err != nil {
		runtime.LastState = prevState
		runtime.NextRun = prevNextRun
		runtime.Output = prevOutput
		runtime.NextDue = prevNextDue
		return err
	}
	env := runEnv{
		logsDir:  s.store.Paths.LogsDir,
		maxFiles: s.store.Config.MaxLogFiles,
		maxAge:   s.store.Config.MaxLogAgeDays,
	}
	// Capture ctx under the lock so a concurrent Start/Stop cannot swap it out
	// from under the goroutine after we release mu.
	go s.executeRun(s.ctx, jobCopy, trigger, env)
	return nil
}

// executeRun runs the job off the lock, then records the result back through the
// Service under the lock and announces it. If the job was marked Pending while
// running (the "queue" overlap policy), and it is still enabled and the scheduler
// is not paused, deferred runs are started one at a time until PendingRuns reaches
// zero. Each deferred run runs on its own goroutine.
func (s *Service) executeRun(ctx context.Context, jobCopy domain.Job, trigger string, env runEnv) {
	record, logErr := s.runJob(ctx, &jobCopy, trigger, env.logsDir)

	s.mu.Lock()
	var cleanupErr, saveErr error
	var rerunStarted bool
	if current := s.findByIDLocked(jobCopy.ID); current != nil {
		runtime := s.runtimeForLocked(current)
		runtime.LastRun = record.Time
		runtime.LastState = record.State
		runtime.Output = record.Output
		prependLog(runtime, record)
		updateStats(runtime, record)
		rerun := runtime.PendingRuns > 0 && current.Enabled && !s.paused
		if rerun {
			runtime.PendingRuns--
			// A scheduled occurrence fired while this run was active under the
			// "queue" policy; start one deferred run now.
			saveErr = s.startRunLocked(current, runtime, "Schedule", time.Now())
			rerunStarted = saveErr == nil
		} else {
			s.refreshNextRunLocked(current, runtime)
			saveErr = s.store.SaveJobs(s.jobs)
		}
		cleanupErr = runner.CleanupLogs(env.logsDir, env.maxFiles, env.maxAge)
	}
	s.mu.Unlock()

	if logErr != nil {
		s.emit(ErrorOccurred{Err: fmt.Errorf("write run log for %q: %w", jobCopy.Name, logErr)})
	}
	if cleanupErr != nil {
		s.emit(ErrorOccurred{Err: fmt.Errorf("log cleanup after run %q: %w", jobCopy.Name, cleanupErr)})
	}
	if saveErr != nil {
		s.emit(ErrorOccurred{Err: fmt.Errorf("save jobs after run %q: %w", jobCopy.Name, saveErr)})
	}
	s.emit(RunRecorded{Record: record})
	if !rerunStarted {
		s.emit(JobChanged{JobID: jobCopy.ID})
	}
}

// effectiveOverlapPolicy resolves the overlap policy that actually governs a
// job: the job's own value when set, otherwise the global Config default. An
// empty Job.OverlapPolicy means "inherit the global default", which is why
// normalizeJobs leaves it empty rather than backfilling the configured value.
func (s *Service) effectiveOverlapPolicy(job *domain.Job) domain.OverlapPolicy {
	if policy := domain.OverlapPolicy(strings.TrimSpace(job.OverlapPolicy)); policy != "" {
		return policy
	}
	return s.store.Config.OverlapPolicy
}

// anyRunningLocked reports whether any loaded job is currently in the "Running"
// state. It backs the sequential-mode guards in RunNow and RunDue. The caller
// must hold mu.
func (s *Service) anyRunningLocked() bool {
	for index := range s.jobs {
		runtime, ok := s.runtimes[s.jobs[index].ID]
		if ok && runtime != nil && runtime.LastState == "Running" {
			return true
		}
	}
	return false
}

// advanceNextDueLocked moves a job's NextDue to the next scheduled time after
// from, leaving the NextRun display string untouched so callers can keep it
// showing "Running" during a run. A missing schedule cache (an unparseable
// schedule) zeroes NextDue. The caller must hold mu.
func (s *Service) advanceNextDueLocked(job *domain.Job, runtime *domain.JobRuntime, from time.Time) {
	sched, ok := s.schedules[job.ID]
	if !ok {
		runtime.NextDue = time.Time{}
		return
	}
	runtime.NextDue = sched.Next(from)
}

// updateStats folds one completed RunRecord into the runtime's aggregate
// execution-time statistics. Called under mu inside executeRun.
func updateStats(rt *domain.JobRuntime, r domain.RunRecord) {
	rt.RunCount++
	if r.State == "Failed" {
		rt.FailCount++
	}
	if r.DurationMS <= 0 {
		return
	}
	rt.LastDurationMS = r.DurationMS
	if r.DurationMS > rt.MaxDurationMS {
		rt.MaxDurationMS = r.DurationMS
	}
	rt.TimedRunCount++
	rt.AvgDurationMS = (rt.AvgDurationMS*int64(rt.TimedRunCount-1) + r.DurationMS) / int64(rt.TimedRunCount)
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
