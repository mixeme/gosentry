package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"gitea.mixdep.ru/mix/gosentry/src/runner"
	"gitea.mixdep.ru/mix/gosentry/src/storage"
)

// maxJobLogs bounds the in-memory activity list kept per job. The full history
// lives in the log files on disk; this is only the recent activity shown in the
// GUI, so an old run aging out of the list is intentional.
const maxJobLogs = 50

// timestampLayout matches the format used for run records so UI-action activity
// and command runs line up in the History view.
const timestampLayout = "2006-01-02 15:04:05"

// errJobNotFound is returned by the mutating operations when no loaded job has
// the requested ID.
var errJobNotFound = errors.New("job not found")

// CreateJob normalizes and validates the supplied configuration, assigns the
// next free ID, and adds it to the loaded set. It returns the stored job (with
// its assigned ID) so the caller can select it. The job is persisted and a
// "Created" activity record is emitted.
func (s *Service) CreateJob(job domain.Job) (domain.Job, error) {
	normalizeJob(&job)
	if err := validateJob(job); err != nil {
		return domain.Job{}, err
	}

	s.mu.Lock()
	job.ID = s.nextIDLocked()
	s.jobs = append(s.jobs, job)
	runtime := domain.NewRuntime(job)
	s.runtimes[job.ID] = runtime
	s.parseScheduleLocked(&job)
	record := uiRecord(job.ID, job.Name, "Created", "Job was added")
	prependLog(runtime, record)
	save := s.deferSaveLocked(s.store.PrepareSaveJobs(s.jobs))
	s.mu.Unlock()

	if err := save(); err != nil {
		// The write is atomic, so a failure left the file holding the previous
		// list: take the job back out so memory matches what is on disk. Another
		// operation may have run in between, so it is removed by ID rather than by
		// truncating the slice.
		s.mu.Lock()
		if index := s.indexByIDLocked(job.ID); index >= 0 {
			s.jobs = append(s.jobs[:index], s.jobs[index+1:]...)
		}
		delete(s.runtimes, job.ID)
		delete(s.schedules, job.ID)
		s.mu.Unlock()
		return domain.Job{}, err
	}
	s.emit(RunRecorded{Record: record})
	s.emit(JobChanged{JobID: job.ID})
	return job, nil
}

// UpdateJob replaces the durable configuration of the job with the same ID,
// keeping its runtime state (keyed by ID) and recomputing its next run. The job
// is persisted and an "Updated" activity record is emitted.
func (s *Service) UpdateJob(job domain.Job) error {
	normalizeJob(&job)
	if err := validateJob(job); err != nil {
		return err
	}

	s.mu.Lock()
	existing := s.findByIDLocked(job.ID)
	if existing == nil {
		s.mu.Unlock()
		return fmt.Errorf("update job %d: %w", job.ID, errJobNotFound)
	}
	*existing = job
	runtime := s.runtimeForLocked(existing)
	// An edit may have toggled Enabled; reflect that into the status the same way
	// a dedicated enable/disable would, then recompute the next run.
	if job.Enabled {
		if runtime.LastState == "" || runtime.LastState == "Paused" {
			runtime.LastState = "Ready"
		}
	} else {
		runtime.LastState = "Paused"
	}
	s.parseScheduleLocked(existing)
	s.refreshNextRunLocked(existing, runtime)
	record := uiRecord(job.ID, job.Name, "Updated", "Job settings changed")
	prependLog(runtime, record)
	save := s.deferSaveLocked(s.store.PrepareSaveJobs(s.jobs))
	s.mu.Unlock()

	if err := save(); err != nil {
		return err
	}
	s.emit(RunRecorded{Record: record})
	s.emit(JobChanged{JobID: job.ID})
	return nil
}

// DeleteJob removes the job with the given ID along with its runtime and cached
// schedule. The remaining jobs are persisted and a "Deleted" activity record is
// emitted. The JobChanged event carries a zero ID to signal a broad change.
func (s *Service) DeleteJob(id int) error {
	s.mu.Lock()
	index := s.indexByIDLocked(id)
	if index < 0 {
		s.mu.Unlock()
		return fmt.Errorf("delete job %d: %w", id, errJobNotFound)
	}
	deleted := s.jobs[index]
	s.jobs = append(s.jobs[:index], s.jobs[index+1:]...)
	delete(s.runtimes, id)
	delete(s.schedules, id)
	record := uiRecord(id, deleted.Name, "Deleted", "Job was removed")
	save := s.deferSaveLocked(s.store.PrepareSaveJobs(s.jobs))
	s.mu.Unlock()

	if err := save(); err != nil {
		return err
	}
	s.emit(RunRecorded{Record: record})
	s.emit(JobChanged{JobID: 0})
	return nil
}

// SetEnabled enables or disables a single job. Enabling moves it back to "Ready"
// and recomputes its next run (respecting the global pause); disabling parks it
// at "Paused". The job is persisted and a "Resumed"/"Paused" activity record is
// emitted.
func (s *Service) SetEnabled(id int, enabled bool) error {
	s.mu.Lock()
	job := s.findByIDLocked(id)
	if job == nil {
		s.mu.Unlock()
		return fmt.Errorf("set enabled job %d: %w", id, errJobNotFound)
	}
	job.Enabled = enabled
	runtime := s.runtimeForLocked(job)
	s.parseScheduleLocked(job)

	var record domain.RunRecord
	if enabled {
		runtime.LastState = "Ready"
		s.refreshNextRunLocked(job, runtime)
		record = uiRecord(id, job.Name, "Resumed", "Job was enabled")
	} else {
		runtime.LastState = "Paused"
		runtime.NextRun = "Paused"
		runtime.NextDue = time.Time{}
		// A disabled job's own occurrences stop firing, so a "queue" backlog it was
		// carrying no longer corresponds to anything: clear it rather than replaying
		// stale deferred runs if the job is re-enabled later.
		runtime.PendingRuns = 0
		record = uiRecord(id, job.Name, "Paused", "Job was disabled")
	}
	prependLog(runtime, record)
	save := s.deferSaveLocked(s.store.PrepareSaveJobs(s.jobs))
	s.mu.Unlock()

	if err := save(); err != nil {
		return err
	}
	s.emit(RunRecorded{Record: record})
	s.emit(JobChanged{JobID: id})
	return nil
}

// SetGlobalPause flips the global pause that gates scheduled execution.
// Manual "Run now" remains available while paused. Each enabled job's next-run
// text reflects the new state immediately so the list view is understandable
// before the next tick. A "Paused"/"Resumed" scheduler activity record and a
// SchedulerStateChanged event are emitted.
func (s *Service) SetGlobalPause(paused bool) error {
	s.mu.Lock()
	s.paused = paused
	s.store.Config.Paused = paused
	now := time.Now()
	for index := range s.jobs {
		job := &s.jobs[index]
		runtime := s.runtimeForLocked(job)
		if paused {
			// A "queue" backlog counts occurrences missed *while paused is off*; once
			// paused, none of those correspond to anything the user would expect
			// replayed on resume, so drop it rather than letting a stale counter fire
			// a deferred run for an occurrence from before the pause.
			runtime.PendingRuns = 0
		}
		s.refreshNextRunFromLocked(job, runtime, now)
	}
	save := s.deferSaveLocked(s.store.PrepareSaveConfig())
	s.mu.Unlock()

	if err := save(); err != nil {
		return err
	}
	state, detail := "Resumed", "All job execution resumed"
	if paused {
		state, detail = "Paused", "All job execution paused"
	}
	s.emit(RunRecorded{Record: uiRecord(0, "Scheduler", state, detail)})
	s.emit(SchedulerStateChanged{Paused: paused})
	return nil
}

// SetJobListView persists the Jobs list density preference. Unlike
// SetGlobalPause this touches nothing but the config: no job changed, so there
// is no SaveJobs, and no event is emitted — the choice is presentational and the
// Jobs view refreshes its own list, whereas an event would trigger a pointless
// whole-window refresh. Anything that is not "compact" is stored as detailed so
// the file never gains an unrecognised value.
func (s *Service) SetJobListView(view domain.JobListView) error {
	if !view.IsCompact() {
		view = domain.JobListViewDetailed
	}
	s.mu.Lock()
	if s.store.Config.JobListView == view {
		s.mu.Unlock()
		return nil
	}
	s.store.Config.JobListView = view
	save := s.deferSaveLocked(s.store.PrepareSaveConfig())
	s.mu.Unlock()
	return save()
}

// ShouldNotifyOnFailure reports whether the user has enabled desktop
// notifications for failed job runs. It reads the config under mu so it is
// safe to call from any goroutine.
func (s *Service) ShouldNotifyOnFailure() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.Config.NotifyOnFailure
}

// UpdateSettings validates and persists a new application configuration. The
// loaded jobs are re-saved because the jobs file may have changed, and log
// cleanup runs so a tightened retention policy takes effect immediately.
//
// Pointing the config at a different jobs file that already exists adopts that
// file: its jobs replace the loaded ones, which is the only way the user can
// switch between job lists. A path with no file there yet receives the current
// jobs instead, which is how the jobs file is renamed or relocated. Adoption
// discards all runtime state, so it is refused while a job is running.
func (s *Service) UpdateSettings(config domain.Config) error {
	if err := validateConfig(config); err != nil {
		return err
	}
	// The path is stored exactly as it is resolved, so a hand-typed value with
	// stray spaces cannot make the saved setting and the file in use disagree.
	config.JobsFile = strings.TrimSpace(config.JobsFile)

	s.mu.Lock()
	// AppDir is fixed for the process and only UpdateSettings itself — a UI
	// action — can move JobsPath, so this snapshot stays valid across the reads
	// below.
	appDir := s.store.Paths.AppDir
	jobsPath := storage.ResolveConfiguredPath(appDir, config.JobsFile)
	switching := jobsPath != s.store.Paths.JobsPath
	running := s.anyRunningLocked()
	s.mu.Unlock()

	if switching && running {
		return errors.New("cannot change the jobs file while a job is running")
	}
	// Read the new file, and reconstruct its jobs' statistics from the logs the
	// new config points at, before anything is written and while no lock is held:
	// both are file I/O, and SeedStats opens every log in the directory. A file
	// that cannot be parsed leaves both the config and the current jobs untouched.
	var adopted []domain.Job
	var seeds map[int]runner.SeededStats
	if switching {
		jobs, found, err := storage.LoadJobsFile(jobsPath)
		if err != nil {
			return fmt.Errorf("read jobs file %s: %w", jobsPath, err)
		}
		if found {
			adopted = jobs
			seeds = runner.SeedStats(storage.ResolveConfiguredPath(appDir, config.LogsDir), jobs, config.MaxLogFiles)
		}
	}

	s.mu.Lock()
	// The guard above was evaluated before the reads, off the lock, so re-check
	// it: a scheduled run may have started in the meantime, and adoption drops
	// every runtime.
	if switching && s.anyRunningLocked() {
		s.mu.Unlock()
		return errors.New("cannot change the jobs file while a job is running")
	}
	s.store.Config = config
	saveConfig := s.store.PrepareSaveConfig()
	if adopted != nil {
		s.adoptJobsLocked(adopted)
		s.applySeededStatsLocked(seeds)
	}
	// PrepareSaveConfig re-resolved the paths from the new config, so the jobs
	// write targets the (possibly new) jobs file and cleanup targets the new logs
	// dir. Adopted jobs are written back too, which persists the IDs and defaults
	// that normalization filled in, exactly as loading them at startup would. The
	// jobs write is skipped when the config write fails, because both writes run
	// in the order prepared and stop at the first error.
	save := s.deferSaveLocked(saveConfig, s.store.PrepareSaveJobs(s.jobs))
	loaded := len(s.jobs)
	logsDir := s.store.Paths.LogsDir
	maxFiles := s.store.Config.MaxLogFiles
	maxAge := s.store.Config.MaxLogAgeDays
	s.mu.Unlock()

	saveErr := save()
	if adopted != nil {
		// A broad JobChanged redraws the job list; JobsLoaded tells the user in
		// History which file those jobs came from, since nothing was asked. Both
		// are emitted even when the write failed: the adopted jobs are already the
		// in-memory list, and a job list the user cannot see would be worse than
		// the error they are about to be shown.
		s.emit(JobsLoaded{Path: jobsPath, Count: loaded})
		s.emit(JobChanged{})
	}
	if saveErr != nil {
		return saveErr
	}
	return runner.CleanupLogs(logsDir, maxFiles, maxAge)
}

// refreshNextRunLocked recomputes a job's next-run display from the current time,
// honoring enabled/paused state. The caller must hold mu.
func (s *Service) refreshNextRunLocked(job *domain.Job, runtime *domain.JobRuntime) {
	s.refreshNextRunFromLocked(job, runtime, time.Now())
}

// refreshNextRunFromLocked is refreshNextRunLocked with an explicit reference
// time, used when one timestamp should drive a whole batch (e.g. a global
// pause). The caller must hold mu.
func (s *Service) refreshNextRunFromLocked(job *domain.Job, runtime *domain.JobRuntime, from time.Time) {
	if !job.Enabled {
		runtime.NextRun = "Paused"
		runtime.NextDue = time.Time{}
		return
	}
	if s.paused {
		runtime.NextRun = "Scheduler paused"
		runtime.NextDue = time.Time{}
		return
	}
	s.prepareNextRunLocked(job, runtime, from)
}

// prepareNextRunLocked computes the concrete next-due time from the cached
// schedule. A missing cache entry means the schedule string was unparseable.
// The caller must hold mu.
func (s *Service) prepareNextRunLocked(job *domain.Job, runtime *domain.JobRuntime, from time.Time) {
	sched, ok := s.schedules[job.ID]
	if !ok {
		runtime.NextRun = "Invalid schedule"
		runtime.NextDue = time.Time{}
		return
	}
	runtime.NextDue = sched.Next(from)
	runtime.NextRun = runtime.NextDue.Format(timestampLayout)
}

// parseScheduleLocked caches a parsed schedule for the job, dropping the cache
// entry when the schedule string is invalid so prepareNextRunLocked can tell the
// two apart. The caller must hold mu.
func (s *Service) parseScheduleLocked(job *domain.Job) {
	sched, err := domain.Parse(job.Schedule)
	if err != nil {
		delete(s.schedules, job.ID)
		return
	}
	s.schedules[job.ID] = sched
}

// findByIDLocked returns a pointer into the jobs slice for the job with the
// given ID, or nil. The caller must hold mu.
func (s *Service) findByIDLocked(id int) *domain.Job {
	index := s.indexByIDLocked(id)
	if index < 0 {
		return nil
	}
	return &s.jobs[index]
}

// indexByIDLocked returns the slice index of the job with the given ID, or -1.
// The caller must hold mu.
func (s *Service) indexByIDLocked(id int) int {
	for index := range s.jobs {
		if s.jobs[index].ID == id {
			return index
		}
	}
	return -1
}

// runtimeForLocked returns the runtime for a job, lazily creating it if missing
// so the Service stays robust if a job lacks an entry. The caller must hold mu.
func (s *Service) runtimeForLocked(job *domain.Job) *domain.JobRuntime {
	runtime, ok := s.runtimes[job.ID]
	if !ok || runtime == nil {
		runtime = domain.NewRuntime(*job)
		s.runtimes[job.ID] = runtime
	}
	return runtime
}

// nextIDLocked returns the smallest ID greater than every loaded job's ID. The
// caller must hold mu.
func (s *Service) nextIDLocked() int {
	next := 1
	for index := range s.jobs {
		if s.jobs[index].ID >= next {
			next = s.jobs[index].ID + 1
		}
	}
	return next
}

// prependLog adds a record to the front of a runtime's activity list and caps
// its length so it cannot grow without bound.
func prependLog(runtime *domain.JobRuntime, record domain.RunRecord) {
	runtime.Logs = append([]domain.RunRecord{record}, runtime.Logs...)
	if len(runtime.Logs) > maxJobLogs {
		runtime.Logs = runtime.Logs[:maxJobLogs]
	}
}

// uiRecord builds an activity record for a user/Service action, using the same
// timestamp shape and "UI" trigger as the GUI did so History stays consistent.
func uiRecord(jobID int, jobName string, state string, detail string) domain.RunRecord {
	return domain.RunRecord{
		Time:    time.Now().Format(timestampLayout),
		JobID:   jobID,
		JobName: jobName,
		Trigger: "UI",
		State:   state,
		Detail:  detail,
	}
}

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
