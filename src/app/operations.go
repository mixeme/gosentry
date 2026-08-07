package app

import (
	"errors"
	"fmt"
	"time"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
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
