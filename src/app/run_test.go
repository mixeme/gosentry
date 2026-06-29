package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

// newQueueService builds a temp-backed Service with a chosen execution mode and
// overlap policy. Config is set before any run starts, so mutating it directly is
// safe (construction is single-threaded).
func newQueueService(t *testing.T, mode domain.ExecutionMode, policy domain.OverlapPolicy, jobs []domain.Job) *Service {
	t.Helper()
	svc := newTempService(t, jobs)
	svc.store.Config.ExecutionMode = mode
	svc.store.Config.OverlapPolicy = policy
	return svc
}

// primeDue forces the given jobs to be due by backdating their NextDue. Tests tick
// RunDue at the real wall clock; a long "@every 1h" schedule then advances a
// started job's NextDue an hour out, so it does not spuriously re-fire on a later
// tick unless a test re-primes it.
func primeDue(t *testing.T, svc *Service, ids ...int) {
	t.Helper()
	svc.mu.Lock()
	defer svc.mu.Unlock()
	for _, id := range ids {
		if rt := svc.runtimes[id]; rt != nil {
			rt.NextDue = time.Now().Add(-time.Second)
		}
	}
}

// completions subscribes a recorder that forwards every RunRecorded onto a
// channel, so tests can wait for runs to finish (and drain in-flight runs before
// the temp dir is cleaned up).
func completions(svc *Service) <-chan domain.RunRecord {
	ch := make(chan domain.RunRecord, 16)
	svc.Subscribe(ObserverFunc(func(e Event) {
		if rr, ok := e.(RunRecorded); ok {
			ch <- rr.Record
		}
	}))
	return ch
}

func waitRecord(t *testing.T, ch <-chan domain.RunRecord) domain.RunRecord {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a run record")
		return domain.RunRecord{}
	}
}

func expectNoEntry(t *testing.T, entered <-chan int) {
	t.Helper()
	select {
	case id := <-entered:
		t.Fatalf("a job (%d) started unexpectedly", id)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestUpdateStats verifies that aggregate statistics are folded correctly after
// a sequence of fake runs with varying durations and states.
func TestUpdateStats(t *testing.T) {
	rt := &domain.JobRuntime{}

	// First run: success, 200 ms.
	updateStats(rt, domain.RunRecord{State: "OK", DurationMS: 200})
	if rt.RunCount != 1 || rt.FailCount != 0 {
		t.Fatalf("after run 1: RunCount=%d FailCount=%d, want 1/0", rt.RunCount, rt.FailCount)
	}
	if rt.LastDurationMS != 200 || rt.MaxDurationMS != 200 || rt.AvgDurationMS != 200 {
		t.Errorf("after run 1: last=%d max=%d avg=%d, want 200/200/200",
			rt.LastDurationMS, rt.MaxDurationMS, rt.AvgDurationMS)
	}

	// Second run: failure, 400 ms.
	updateStats(rt, domain.RunRecord{State: "Failed", DurationMS: 400})
	if rt.RunCount != 2 || rt.FailCount != 1 {
		t.Fatalf("after run 2: RunCount=%d FailCount=%d, want 2/1", rt.RunCount, rt.FailCount)
	}
	if rt.LastDurationMS != 400 || rt.MaxDurationMS != 400 {
		t.Errorf("after run 2: last=%d max=%d, want 400/400", rt.LastDurationMS, rt.MaxDurationMS)
	}
	if rt.AvgDurationMS != 300 {
		t.Errorf("after run 2: avg=%d, want 300", rt.AvgDurationMS)
	}

	// Third run: success, 100 ms — avg should be (200+400+100)/3 = 233.
	updateStats(rt, domain.RunRecord{State: "OK", DurationMS: 100})
	if rt.LastDurationMS != 100 || rt.MaxDurationMS != 400 {
		t.Errorf("after run 3: last=%d max=%d, want 100/400", rt.LastDurationMS, rt.MaxDurationMS)
	}
	if rt.AvgDurationMS != 233 {
		t.Errorf("after run 3: avg=%d, want 233", rt.AvgDurationMS)
	}
}

// TestRunDueParallelStartsAllDueJobs verifies that in parallel mode every due job
// starts at once: both runs are in flight (blocked in the runner) before either
// is released.
func TestRunDueParallelStartsAllDueJobs(t *testing.T) {
	svc := newQueueService(t, domain.ExecutionModeParallel, domain.OverlapPolicySkip, []domain.Job{
		{ID: 1, Name: "A", Schedule: "@every 1h", Command: "echo", Enabled: true},
		{ID: 2, Name: "B", Schedule: "@every 1h", Command: "echo", Enabled: true},
	})

	entered := make(chan int, 2)
	release := make(chan struct{})
	svc.runJob = func(_ context.Context, job *domain.Job, _ string, _ string) (domain.RunRecord, error) {
		entered <- job.ID
		<-release
		return domain.RunRecord{Time: "t", JobID: job.ID, JobName: job.Name, State: "Success"}, nil
	}
	done := completions(svc)

	primeDue(t, svc, 1, 2)
	svc.RunDue(time.Now())

	// Both runs must reach the runner (and block) before any is released, which is
	// only possible if RunDue started them concurrently.
	got := map[int]bool{}
	for i := 0; i < 2; i++ {
		select {
		case id := <-entered:
			got[id] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d job(s) started in parallel, want 2", len(got))
		}
	}
	if !got[1] || !got[2] {
		t.Fatalf("started jobs = %v, want both 1 and 2", got)
	}

	close(release)
	waitRecord(t, done)
	waitRecord(t, done)
}

// TestRunDueSequentialSerializes verifies that in sequential mode only one due job
// runs at a time: the second due job waits until the first finishes and a later
// tick picks it up.
func TestRunDueSequentialSerializes(t *testing.T) {
	svc := newQueueService(t, domain.ExecutionModeSequential, domain.OverlapPolicySkip, []domain.Job{
		{ID: 1, Name: "A", Schedule: "@every 1h", Command: "echo", Enabled: true},
		{ID: 2, Name: "B", Schedule: "@every 1h", Command: "echo", Enabled: true},
	})

	entered := make(chan int, 2)
	release := make(chan struct{})
	svc.runJob = func(_ context.Context, job *domain.Job, _ string, _ string) (domain.RunRecord, error) {
		entered <- job.ID
		<-release
		return domain.RunRecord{Time: "t", JobID: job.ID, JobName: job.Name, State: "Success"}, nil
	}
	done := completions(svc)

	primeDue(t, svc, 1, 2)
	svc.RunDue(time.Now())

	// Exactly one job — the first in order — starts; the second is held back.
	if id := <-entered; id != 1 {
		t.Fatalf("first started job = %d, want 1", id)
	}
	expectNoEntry(t, entered)

	// Let the first run finish, then tick again: now the second job runs.
	close(release)
	waitRecord(t, done)
	svc.RunDue(time.Now())
	if id := <-entered; id != 2 {
		t.Fatalf("second started job = %d, want 2", id)
	}
	waitRecord(t, done)
}

// TestRunDueSkipDropsOverlap verifies that under the "skip" overlap policy a job
// coming due again while its own run is in flight does not queue or start a second
// run.
func TestRunDueSkipDropsOverlap(t *testing.T) {
	svc := newQueueService(t, domain.ExecutionModeParallel, domain.OverlapPolicySkip, []domain.Job{
		{ID: 1, Name: "A", Schedule: "@every 1h", Command: "echo", Enabled: true},
	})

	entered := make(chan int, 2)
	release := make(chan struct{})
	var calls int32
	svc.runJob = func(_ context.Context, job *domain.Job, _ string, _ string) (domain.RunRecord, error) {
		atomic.AddInt32(&calls, 1)
		entered <- job.ID
		<-release
		return domain.RunRecord{Time: "t", JobID: job.ID, JobName: job.Name, State: "Success"}, nil
	}
	done := completions(svc)

	primeDue(t, svc, 1)
	svc.RunDue(time.Now())
	if id := <-entered; id != 1 {
		t.Fatalf("started job = %d, want 1", id)
	}

	// The job is now in flight; make it due again and tick. Skip must drop it.
	primeDue(t, svc, 1)
	svc.RunDue(time.Now())
	expectNoEntry(t, entered)

	svc.mu.Lock()
	pending := svc.runtimes[1].Pending
	svc.mu.Unlock()
	if pending {
		t.Error("skip policy must not mark the job Pending")
	}

	close(release)
	waitRecord(t, done)
	// No re-run is queued, so the runner is invoked exactly once.
	expectNoEntry(t, entered)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("runner called %d time(s), want 1", got)
	}
}

// TestRunDueQueueRerunsAfterFinish verifies that under the "queue" overlap policy a
// job coming due again while running is marked Pending and re-run as soon as the
// in-flight run finishes.
func TestRunDueQueueRerunsAfterFinish(t *testing.T) {
	svc := newQueueService(t, domain.ExecutionModeParallel, domain.OverlapPolicyQueue, []domain.Job{
		{ID: 1, Name: "A", Schedule: "@every 1h", Command: "echo", Enabled: true},
	})

	entered := make(chan int, 2)
	release := make(chan struct{})
	var calls int32
	svc.runJob = func(_ context.Context, job *domain.Job, _ string, _ string) (domain.RunRecord, error) {
		atomic.AddInt32(&calls, 1)
		entered <- job.ID
		<-release
		return domain.RunRecord{Time: "t", JobID: job.ID, JobName: job.Name, State: "Success"}, nil
	}
	done := completions(svc)

	primeDue(t, svc, 1)
	svc.RunDue(time.Now())
	if id := <-entered; id != 1 {
		t.Fatalf("started job = %d, want 1", id)
	}

	// Re-due the running job and tick: queue must mark it Pending without starting
	// a second concurrent run.
	primeDue(t, svc, 1)
	svc.RunDue(time.Now())
	expectNoEntry(t, entered)

	svc.mu.Lock()
	pending := svc.runtimes[1].Pending
	svc.mu.Unlock()
	if !pending {
		t.Fatal("queue policy must mark the job Pending")
	}

	// Releasing the first run lets executeRun start the deferred run automatically.
	close(release)
	waitRecord(t, done)
	if id := <-entered; id != 1 {
		t.Fatalf("re-run job = %d, want 1", id)
	}
	waitRecord(t, done)

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("runner called %d time(s), want 2 (original + queued re-run)", got)
	}
	svc.mu.Lock()
	pending = svc.runtimes[1].Pending
	svc.mu.Unlock()
	if pending {
		t.Error("Pending must be cleared after the re-run starts")
	}
}

// TestRunDuePerJobQueueOverridesGlobalSkip verifies that a job carrying its own
// "queue" policy queues a re-run even though the global default is "skip": the
// effective policy is resolved per job, so the job-level value wins.
func TestRunDuePerJobQueueOverridesGlobalSkip(t *testing.T) {
	svc := newQueueService(t, domain.ExecutionModeParallel, domain.OverlapPolicySkip, []domain.Job{
		{ID: 1, Name: "A", Schedule: "@every 1h", Command: "echo", Enabled: true, OverlapPolicy: string(domain.OverlapPolicyQueue)},
	})

	entered := make(chan int, 2)
	release := make(chan struct{})
	var calls int32
	svc.runJob = func(_ context.Context, job *domain.Job, _ string, _ string) (domain.RunRecord, error) {
		atomic.AddInt32(&calls, 1)
		entered <- job.ID
		<-release
		return domain.RunRecord{Time: "t", JobID: job.ID, JobName: job.Name, State: "Success"}, nil
	}
	done := completions(svc)

	primeDue(t, svc, 1)
	svc.RunDue(time.Now())
	if id := <-entered; id != 1 {
		t.Fatalf("started job = %d, want 1", id)
	}

	// Re-due the running job and tick. Despite the global "skip", the job's own
	// "queue" policy must mark it Pending.
	primeDue(t, svc, 1)
	svc.RunDue(time.Now())
	expectNoEntry(t, entered)

	svc.mu.Lock()
	pending := svc.runtimes[1].Pending
	svc.mu.Unlock()
	if !pending {
		t.Fatal("per-job queue policy must mark the job Pending despite global skip")
	}

	// Releasing the first run lets executeRun start the deferred re-run.
	close(release)
	waitRecord(t, done)
	if id := <-entered; id != 1 {
		t.Fatalf("re-run job = %d, want 1", id)
	}
	waitRecord(t, done)
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("runner called %d time(s), want 2 (original + queued re-run)", got)
	}
}

// TestRunDuePerJobSkipOverridesGlobalQueue verifies the reverse override: a job
// carrying its own "skip" policy drops an overlapping run even though the global
// default is "queue".
func TestRunDuePerJobSkipOverridesGlobalQueue(t *testing.T) {
	svc := newQueueService(t, domain.ExecutionModeParallel, domain.OverlapPolicyQueue, []domain.Job{
		{ID: 1, Name: "A", Schedule: "@every 1h", Command: "echo", Enabled: true, OverlapPolicy: string(domain.OverlapPolicySkip)},
	})

	entered := make(chan int, 2)
	release := make(chan struct{})
	var calls int32
	svc.runJob = func(_ context.Context, job *domain.Job, _ string, _ string) (domain.RunRecord, error) {
		atomic.AddInt32(&calls, 1)
		entered <- job.ID
		<-release
		return domain.RunRecord{Time: "t", JobID: job.ID, JobName: job.Name, State: "Success"}, nil
	}
	done := completions(svc)

	primeDue(t, svc, 1)
	svc.RunDue(time.Now())
	if id := <-entered; id != 1 {
		t.Fatalf("started job = %d, want 1", id)
	}

	// Re-due the running job and tick. Despite the global "queue", the job's own
	// "skip" policy must drop it without marking Pending.
	primeDue(t, svc, 1)
	svc.RunDue(time.Now())
	expectNoEntry(t, entered)

	svc.mu.Lock()
	pending := svc.runtimes[1].Pending
	svc.mu.Unlock()
	if pending {
		t.Error("per-job skip policy must not mark the job Pending despite global queue")
	}

	close(release)
	waitRecord(t, done)
	expectNoEntry(t, entered)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("runner called %d time(s), want 1", got)
	}
}

// TestRunDueEmptyOverlapInheritsGlobal verifies that a job with no own policy
// inherits the global default: with global "queue" and an empty Job.OverlapPolicy
// the job queues a re-run.
func TestRunDueEmptyOverlapInheritsGlobal(t *testing.T) {
	svc := newQueueService(t, domain.ExecutionModeParallel, domain.OverlapPolicyQueue, []domain.Job{
		{ID: 1, Name: "A", Schedule: "@every 1h", Command: "echo", Enabled: true},
	})
	if svc.jobs[0].OverlapPolicy != "" {
		t.Fatalf("test setup: job OverlapPolicy = %q, want empty (inherit)", svc.jobs[0].OverlapPolicy)
	}

	entered := make(chan int, 2)
	release := make(chan struct{})
	svc.runJob = func(_ context.Context, job *domain.Job, _ string, _ string) (domain.RunRecord, error) {
		entered <- job.ID
		<-release
		return domain.RunRecord{Time: "t", JobID: job.ID, JobName: job.Name, State: "Success"}, nil
	}
	done := completions(svc)

	primeDue(t, svc, 1)
	svc.RunDue(time.Now())
	if id := <-entered; id != 1 {
		t.Fatalf("started job = %d, want 1", id)
	}

	primeDue(t, svc, 1)
	svc.RunDue(time.Now())
	expectNoEntry(t, entered)

	svc.mu.Lock()
	pending := svc.runtimes[1].Pending
	svc.mu.Unlock()
	if !pending {
		t.Fatal("empty per-job policy must inherit the global queue and mark Pending")
	}

	close(release)
	waitRecord(t, done)
	if id := <-entered; id != 1 {
		t.Fatalf("inherited-queue re-run job = %d, want 1", id)
	}
	waitRecord(t, done)
}

// TestRunNowSequentialGuard verifies the sequential-mode guard in RunNow: a manual
// run is refused while another job is running, and allowed once nothing is.
func TestRunNowSequentialGuard(t *testing.T) {
	svc := newQueueService(t, domain.ExecutionModeSequential, domain.OverlapPolicySkip, []domain.Job{
		{ID: 1, Name: "A", Schedule: "@every 1h", Command: "echo", Enabled: true},
		{ID: 2, Name: "B", Schedule: "@every 1h", Command: "echo", Enabled: true},
	})

	entered := make(chan int, 2)
	release := make(chan struct{})
	svc.runJob = func(_ context.Context, job *domain.Job, _ string, _ string) (domain.RunRecord, error) {
		entered <- job.ID
		<-release
		return domain.RunRecord{Time: "t", JobID: job.ID, JobName: job.Name, State: "Success"}, nil
	}
	done := completions(svc)

	if err := svc.RunNow(1); err != nil {
		t.Fatalf("first RunNow: %v", err)
	}
	<-entered // job 1 is in flight

	if err := svc.RunNow(2); err == nil {
		t.Error("expected RunNow to be refused while another job runs (sequential mode)")
	}

	// Once job 1 finishes, a manual run of job 2 is allowed.
	close(release)
	waitRecord(t, done)
	if err := svc.RunNow(2); err != nil {
		t.Fatalf("RunNow after first finished: %v", err)
	}
	if id := <-entered; id != 2 {
		t.Fatalf("second manual run job = %d, want 2", id)
	}
	waitRecord(t, done)
}
