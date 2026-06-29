package app

import (
	"context"
	"sync"
	"time"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"gitea.mixdep.ru/mix/gosentry/src/platform/autostart"
	"gitea.mixdep.ru/mix/gosentry/src/runner"
	"gitea.mixdep.ru/mix/gosentry/src/scheduler"
	"gitea.mixdep.ru/mix/gosentry/src/storage"
)

// Service is the application-service layer: the single owner of GoSentry's
// in-memory state. It holds the durable jobs slice, the transient runtime map
// keyed by Job.ID, and a reference to the store that persists them. All access
// to that state goes through a mutex so the GUI and the scheduler can no longer
// race on a shared *[]Job.
//
// Mutations live in operations.go; scheduling and run dispatch live in run.go;
// typed events live in events.go. The scheduler is a thin timing loop that calls
// RunDue on every tick and holds no job state of its own.
//
// Locking contract: mu is a plain, non-reentrant mutex. Exported methods take
// it; unexported helpers ending in "Locked" assume the caller already holds it.
// The Service must never call back into the UI (or any code that might re-enter
// the Service) while holding mu — in particular emit() is always called after
// mu is released.
type Service struct {
	mu       sync.Mutex
	store    *storage.Store
	jobs     []domain.Job
	runtimes map[int]*domain.JobRuntime

	// schedules caches a parsed Schedule per job ID so timing math does not
	// re-parse the schedule string on every use. paused is the global pause flag.
	// Both are guarded by mu.
	schedules map[int]domain.Schedule
	paused    bool

	// runJob is the run seam. It defaults to runner.RunJob and is overridden in
	// tests with a fake so the run paths can be exercised without spawning real
	// processes. ctx is the lifecycle context passed to runs; Start replaces it
	// with a cancelable context so Stop can abort in-flight runs, and until Start
	// it is context.Background().
	runJob func(ctx context.Context, job *domain.Job, trigger string, logsDir string) (domain.RunRecord, error)
	ctx    context.Context

	// sched is the timing loop installed by Start; cancel tears down ctx on Stop.
	// Both are guarded by mu.
	sched  *scheduler.Scheduler
	cancel context.CancelFunc

	// manager is the platform autostart implementation. It is nil in tests that
	// do not exercise autostart; Open() wires it via autostart.New().
	manager autostart.Manager

	// observers and their guard live in events.go. dispatchMu is separate from mu
	// so that emitting an event never requires (or is held under) the state lock:
	// the Service must release mu before dispatching, per the locking contract.
	dispatchMu sync.Mutex
	observers  []Observer
}

// NewService wires the Service to a loaded store and its jobs. It builds the
// initial runtime map from the durable jobs so every job has transient state
// from the moment the Service exists, and parses each job's schedule once. The
// store is the Service's sole channel to persistence.
func NewService(store *storage.Store, jobs []domain.Job) *Service {
	s := &Service{
		store:     store,
		jobs:      jobs,
		runtimes:  domain.NewRuntimes(jobs),
		schedules: make(map[int]domain.Schedule, len(jobs)),
		runJob:    runner.RunJob,
		ctx:       context.Background(),
		paused:    store.Config.Paused,
	}
	// Parse every schedule once, then compute each job's first next-run so the
	// Service is ready to schedule the moment it exists — mirroring the old
	// scheduler's reset-on-construction. No lock is needed: construction is
	// single-threaded, before Start launches the timing loop.
	now := time.Now()
	for index := range s.jobs {
		job := &s.jobs[index]
		s.parseScheduleLocked(job)
		s.refreshNextRunFromLocked(job, s.runtimes[job.ID], now)
	}
	// Seed execution-time statistics from existing log files so the details panel
	// shows accumulated run history immediately after a restart, not just runs
	// since this process started.
	for id, seed := range runner.SeedStats(store.Paths.LogsDir, jobs, store.Config.MaxLogFiles) {
		runtime := s.runtimes[id]
		if runtime == nil {
			continue
		}
		runtime.RunCount = seed.RunCount
		runtime.FailCount = seed.FailCount
		runtime.LastDurationMS = seed.LastDurationMS
		runtime.AvgDurationMS = seed.AvgDurationMS
		runtime.MaxDurationMS = seed.MaxDurationMS
		runtime.TimedRunCount = seed.TimedRunCount
	}
	return s
}

// Start begins scheduling with the real wall clock. It is the production entry
// point; tests should call StartWith and supply a fake clock instead. Start is
// expected once, during setup, before any concurrent use.
func (s *Service) Start() {
	s.StartWith(scheduler.NewRealClock())
}

// StartWith begins scheduling driven by the given clock; every tick calls
// RunDue. Used by tests to inject a fake clock.
func (s *Service) StartWith(clock scheduler.Clock) {
	s.mu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel
	s.sched = scheduler.NewScheduler(clock, s.RunDue)
	sched := s.sched
	s.mu.Unlock()

	sched.Start()
}

// Stop halts scheduling and cancels the run context so in-flight runs see a
// canceled context. It is safe to call when Start was never called.
func (s *Service) Stop() {
	s.mu.Lock()
	sched := s.sched
	cancel := s.cancel
	s.mu.Unlock()

	if sched != nil {
		sched.Stop()
	}
	if cancel != nil {
		cancel()
	}
}

// Open loads the store and constructs a Service from it in one step. It is the
// convenience entry point for the application; tests inject a pre-built store
// via NewService instead.
func Open() (*Service, error) {
	store, jobs, err := storage.OpenStore()
	if err != nil {
		return nil, err
	}
	svc := NewService(store, jobs)
	svc.manager = autostart.New()
	return svc, nil
}

// Store returns the underlying store. It is exposed so callers that still need
// resolved paths and config (the GUI, during the transition) can reach them;
// later phases narrow this surface.
func (s *Service) Store() *storage.Store {
	return s.store
}

// Jobs returns a copy of the durable jobs slice. Returning a copy keeps callers
// from mutating Service-owned state behind its back: the Service stays the sole
// writer.
func (s *Service) Jobs() []domain.Job {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobs := make([]domain.Job, len(s.jobs))
	copy(jobs, s.jobs)
	return jobs
}

// Runtime returns the transient runtime state for a job ID, or nil if no job
// with that ID is loaded. The returned pointer is the live runtime; reads of it
// are only safe while no concurrent mutation is in flight. The UI listener
// marshals reads onto the main thread via fyne.Do.
func (s *Service) Runtime(id int) *domain.JobRuntime {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.runtimes[id]
}
