package app

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"gitea.mixdep.ru/mix/gosentry/src/storage"
)

// newTempService builds a Service backed by a store rooted in a temp directory,
// so the mutating operations can persist to real (throwaway) files.
func newTempService(t *testing.T, jobs []domain.Job) *Service {
	t.Helper()
	dir := t.TempDir()
	store := &storage.Store{
		Paths: storage.Paths{
			ExecutablePath: filepath.Join(dir, "gosentry"),
			AppDir:         dir,
			ConfigPath:     filepath.Join(dir, "gosentry.json"),
			JobsDir:        dir,
			JobsPath:       filepath.Join(dir, "jobs.json"),
			LogsDir:        filepath.Join(dir, "logs"),
		},
		Config: domain.Config{JobsDir: ".", LogsDir: "logs", MaxLogFiles: 100, MaxLogAgeDays: 30, ExecutionMode: domain.ExecutionModeParallel, OverlapPolicy: domain.OverlapPolicySkip},
	}
	return NewService(store, jobs)
}

// recorder is a test observer that captures every emitted event.
type recorder struct {
	events []Event
}

func (r *recorder) OnEvent(e Event) { r.events = append(r.events, e) }

func (r *recorder) jobChanged() (ids []int) {
	for _, e := range r.events {
		if jc, ok := e.(JobChanged); ok {
			ids = append(ids, jc.JobID)
		}
	}
	return ids
}

func (r *recorder) records() (out []domain.RunRecord) {
	for _, e := range r.events {
		if rr, ok := e.(RunRecorded); ok {
			out = append(out, rr.Record)
		}
	}
	return out
}

func TestCreateJobAssignsIDAndEmits(t *testing.T) {
	svc := newTempService(t, nil)
	rec := &recorder{}
	svc.Subscribe(rec)

	created, err := svc.CreateJob(domain.Job{Name: "Build", Schedule: "@every 1m", Command: "echo hi", Enabled: true})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if created.ID != 1 {
		t.Errorf("first job ID = %d, want 1", created.ID)
	}
	if got := svc.Jobs(); len(got) != 1 || got[0].Name != "Build" {
		t.Fatalf("jobs after create = %+v", got)
	}
	if rt := svc.Runtime(1); rt == nil || rt.LastState != "Ready" {
		t.Errorf("runtime = %+v, want LastState Ready", rt)
	}
	if recs := rec.records(); len(recs) != 1 || recs[0].State != "Created" {
		t.Errorf("records = %+v, want one Created", recs)
	}
	if ids := rec.jobChanged(); len(ids) != 1 || ids[0] != 1 {
		t.Errorf("JobChanged ids = %v, want [1]", ids)
	}

	// A second job takes the next free ID.
	second, err := svc.CreateJob(domain.Job{Name: "Two", Schedule: "@every 1m", Command: "echo two"})
	if err != nil {
		t.Fatalf("CreateJob 2: %v", err)
	}
	if second.ID != 2 {
		t.Errorf("second job ID = %d, want 2", second.ID)
	}
}

func TestCreateJobValidates(t *testing.T) {
	svc := newTempService(t, nil)
	if _, err := svc.CreateJob(domain.Job{Schedule: "@every 1m", Command: "echo"}); err == nil {
		t.Error("expected error for missing name")
	}
	if got := svc.Jobs(); len(got) != 0 {
		t.Errorf("invalid job should not be stored, jobs = %+v", got)
	}
}

func TestUpdateJobKeepsRuntimeAndReflectsDisable(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 5, Name: "Old", Schedule: "@every 1m", Command: "echo", Enabled: true}})
	if err := svc.UpdateJob(domain.Job{ID: 5, Name: "New", Schedule: "@every 1m", Command: "echo", Enabled: false}); err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}
	got := svc.Jobs()
	if got[0].Name != "New" || got[0].Enabled {
		t.Errorf("job after update = %+v", got[0])
	}
	if rt := svc.Runtime(5); rt == nil || rt.LastState != "Paused" || rt.NextRun != "Paused" {
		t.Errorf("runtime after disable = %+v", rt)
	}
}

func TestUpdateJobReenablesPausedJob(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 5, Name: "Old", Schedule: "@every 1m", Command: "echo", Enabled: false}})
	if rt := svc.Runtime(5); rt.LastState != "Paused" {
		t.Fatalf("precondition: runtime = %+v, want Paused", rt)
	}
	if err := svc.UpdateJob(domain.Job{ID: 5, Name: "Old", Schedule: "@every 1m", Command: "echo", Enabled: true}); err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}
	if rt := svc.Runtime(5); rt.LastState != "Ready" || rt.NextDue.IsZero() {
		t.Errorf("re-enabled runtime = %+v, want Ready with a next-due", rt)
	}
}

// runtimeForLocked lazily recreates a missing runtime entry so the Service stays
// robust if a job somehow lacks one. Dropping the entry and driving an operation
// that needs it exercises that path.
func TestRuntimeLazilyRecreated(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}})

	svc.mu.Lock()
	delete(svc.runtimes, 1)
	svc.mu.Unlock()

	if err := svc.SetEnabled(1, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if rt := svc.Runtime(1); rt == nil {
		t.Error("runtime was not lazily recreated")
	}
}

func TestUpdateJobNotFound(t *testing.T) {
	svc := newTempService(t, nil)
	if err := svc.UpdateJob(domain.Job{ID: 99, Name: "X", Schedule: "@every 1m", Command: "echo"}); err == nil {
		t.Error("expected not-found error")
	}
}

func TestDeleteJobRemovesEverything(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}})
	rec := &recorder{}
	svc.Subscribe(rec)

	if err := svc.DeleteJob(1); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
	if got := svc.Jobs(); len(got) != 0 {
		t.Errorf("jobs after delete = %+v", got)
	}
	if rt := svc.Runtime(1); rt != nil {
		t.Errorf("runtime should be gone, got %+v", rt)
	}
	if recs := rec.records(); len(recs) != 1 || recs[0].State != "Deleted" {
		t.Errorf("records = %+v, want one Deleted", recs)
	}
	if ids := rec.jobChanged(); len(ids) != 1 || ids[0] != 0 {
		t.Errorf("JobChanged ids = %v, want [0] (broad)", ids)
	}
}

func TestDeleteJobNotFound(t *testing.T) {
	svc := newTempService(t, nil)
	if err := svc.DeleteJob(42); err == nil {
		t.Error("expected not-found error deleting unknown job")
	}
}

func TestSetEnabledNotFound(t *testing.T) {
	svc := newTempService(t, nil)
	if err := svc.SetEnabled(42, true); err == nil {
		t.Error("expected not-found error enabling unknown job")
	}
}

func TestSetEnabledToggles(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: false}})

	if err := svc.SetEnabled(1, true); err != nil {
		t.Fatalf("SetEnabled true: %v", err)
	}
	if rt := svc.Runtime(1); rt.LastState != "Ready" || rt.NextDue.IsZero() {
		t.Errorf("enabled runtime = %+v, want Ready with a next-due", rt)
	}
	if err := svc.SetEnabled(1, false); err != nil {
		t.Fatalf("SetEnabled false: %v", err)
	}
	if rt := svc.Runtime(1); rt.LastState != "Paused" || !rt.NextDue.IsZero() {
		t.Errorf("disabled runtime = %+v, want Paused with no next-due", rt)
	}
}

func TestSetGlobalPauseUpdatesRuntimesAndEmits(t *testing.T) {
	svc := newTempService(t, []domain.Job{
		{ID: 1, Name: "On", Schedule: "@every 1m", Command: "echo", Enabled: true},
		{ID: 2, Name: "Off", Schedule: "@every 1m", Command: "echo", Enabled: false},
	})
	rec := &recorder{}
	svc.Subscribe(rec)

	if err := svc.SetGlobalPause(true); err != nil {
		t.Fatalf("SetGlobalPause: %v", err)
	}
	if rt := svc.Runtime(1); rt.NextRun != "Scheduler paused" {
		t.Errorf("enabled job next-run = %q, want %q", rt.NextRun, "Scheduler paused")
	}
	if rt := svc.Runtime(2); rt.NextRun != "Paused" {
		t.Errorf("disabled job next-run = %q, want %q", rt.NextRun, "Paused")
	}
	var sawState bool
	for _, e := range rec.events {
		if ss, ok := e.(SchedulerStateChanged); ok && ss.Paused {
			sawState = true
		}
	}
	if !sawState {
		t.Error("expected a SchedulerStateChanged{Paused:true} event")
	}

	// Resuming recomputes a real next run for the enabled job.
	if err := svc.SetGlobalPause(false); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if rt := svc.Runtime(1); rt.NextDue.IsZero() {
		t.Errorf("resumed enabled job should have a next-due, got %+v", rt)
	}
}

func TestRunNowUsesRunnerAndRecords(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}})

	done := make(chan domain.RunRecord, 1)
	svc.runJob = func(_ context.Context, job *domain.Job, trigger string, _ string) domain.RunRecord {
		if trigger != "Manual" {
			t.Errorf("trigger = %q, want Manual", trigger)
		}
		return domain.RunRecord{Time: "2026-06-19 12:00:00", JobID: job.ID, JobName: job.Name, State: "Success", Output: "ok"}
	}
	svc.Subscribe(ObserverFunc(func(e Event) {
		if rr, ok := e.(RunRecorded); ok && rr.Record.JobID == 1 {
			select {
			case done <- rr.Record:
			default:
			}
		}
	}))

	if err := svc.RunNow(1); err != nil {
		t.Fatalf("RunNow: %v", err)
	}

	select {
	case record := <-done:
		if record.State != "Success" {
			t.Errorf("recorded state = %q, want Success", record.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for run to be recorded")
	}

	if rt := svc.Runtime(1); rt.LastState != "Success" || rt.Output != "ok" {
		t.Errorf("runtime after run = %+v", rt)
	}
}

func TestRunNowNotFound(t *testing.T) {
	svc := newTempService(t, nil)
	if err := svc.RunNow(99); err == nil {
		t.Error("expected not-found error for unknown job")
	}
}

func TestRunNowRefusedWhileAlreadyRunning(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}})

	// Park the job in the "Running" state so a second RunNow must refuse: the
	// runner signals once it has started and then blocks until released.
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var calls int32
	svc.runJob = func(_ context.Context, job *domain.Job, _ string, _ string) domain.RunRecord {
		atomic.AddInt32(&calls, 1)
		entered <- struct{}{}
		<-release
		return domain.RunRecord{Time: "2026-06-19 12:00:00", JobID: job.ID, JobName: job.Name, State: "Success"}
	}
	done := make(chan struct{}, 1)
	svc.Subscribe(ObserverFunc(func(e Event) {
		if rr, ok := e.(RunRecorded); ok && rr.Record.State == "Success" {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	}))

	if err := svc.RunNow(1); err != nil {
		t.Fatalf("first RunNow: %v", err)
	}
	<-entered // the run is now in-flight and blocked

	if err := svc.RunNow(1); err == nil {
		t.Error("expected RunNow to be refused while already running")
	}
	close(release)

	// Wait for the in-flight run to finish before returning so its background
	// writes complete before t.TempDir cleanup removes the directory.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the in-flight run to complete")
	}

	// Only the first run should ever have reached the runner.
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("runner called %d times, want 1", got)
	}
}

func TestRunNowRefusedWhilePaused(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}})
	var ran bool
	svc.runJob = func(context.Context, *domain.Job, string, string) domain.RunRecord {
		ran = true
		return domain.RunRecord{}
	}
	if err := svc.SetGlobalPause(true); err != nil {
		t.Fatalf("SetGlobalPause: %v", err)
	}
	if err := svc.RunNow(1); err == nil {
		t.Error("expected RunNow to be refused while paused")
	}
	if ran {
		t.Error("runner must not be invoked while paused")
	}
}

func TestRunDueStartsDueJob(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}})

	done := make(chan domain.RunRecord, 1)
	svc.runJob = func(_ context.Context, job *domain.Job, trigger string, _ string) domain.RunRecord {
		if trigger != "Schedule" {
			t.Errorf("trigger = %q, want Schedule", trigger)
		}
		return domain.RunRecord{Time: "2026-06-19 12:00:00", JobID: job.ID, JobName: job.Name, State: "Success", Output: "ok"}
	}
	svc.Subscribe(ObserverFunc(func(e Event) {
		if rr, ok := e.(RunRecorded); ok && rr.Record.JobID == 1 && rr.Record.State == "Success" {
			select {
			case done <- rr.Record:
			default:
			}
		}
	}))

	// The job's next-due was primed ~1m ahead at construction; tick well past it.
	svc.RunDue(time.Now().Add(2 * time.Minute))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunDue did not start the due job")
	}
	if rt := svc.Runtime(1); rt.LastState != "Success" || rt.Output != "ok" {
		t.Errorf("runtime after scheduled run = %+v", rt)
	}
}

func TestRunDueSkipsJobNotYetDue(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}})
	var ran int32
	svc.runJob = func(context.Context, *domain.Job, string, string) domain.RunRecord {
		atomic.AddInt32(&ran, 1)
		return domain.RunRecord{}
	}

	// Next-due is ~1m out, so nothing is due "now".
	svc.RunDue(time.Now())
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&ran) != 0 {
		t.Error("RunDue ran a job before it was due")
	}
}

// TestRunDueSkipsJobInRunningState verifies that RunDue will not start a second
// concurrent instance of a job that is already in "Running" state — even if the
// job's NextDue is in the past. This guards against the window between
// executeRun completing and refreshNextRunLocked setting a new NextDue.
func TestRunDueSkipsJobInRunningState(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}})

	var calls int32
	svc.runJob = func(context.Context, *domain.Job, string, string) domain.RunRecord {
		atomic.AddInt32(&calls, 1)
		return domain.RunRecord{State: "Success"}
	}

	// Force the job into "Running" with a past NextDue, simulating an in-flight
	// run. We set NextDue to a past time so the due check would otherwise pass.
	svc.mu.Lock()
	rt := svc.runtimes[1]
	rt.LastState = "Running"
	rt.NextDue = time.Now().Add(-time.Minute)
	svc.mu.Unlock()

	svc.RunDue(time.Now().Add(2 * time.Minute))
	time.Sleep(50 * time.Millisecond)

	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Errorf("RunDue called runner %d time(s) for a job in Running state, want 0", got)
	}
}

func TestRunDueDoesNothingWhilePaused(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}})
	var ran int32
	svc.runJob = func(context.Context, *domain.Job, string, string) domain.RunRecord {
		atomic.AddInt32(&ran, 1)
		return domain.RunRecord{}
	}
	if err := svc.SetGlobalPause(true); err != nil {
		t.Fatalf("SetGlobalPause: %v", err)
	}

	svc.RunDue(time.Now().Add(2 * time.Minute))
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&ran) != 0 {
		t.Error("RunDue ran a job while globally paused")
	}
}

// appFakeClock is a scheduler.Clock whose tick and "now" the test controls, used
// to verify Start wires the loop to RunDue without the wall clock.
type appFakeClock struct {
	ticks chan time.Time
	now   time.Time
}

func (c *appFakeClock) Now() time.Time          { return c.now }
func (c *appFakeClock) Ticks() <-chan time.Time { return c.ticks }
func (c *appFakeClock) Stop()                   {}

func TestStartDrivesRunDueOnTick(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}})

	done := make(chan struct{}, 1)
	svc.runJob = func(context.Context, *domain.Job, string, string) domain.RunRecord {
		select {
		case done <- struct{}{}:
		default:
		}
		return domain.RunRecord{State: "Success"}
	}

	clock := &appFakeClock{ticks: make(chan time.Time, 1), now: time.Now().Add(2 * time.Minute)}
	svc.StartWith(clock)
	defer svc.Stop()

	clock.ticks <- clock.now
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not drive a run from a clock tick")
	}
}

func TestUpdateSettingsPersistsAndValidates(t *testing.T) {
	svc := newTempService(t, nil)

	bad := svc.store.Config
	bad.MaxLogFiles = 0
	if err := svc.UpdateSettings(bad); err == nil {
		t.Error("expected validation error for non-positive max log files")
	}

	good := svc.store.Config
	good.NotifyOnFailure = false
	good.MaxLogAgeDays = 7
	if err := svc.UpdateSettings(good); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if svc.Store().Config.MaxLogAgeDays != 7 || svc.Store().Config.NotifyOnFailure {
		t.Errorf("config not applied: %+v", svc.Store().Config)
	}
}

func TestUpdateSettingsRejectsInvalidConfigs(t *testing.T) {
	svc := newTempService(t, nil)
	base := svc.store.Config

	tests := []struct {
		name   string
		mutate func(c *domain.Config)
	}{
		{"missing jobs dir", func(c *domain.Config) { c.JobsDir = "  " }},
		{"missing logs dir", func(c *domain.Config) { c.LogsDir = "" }},
		{"non-positive max files", func(c *domain.Config) { c.MaxLogFiles = 0 }},
		{"non-positive max age", func(c *domain.Config) { c.MaxLogAgeDays = -1 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.mutate(&cfg)
			if err := svc.UpdateSettings(cfg); err == nil {
				t.Errorf("expected validation error for %s", tc.name)
			}
		})
	}
}

func TestPrependLogCapsActivityList(t *testing.T) {
	runtime := &domain.JobRuntime{}
	for i := 0; i < maxJobLogs+10; i++ {
		prependLog(runtime, domain.RunRecord{Detail: "r"})
	}
	if len(runtime.Logs) != maxJobLogs {
		t.Errorf("activity list len = %d, want capped at %d", len(runtime.Logs), maxJobLogs)
	}
}
