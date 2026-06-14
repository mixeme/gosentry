package core

import (
	"context"
	"strings"
	"sync"
	"time"
)

type Scheduler struct {
	store    *Store
	jobs     *[]Job
	onChange func(RunRecord)

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	paused bool
}

func NewScheduler(store *Store, jobs *[]Job, onChange func(RunRecord)) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{
		store:    store,
		jobs:     jobs,
		onChange: onChange,
		ctx:      ctx,
		cancel:   cancel,
	}
	s.resetNextRuns(time.Now())
	return s
}

func (s *Scheduler) Start() {
	ticker := time.NewTicker(time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case now := <-ticker.C:
				s.tick(now)
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	s.cancel()
}

func (s *Scheduler) SetPaused(paused bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.paused = paused
	now := time.Now()
	for index := range *s.jobs {
		job := &(*s.jobs)[index]
		if !job.Enabled {
			job.NextRun = "Paused"
			continue
		}
		if paused {
			job.NextRun = "Scheduler paused"
			continue
		}
		s.prepareNextRun(job, now)
	}
	_ = s.store.SaveJobs(*s.jobs)
}

func (s *Scheduler) RunNow(index int) RunRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(*s.jobs) {
		return RunRecord{}
	}
	job := &(*s.jobs)[index]
	record := RunJob(s.ctx, job, "Manual", s.store.Paths.LogsDir)
	s.prepareNextRun(job, time.Now())
	_ = CleanupLogs(s.store.Paths.LogsDir, s.store.Config.MaxLogFiles, s.store.Config.MaxLogAgeDays)
	_ = s.store.SaveJobs(*s.jobs)
	return record
}

func (s *Scheduler) RefreshSchedule(index int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(*s.jobs) {
		return
	}
	job := &(*s.jobs)[index]
	if !job.Enabled {
		job.NextRun = "Paused"
		return
	}
	if s.paused {
		job.NextRun = "Scheduler paused"
		return
	}
	s.prepareNextRun(job, time.Now())
}

func (s *Scheduler) tick(now time.Time) {
	var record RunRecord
	var changed bool

	s.mu.Lock()
	if !s.paused {
		for index := range *s.jobs {
			job := &(*s.jobs)[index]
			if !job.Enabled || job.nextDue.IsZero() || now.Before(job.nextDue) {
				continue
			}
			record = RunJob(s.ctx, job, "Schedule", s.store.Paths.LogsDir)
			s.prepareNextRun(job, time.Now())
			_ = CleanupLogs(s.store.Paths.LogsDir, s.store.Config.MaxLogFiles, s.store.Config.MaxLogAgeDays)
			changed = true
			break
		}
	}
	if changed {
		_ = s.store.SaveJobs(*s.jobs)
	}
	s.mu.Unlock()

	if changed && s.onChange != nil {
		s.onChange(record)
	}
}

func (s *Scheduler) resetNextRuns(now time.Time) {
	for index := range *s.jobs {
		job := &(*s.jobs)[index]
		if !job.Enabled {
			job.NextRun = "Paused"
			continue
		}
		s.prepareNextRun(job, now)
	}
	_ = s.store.SaveJobs(*s.jobs)
}

func (s *Scheduler) prepareNextRun(job *Job, from time.Time) {
	interval, ok := parseEvery(job.Schedule)
	if !ok {
		job.NextRun = "Unsupported schedule"
		job.nextDue = time.Time{}
		return
	}
	job.nextDue = from.Add(interval)
	job.NextRun = job.nextDue.Format("2006-01-02 15:04:05")
}

func parseEvery(schedule string) (time.Duration, bool) {
	schedule = strings.TrimSpace(schedule)
	if !strings.HasPrefix(schedule, "@every ") {
		return 0, false
	}
	interval, err := time.ParseDuration(strings.TrimSpace(strings.TrimPrefix(schedule, "@every ")))
	if err != nil || interval <= 0 {
		return 0, false
	}
	return interval, true
}
