package app

import (
	"context"
	"encoding/json"
	"os"
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
		Config: domain.Config{JobsFile: "jobs.json", LogsDir: "logs", MaxLogFiles: 100, MaxLogAgeDays: 30, ExecutionMode: domain.ExecutionModeParallel, OverlapPolicy: domain.OverlapPolicySkip, DefaultTimeoutSeconds: 30},
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
	if _, err := svc.CreateJob(domain.Job{Name: "A", Schedule: "@every 1m", Command: "echo", OverlapPolicy: "invalid"}); err == nil {
		t.Error("expected error for invalid overlap policy")
	}
	if _, err := svc.CreateJob(domain.Job{Name: "A", Schedule: "@every 1m", Command: "echo", TimeoutSeconds: domain.TimeoutSecondsPtr(-1)}); err == nil {
		t.Error("expected error for negative per-job timeout")
	}
	// An explicit 0 is a valid choice ("no timeout"), not a rejected one.
	if _, err := svc.CreateJob(domain.Job{Name: "Zero", Schedule: "@every 1m", Command: "echo", TimeoutSeconds: domain.TimeoutSecondsPtr(0)}); err != nil {
		t.Errorf("explicit zero per-job timeout should be accepted: %v", err)
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

// TestSetEnabledClearsPendingRuns verifies that disabling a job drops any
// "queue" overlap backlog it was carrying, so re-enabling it later does not
// replay a deferred run for an occurrence that fired before the disable.
func TestSetEnabledClearsPendingRuns(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}})
	svc.mu.Lock()
	svc.runtimes[1].PendingRuns = 2
	svc.mu.Unlock()

	if err := svc.SetEnabled(1, false); err != nil {
		t.Fatalf("SetEnabled false: %v", err)
	}
	if rt := svc.Runtime(1); rt.PendingRuns != 0 {
		t.Errorf("PendingRuns after disable = %d, want 0", rt.PendingRuns)
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
	svc.runJob = func(_ context.Context, job *domain.Job, trigger string, _ string, _ time.Duration) (domain.RunRecord, error) {
		if trigger != "Manual" {
			t.Errorf("trigger = %q, want Manual", trigger)
		}
		return domain.RunRecord{Time: "2026-06-19 12:00:00", JobID: job.ID, JobName: job.Name, State: "Success", Output: "ok"}, nil
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
	svc.runJob = func(_ context.Context, job *domain.Job, _ string, _ string, _ time.Duration) (domain.RunRecord, error) {
		atomic.AddInt32(&calls, 1)
		entered <- struct{}{}
		<-release
		return domain.RunRecord{Time: "2026-06-19 12:00:00", JobID: job.ID, JobName: job.Name, State: "Success"}, nil
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

func TestRunNowAllowedWhilePaused(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}})
	done := make(chan struct{}, 1)
	svc.runJob = func(context.Context, *domain.Job, string, string, time.Duration) (domain.RunRecord, error) {
		select {
		case done <- struct{}{}:
		default:
		}
		return domain.RunRecord{State: "Success"}, nil
	}
	if err := svc.SetGlobalPause(true); err != nil {
		t.Fatalf("SetGlobalPause: %v", err)
	}
	// Pause stops only scheduled runs; an explicit manual run is still allowed.
	if err := svc.RunNow(1); err != nil {
		t.Fatalf("RunNow should be allowed while paused: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("runner was not invoked for a manual run while paused")
	}
}

func TestRunDueStartsDueJob(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}})

	done := make(chan domain.RunRecord, 1)
	svc.runJob = func(_ context.Context, job *domain.Job, trigger string, _ string, _ time.Duration) (domain.RunRecord, error) {
		if trigger != "Schedule" {
			t.Errorf("trigger = %q, want Schedule", trigger)
		}
		return domain.RunRecord{Time: "2026-06-19 12:00:00", JobID: job.ID, JobName: job.Name, State: "Success", Output: "ok"}, nil
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
	svc.runJob = func(context.Context, *domain.Job, string, string, time.Duration) (domain.RunRecord, error) {
		atomic.AddInt32(&ran, 1)
		return domain.RunRecord{}, nil
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
	svc.runJob = func(context.Context, *domain.Job, string, string, time.Duration) (domain.RunRecord, error) {
		atomic.AddInt32(&calls, 1)
		return domain.RunRecord{State: "Success"}, nil
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
	svc.runJob = func(context.Context, *domain.Job, string, string, time.Duration) (domain.RunRecord, error) {
		atomic.AddInt32(&ran, 1)
		return domain.RunRecord{}, nil
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
	svc.runJob = func(context.Context, *domain.Job, string, string, time.Duration) (domain.RunRecord, error) {
		select {
		case done <- struct{}{}:
		default:
		}
		return domain.RunRecord{State: "Success"}, nil
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
		{"missing jobs file", func(c *domain.Config) { c.JobsFile = "  " }},
		{"jobs file without a file name", func(c *domain.Config) { c.JobsFile = "jobs" + string(filepath.Separator) }},
		{"missing logs dir", func(c *domain.Config) { c.LogsDir = "" }},
		{"non-positive max files", func(c *domain.Config) { c.MaxLogFiles = 0 }},
		{"non-positive max age", func(c *domain.Config) { c.MaxLogAgeDays = -1 }},
		{"negative default timeout", func(c *domain.Config) { c.DefaultTimeoutSeconds = -1 }},
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

func TestHasFileName(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"jobs.json", true},
		{filepath.Join("data", "team.json"), true},
		{"jobs" + string(filepath.Separator), false},
		{"data/", false},
		{".", false},
		{"..", false},
		{string(filepath.Separator), false},
	}
	for _, tc := range tests {
		if got := hasFileName(tc.path); got != tc.want {
			t.Errorf("hasFileName(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// Renaming or relocating the jobs file writes the loaded jobs to the new path,
// which is what makes the Settings change take effect without a restart.
func TestUpdateSettingsWritesJobsToTheNewFile(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "Kept", Schedule: "@every 1m", Command: "echo hi", Enabled: true}})

	config := svc.store.Config
	config.JobsFile = filepath.Join("data", "team-jobs.json")
	if err := svc.UpdateSettings(config); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	moved := filepath.Join(svc.store.Paths.AppDir, "data", "team-jobs.json")
	if svc.store.Paths.JobsPath != moved {
		t.Errorf("JobsPath: got %q, want %q", svc.store.Paths.JobsPath, moved)
	}
	data, err := os.ReadFile(moved)
	if err != nil {
		t.Fatalf("read moved jobs file: %v", err)
	}
	var file domain.JobsFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("unmarshal moved jobs file: %v", err)
	}
	if len(file.Jobs) != 1 || file.Jobs[0].Name != "Kept" {
		t.Errorf("moved jobs file: got %+v, want the single 'Kept' job", file.Jobs)
	}
}

// Pointing Settings at a jobs file that already exists must adopt that file:
// its jobs replace the loaded ones instead of being overwritten by them. This is
// the only way the user can switch between job lists, so the file's contents
// win, the job list is rebuilt around them, and History is told where they came
// from.
func TestUpdateSettingsAdoptsExistingJobsFile(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "Local", Schedule: "@every 1m", Command: "echo local", Enabled: true}})
	rec := &recorder{}
	svc.Subscribe(rec)

	shared := filepath.Join(svc.store.Paths.AppDir, "shared.json")
	existing := domain.JobsFile{Jobs: []domain.Job{
		{ID: 4, Name: "Adopted", Schedule: "@every 5m", Command: "echo adopted", Enabled: true},
		{Name: "Needs an ID", Schedule: "@every 9m", Command: "echo second", Enabled: false},
	}}
	data, err := json.Marshal(existing)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shared, data, 0o644); err != nil {
		t.Fatal(err)
	}

	config := svc.store.Config
	config.JobsFile = shared
	if err := svc.UpdateSettings(config); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	jobs := svc.Jobs()
	if len(jobs) != 2 || jobs[0].Name != "Adopted" {
		t.Fatalf("jobs after adoption: got %+v, want the two jobs from the selected file", jobs)
	}
	// The adopted jobs must be fully live, not just listed: runtime and parsed
	// schedule are rebuilt for the IDs the file brought (including the one
	// normalization had to assign).
	for _, job := range jobs {
		if svc.Runtime(job.ID) == nil {
			t.Errorf("job %d (%q) has no runtime after adoption", job.ID, job.Name)
		}
	}
	if svc.Runtime(1) != nil {
		t.Error("runtime of the replaced job should be gone")
	}

	var loaded []JobsLoaded
	for _, e := range rec.events {
		if jl, ok := e.(JobsLoaded); ok {
			loaded = append(loaded, jl)
		}
	}
	if len(loaded) != 1 || loaded[0].Path != shared || loaded[0].Count != 2 {
		t.Errorf("JobsLoaded events: got %+v, want one for %q with 2 jobs", loaded, shared)
	}
}

// A path with no file behind it is the "rename or relocate" case: the current
// jobs are written there rather than an empty list being adopted.
func TestUpdateSettingsKeepsJobsWhenTheNewFileIsMissing(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "Local", Schedule: "@every 1m", Command: "echo local", Enabled: true}})

	config := svc.store.Config
	config.JobsFile = filepath.Join("moved", "jobs.json")
	if err := svc.UpdateSettings(config); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	jobs := svc.Jobs()
	if len(jobs) != 1 || jobs[0].Name != "Local" {
		t.Fatalf("jobs after the move: got %+v, want the original job", jobs)
	}
	if _, err := os.Stat(filepath.Join(svc.store.Paths.AppDir, "moved", "jobs.json")); err != nil {
		t.Errorf("jobs should have been written to the new path: %v", err)
	}
}

// Adoption throws away every runtime, including the state of a run in flight,
// and a finishing run would then write its result onto whichever job inherited
// its ID. Refusing the switch is what keeps that from happening.
func TestUpdateSettingsRefusesJobsFileSwitchWhileRunning(t *testing.T) {
	svc := newTempService(t, []domain.Job{{ID: 1, Name: "Long", Schedule: "@every 1h", Command: "echo long", Enabled: true}})
	entered := make(chan int, 1)
	release := make(chan struct{})
	svc.runJob = func(_ context.Context, job *domain.Job, _ string, _ string, _ time.Duration) (domain.RunRecord, error) {
		entered <- job.ID
		<-release
		return domain.RunRecord{Time: "t", JobID: job.ID, JobName: job.Name, State: "Success"}, nil
	}
	done := completions(svc)

	if err := svc.RunNow(1); err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	<-entered

	config := svc.store.Config
	config.JobsFile = filepath.Join("elsewhere", "jobs.json")
	if err := svc.UpdateSettings(config); err == nil {
		t.Error("expected the jobs-file switch to be refused while a job is running")
	}
	if svc.Store().Config.JobsFile == config.JobsFile {
		t.Error("the refused switch must not have been persisted")
	}

	// A setting that does not touch the jobs file still saves during a run.
	unrelated := svc.Store().Config
	unrelated.NotifyOnFailure = !unrelated.NotifyOnFailure
	if err := svc.UpdateSettings(unrelated); err != nil {
		t.Errorf("unrelated setting should still save during a run: %v", err)
	}

	close(release)
	waitRecord(t, done)
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

func TestSetGlobalPausePersistsToConfigFile(t *testing.T) {
	svc := newTempService(t, nil)

	if err := svc.SetGlobalPause(true); err != nil {
		t.Fatalf("SetGlobalPause: %v", err)
	}

	data, err := os.ReadFile(svc.store.Paths.ConfigPath)
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}
	var cfg domain.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshalling config: %v", err)
	}
	if !cfg.Paused {
		t.Error("persisted config does not have Paused=true after SetGlobalPause(true)")
	}

	// Resuming clears the flag on disk.
	if err := svc.SetGlobalPause(false); err != nil {
		t.Fatalf("SetGlobalPause(false): %v", err)
	}
	data, err = os.ReadFile(svc.store.Paths.ConfigPath)
	if err != nil {
		t.Fatalf("reading config file after resume: %v", err)
	}
	var cfg2 domain.Config
	if err := json.Unmarshal(data, &cfg2); err != nil {
		t.Fatalf("unmarshalling config after resume: %v", err)
	}
	if cfg2.Paused {
		t.Error("persisted config still has Paused=true after SetGlobalPause(false)")
	}
}

func TestSetJobListViewPersistsToConfigFile(t *testing.T) {
	svc := newTempService(t, nil)

	readConfig := func(stage string) domain.Config {
		t.Helper()
		data, err := os.ReadFile(svc.store.Paths.ConfigPath)
		if err != nil {
			t.Fatalf("reading config file %s: %v", stage, err)
		}
		var cfg domain.Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("unmarshalling config %s: %v", stage, err)
		}
		return cfg
	}

	if err := svc.SetJobListView(domain.JobListViewCompact); err != nil {
		t.Fatalf("SetJobListView(compact): %v", err)
	}
	if got := readConfig("after compact").JobListView; got != domain.JobListViewCompact {
		t.Errorf("persisted JobListView = %q, want %q", got, domain.JobListViewCompact)
	}

	if err := svc.SetJobListView(domain.JobListViewDetailed); err != nil {
		t.Fatalf("SetJobListView(detailed): %v", err)
	}
	if got := readConfig("after detailed").JobListView; got != domain.JobListViewDetailed {
		t.Errorf("persisted JobListView = %q, want %q", got, domain.JobListViewDetailed)
	}
}

// TestSetJobListViewNormalizesUnknownValue guards the config file against
// gaining a value no reader understands: anything but "compact" is stored as
// "detailed".
func TestSetJobListViewNormalizesUnknownValue(t *testing.T) {
	svc := newTempService(t, nil)

	if err := svc.SetJobListView(domain.JobListViewCompact); err != nil {
		t.Fatalf("SetJobListView(compact): %v", err)
	}
	if err := svc.SetJobListView("tiny"); err != nil {
		t.Fatalf("SetJobListView(tiny): %v", err)
	}

	data, err := os.ReadFile(svc.store.Paths.ConfigPath)
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}
	var cfg domain.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshalling config: %v", err)
	}
	if cfg.JobListView != domain.JobListViewDetailed {
		t.Errorf("persisted JobListView = %q, want %q", cfg.JobListView, domain.JobListViewDetailed)
	}
}

func TestServiceRebuiltFromPausedStoreStartsPaused(t *testing.T) {
	jobs := []domain.Job{{ID: 1, Name: "A", Schedule: "@every 1m", Command: "echo", Enabled: true}}
	svc := newTempService(t, jobs)

	if err := svc.SetGlobalPause(true); err != nil {
		t.Fatalf("SetGlobalPause: %v", err)
	}

	// Simulate a restart: NewService reads Config.Paused from the store that
	// SetGlobalPause already updated (both in memory and on disk).
	svc2 := NewService(svc.store, svc.Jobs())

	var ran int32
	runStarted := make(chan struct{}, 1)
	svc2.runJob = func(context.Context, *domain.Job, string, string, time.Duration) (domain.RunRecord, error) {
		atomic.AddInt32(&ran, 1)
		select {
		case runStarted <- struct{}{}:
		default:
		}
		return domain.RunRecord{}, nil
	}

	// RunDue must not start any job while paused: the scheduler stays paused after
	// a restart that rebuilt the service from a paused store.
	svc2.RunDue(time.Now().Add(2 * time.Minute))
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&ran) != 0 {
		t.Error("RunDue ran a job on a service rebuilt from a paused store")
	}

	// A manual RunNow is still allowed while paused — pause only stops the
	// scheduler, not the user's explicit action.
	if err := svc2.RunNow(1); err != nil {
		t.Errorf("RunNow should be allowed while paused: %v", err)
	}
	select {
	case <-runStarted:
	case <-time.After(2 * time.Second):
		t.Error("manual run was not started on a service rebuilt from a paused store")
	}
}
