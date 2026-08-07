package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"gitea.mixdep.ru/mix/gosentry/src/runner"
	"gitea.mixdep.ru/mix/gosentry/src/storage"
)

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
