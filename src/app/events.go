package app

import "gitea.mixdep.ru/mix/gosentry/src/domain"

// Event is something the Service did to its state that observers may want to
// react to. It is a sealed interface: the concrete types in this file are the
// only implementations (enforced by the unexported isEvent marker), so a UI
// listener can exhaustively type-switch over them and the compiler will flag a
// new event type that a switch forgot to handle.
//
// Events replace the old single onChange callback. Instead of the scheduler
// reaching into the GUI, the Service emits typed events and the UI subscribes —
// the UI's listener becomes the one place that touches widgets.
type Event interface {
	isEvent()
}

// JobChanged signals that a job's durable config or transient runtime changed:
// created, edited, deleted, enabled/disabled, or a status transition such as a
// run starting. Observers should re-read the affected state through the Service
// (Jobs/Runtime) rather than expect a payload snapshot — that keeps the event
// small and avoids handing out stale copies.
//
// JobID identifies the affected job. A zero JobID means a broad change (for
// example a delete, or a global pause that touched every job) and observers
// should refresh their whole view.
type JobChanged struct {
	JobID int
}

// RunRecorded signals that a job run finished and produced a RunRecord. It
// carries the record by value because the record is an immutable result that
// observers append to history; there is nothing for them to re-read.
type RunRecorded struct {
	Record domain.RunRecord
}

// SchedulerStateChanged signals that the global scheduler pause state flipped.
// The UI uses it to update the pause/resume control and status text.
type SchedulerStateChanged struct {
	Paused bool
}

// ErrorOccurred signals a background error that could not be returned to a
// caller — typically a failed save or cleanup after an async run. The UI
// surfaces it in the History tab so the user is not silently left with
// un-persisted state.
type ErrorOccurred struct {
	Err error
}

func (JobChanged) isEvent()            {}
func (RunRecorded) isEvent()           {}
func (SchedulerStateChanged) isEvent() {}
func (ErrorOccurred) isEvent()         {}

// Observer receives events emitted by the Service. OnEvent is the single
// reaction point; the UI implements it and marshals any widget work onto the
// main thread (fyne.Do) itself — the Service knows nothing about Fyne.
type Observer interface {
	OnEvent(Event)
}

// ObserverFunc adapts a plain function to the Observer interface, so callers can
// subscribe a closure without declaring a type.
type ObserverFunc func(Event)

// OnEvent calls the wrapped function.
func (f ObserverFunc) OnEvent(event Event) { f(event) }

// Subscribe registers an observer to receive every subsequently emitted event.
// Registration is expected during setup, before the scheduler starts, but is
// guarded so it is safe at any time.
func (s *Service) Subscribe(observer Observer) {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()
	s.observers = append(s.observers, observer)
}

// emit delivers an event to every registered observer.
//
// Single-threaded dispatch contract:
//   - emit holds dispatchMu for the whole dispatch, so observers are never
//     invoked concurrently and never overlap with each other or with Subscribe.
//     Each observer sees events one at a time, in emit order.
//   - emit must be called WITHOUT holding s.mu. The Service computes a state
//     change under mu, releases it, then emits — so an observer is free to call
//     back into read methods (Jobs/Runtime) without deadlocking on the state
//     lock.
//   - An observer must NOT call back into a Service method that emits (directly
//     or indirectly): dispatchMu is non-reentrant, so re-entrant emission would
//     deadlock. Observers react and return quickly; long or UI work is the
//     observer's own responsibility to defer (e.g. fyne.Do).
func (s *Service) emit(event Event) {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()
	for _, observer := range s.observers {
		observer.OnEvent(event)
	}
}
