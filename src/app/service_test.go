package app

import (
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"gitea.mixdep.ru/mix/gosentry/src/storage"
)

func newTestService(jobs []domain.Job) *Service {
	return NewService(&storage.Store{}, jobs)
}

func TestNewServiceBuildsRuntimePerJob(t *testing.T) {
	jobs := []domain.Job{
		{ID: 1, Name: "Enabled", Enabled: true},
		{ID: 2, Name: "Disabled", Enabled: false},
	}
	svc := newTestService(jobs)

	if got := svc.Runtime(1); got == nil {
		t.Fatal("expected runtime for enabled job 1")
	} else if got.LastState != "Ready" {
		t.Errorf("enabled job runtime state = %q, want %q", got.LastState, "Ready")
	}
	if got := svc.Runtime(2); got == nil {
		t.Fatal("expected runtime for disabled job 2")
	} else if got.LastState != "Paused" {
		t.Errorf("disabled job runtime state = %q, want %q", got.LastState, "Paused")
	}
	if got := svc.Runtime(99); got != nil {
		t.Errorf("expected nil runtime for unknown job, got %+v", got)
	}
}

func TestJobsReturnsCopy(t *testing.T) {
	jobs := []domain.Job{{ID: 1, Name: "Original"}}
	svc := newTestService(jobs)

	snapshot := svc.Jobs()
	if len(snapshot) != 1 {
		t.Fatalf("Jobs() len = %d, want 1", len(snapshot))
	}
	// Mutating the returned slice must not affect Service-owned state.
	snapshot[0].Name = "Mutated"
	if again := svc.Jobs(); again[0].Name != "Original" {
		t.Errorf("Service state leaked through Jobs(): name = %q, want %q", again[0].Name, "Original")
	}
}

func TestStoreReturnsWiredStore(t *testing.T) {
	store := &storage.Store{}
	svc := NewService(store, nil)
	if svc.Store() != store {
		t.Error("Store() did not return the wired store")
	}
}
