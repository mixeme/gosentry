package app

import (
	"time"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

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
