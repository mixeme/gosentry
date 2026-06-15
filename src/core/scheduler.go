package core

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// Scheduler owns the timing loop for jobs that are currently loaded in the GUI.
// It receives a pointer to the jobs slice because the GUI edits the same slice;
// this keeps the early architecture simple while storage and scheduling are
// still in one desktop process.
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
	// A one-second ticker is accurate enough for cron-style desktop automation
	// and avoids the complexity of maintaining one timer per job. Five-field cron
	// expressions have minute precision, while @every values may be shorter for
	// testing and lightweight local tasks.
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
	// Pause state is reflected into each job's display string so the list view is
	// understandable even before the next scheduler tick.
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

func (s *Scheduler) RunNow(index int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(*s.jobs) {
		return false
	}
	// Manual runs share the same runner and log writer as scheduled runs. The
	// Trigger field is the only difference, which keeps History comparable and
	// prevents "Run now" from becoming a separate behavior path.
	return s.startRunLocked(index, "Manual")
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
	var changed bool

	s.mu.Lock()
	if !s.paused {
		for index := range *s.jobs {
			job := &(*s.jobs)[index]
			if !job.Enabled || job.nextDue.IsZero() || now.Before(job.nextDue) {
				continue
			}
			// Run only one due job per tick for now. That avoids overlapping shell
			// commands in the GUI process and keeps the first version predictable;
			// a future worker pool can add concurrency once cancellation and status
			// reporting are more explicit.
			changed = s.startRunLocked(index, "Schedule")
			break
		}
	}
	s.mu.Unlock()
	_ = changed
}

func (s *Scheduler) startRunLocked(index int, trigger string) bool {
	job := &(*s.jobs)[index]
	if job.LastState == "Running" {
		return false
	}

	jobCopy := *job
	job.LastState = "Running"
	job.NextRun = "Running"
	job.nextDue = time.Time{}
	_ = s.store.SaveJobs(*s.jobs)

	go func() {
		record := RunJob(s.ctx, &jobCopy, trigger, s.store.Paths.LogsDir)

		s.mu.Lock()
		if current := s.findJobByIDLocked(jobCopy.ID); current != nil {
			current.LastRun = record.Time
			current.LastState = record.State
			current.Output = record.Output
			current.Logs = append([]RunRecord{record}, current.Logs...)
			if len(current.Logs) > 50 {
				current.Logs = current.Logs[:50]
			}
			s.prepareNextRun(current, time.Now())
			_ = CleanupLogs(s.store.Paths.LogsDir, s.store.Config.MaxLogFiles, s.store.Config.MaxLogAgeDays)
			_ = s.store.SaveJobs(*s.jobs)
		}
		s.mu.Unlock()

		if s.onChange != nil {
			s.onChange(record)
		}
	}()
	return true
}

func (s *Scheduler) findJobByIDLocked(id int) *Job {
	for index := range *s.jobs {
		if (*s.jobs)[index].ID == id {
			return &(*s.jobs)[index]
		}
	}
	return nil
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
	next, ok := nextRunTime(job.Schedule, from)
	if !ok {
		job.NextRun = "Invalid schedule"
		job.nextDue = time.Time{}
		return
	}
	job.nextDue = next
	job.NextRun = job.nextDue.Format("2006-01-02 15:04:05")
}

func nextRunTime(schedule string, from time.Time) (time.Time, bool) {
	schedule = strings.TrimSpace(schedule)
	if schedule == "" {
		return time.Time{}, false
	}
	if strings.HasPrefix(schedule, "@every ") {
		// @every is kept alongside cron because it is convenient for quick tests
		// and for simple intervals that are awkward to express as five fields.
		interval, err := time.ParseDuration(strings.TrimSpace(strings.TrimPrefix(schedule, "@every ")))
		if err != nil || interval <= 0 {
			return time.Time{}, false
		}
		return from.Add(interval), true
	}
	// Standard five-field cron keeps PySentry compatible with the mental model
	// users already know from Unix cron, while robfig/cron handles edge cases
	// such as ranges, steps, and day-of-week names.
	parsed, err := cronParser.Parse(schedule)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.Next(from), true
}
