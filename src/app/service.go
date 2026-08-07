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
// mu is released. Blocking file I/O follows the same rule: mu is the lock the
// Fyne main thread takes on every Jobs() and Runtime() call, so a JSON write, a
// log-directory scan, or a pass over every log header must not happen inside it
// (see deferSaveLocked, executeRun, and applySeededStatsLocked).
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
	runJob func(ctx context.Context, job *domain.Job, trigger string, logsDir string, timeout time.Duration) (domain.RunRecord, error)
	ctx    context.Context

	// sched is the timing loop installed by Start; cancel tears down ctx on Stop.
	// Both are guarded by mu.
	sched  *scheduler.Scheduler
	cancel context.CancelFunc

	// manager is the platform autostart implementation. It is nil in tests that
	// do not exercise autostart; Open() wires it via autostart.New().
	manager autostart.Manager

	// saveMu serializes the store writes that operations prepare under mu and run
	// after releasing it. It is taken while mu is still held and released once the
	// write is done, so writes reach the file in the same order their snapshots
	// were taken and an older snapshot can never land on top of a newer one.
	// Nothing may take mu while holding saveMu.
	saveMu sync.Mutex

	// observers and their guard live in events.go. dispatchMu is separate from mu
	// so that emitting an event never requires (or is held under) the state lock:
	// the Service must release mu before dispatching, per the locking contract.
	dispatchMu sync.Mutex
	observers  []Observer
}

// deferSaveLocked prepares the store writes for the caller to run after mu is
// released, and takes saveMu now so a later operation's write cannot overtake
// this one. The caller must hold mu, must unlock it before calling the returned
// function, and must call that function exactly once. Keeping the marshal, the
// fsync, and the rename out of the critical section is what stops a settings
// change or a job edit from blocking a scheduler tick or a finishing run. The
// writes run in the order given and stop at the first error.
func (s *Service) deferSaveLocked(writes ...func() error) func() error {
	s.saveMu.Lock()
	return func() error {
		defer s.saveMu.Unlock()
		for _, write := range writes {
			if err := write(); err != nil {
				return err
			}
		}
		return nil
	}
}

// NewService wires the Service to a loaded store and its jobs. It builds the
// initial runtime map from the durable jobs so every job has transient state
// from the moment the Service exists, and parses each job's schedule once. The
// store is the Service's sole channel to persistence.
func NewService(store *storage.Store, jobs []domain.Job) *Service {
	s := &Service{
		store:  store,
		runJob: runner.RunJob,
		ctx:    context.Background(),
		paused: store.Config.Paused,
	}
	// No lock is needed here: construction is single-threaded, before Start
	// launches the timing loop.
	s.adoptJobsLocked(jobs)
	s.applySeededStatsLocked(runner.SeedStats(store.Paths.LogsDir, s.jobs, store.Config.MaxLogFiles))
	return s
}

// adoptJobsLocked makes jobs the Service's durable state and rebuilds everything
// derived from it: the runtime map, the parsed-schedule cache, and each job's
// first next-run — so the Service is ready to schedule the moment it exists,
// mirroring the old scheduler's reset-on-construction.
//
// It backs both construction and a Settings change that points at a different
// jobs file. Statistics seeded from existing log files are applied separately by
// applySeededStatsLocked, because reconstructing them is file I/O. The caller
// must hold mu.
func (s *Service) adoptJobsLocked(jobs []domain.Job) {
	s.jobs = jobs
	s.runtimes = domain.NewRuntimes(jobs)
	s.schedules = make(map[int]domain.Schedule, len(jobs))

	now := time.Now()
	for index := range s.jobs {
		job := &s.jobs[index]
		s.parseScheduleLocked(job)
		s.refreshNextRunFromLocked(job, s.runtimes[job.ID], now)
	}
}

// applySeededStatsLocked folds statistics reconstructed from existing log files
// into the runtime map, so the details panel shows accumulated run history
// immediately rather than only runs since this process started. It is separate
// from adoptJobsLocked because producing the seeds opens every log file in the
// directory, which must not happen under mu: callers compute the map first and
// apply it here. The caller must hold mu.
func (s *Service) applySeededStatsLocked(seeds map[int]runner.SeededStats) {
	for id, seed := range seeds {
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
		runtime.DurationSumMS = seed.DurationSumMS
	}
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

// Config returns a copy of the current application configuration, safe to
// call from any goroutine. UpdateSettings, SetGlobalPause, and SetJobListView
// are the only writers and all mutate store.Config under mu; copying under the
// same lock is what keeps a UI read from racing them, instead of holding onto
// the *storage.Store this used to hand out (see STANDARDS: the UI reads
// Service state through typed events and accessors, never shared mutable
// state).
func (s *Service) Config() domain.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.Config
}

// Paths returns a copy of the store's resolved filesystem paths. AppDir and
// ConfigPath are fixed for the process; JobsPath, JobsDir, and LogsDir are
// re-derived under mu on every settings save (storage.Store.applyConfigPaths),
// so this copies under the same lock as Config for the same reason.
func (s *Service) Paths() storage.Paths {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.Paths
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
