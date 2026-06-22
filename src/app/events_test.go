package app

import (
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

func TestEmitDeliversToAllObserversInOrder(t *testing.T) {
	svc := newTestService(nil)

	var first, second []Event
	svc.Subscribe(ObserverFunc(func(e Event) { first = append(first, e) }))
	svc.Subscribe(ObserverFunc(func(e Event) { second = append(second, e) }))

	svc.emit(JobChanged{JobID: 7})
	svc.emit(RunRecorded{Record: domain.RunRecord{JobID: 7, State: "Success"}})
	svc.emit(SchedulerStateChanged{Paused: true})

	for name, got := range map[string][]Event{"first": first, "second": second} {
		if len(got) != 3 {
			t.Fatalf("%s observer got %d events, want 3", name, len(got))
		}
		if jc, ok := got[0].(JobChanged); !ok || jc.JobID != 7 {
			t.Errorf("%s event[0] = %#v, want JobChanged{JobID:7}", name, got[0])
		}
		if rr, ok := got[1].(RunRecorded); !ok || rr.Record.State != "Success" {
			t.Errorf("%s event[1] = %#v, want RunRecorded Success", name, got[1])
		}
		if ss, ok := got[2].(SchedulerStateChanged); !ok || !ss.Paused {
			t.Errorf("%s event[2] = %#v, want SchedulerStateChanged{Paused:true}", name, got[2])
		}
	}
}

func TestEmitWithNoObserversIsNoop(t *testing.T) {
	svc := newTestService(nil)
	// Must not panic with an empty observer list.
	svc.emit(JobChanged{})
}

// Observers may read Service state from within OnEvent without deadlocking,
// because emit is called outside the state lock.
func TestObserverCanReadServiceState(t *testing.T) {
	jobs := []domain.Job{{ID: 1, Name: "Job", Enabled: true}}
	svc := newTestService(jobs)

	var sawName string
	svc.Subscribe(ObserverFunc(func(Event) {
		if snapshot := svc.Jobs(); len(snapshot) == 1 {
			sawName = snapshot[0].Name
		}
	}))

	svc.emit(JobChanged{JobID: 1})
	if sawName != "Job" {
		t.Errorf("observer read name = %q, want %q", sawName, "Job")
	}
}
