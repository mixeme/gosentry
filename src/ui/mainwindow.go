package ui

import (
	"time"

	"gitea.mixdep.ru/mix/gosentry/assets"
	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The UI package aliases domain types to keep widget callbacks short. The actual
// durable model still lives in src/domain, so UI code does not define a second
// copy of the scheduler data.
type job = domain.Job
type event = domain.RunRecord

func newMainView(w fyne.Window) (fyne.CanvasObject, func(time.Duration, bool)) {
	svc, err := app.Open()
	if err != nil {
		return container.NewPadded(widget.NewLabel("Failed to load GoSentry configuration: " + err.Error())), func(time.Duration, bool) {}
	}
	svc.InstallDesktopIcon(appID, assets.IconBytes())

	// Build the initial event history from the current runtime state. Jobs and
	// runtimes are read here only for this one-time initialization; the jobs view
	// owns all subsequent state via its own syncFromService closure.
	initialJobs := svc.Jobs()
	initialRuntimes := make(map[int]*domain.JobRuntime, len(initialJobs))
	for _, j := range initialJobs {
		if rt := svc.Runtime(j.ID); rt != nil {
			initialRuntimes[j.ID] = rt
		}
	}
	events := collectActivity(initialJobs, initialRuntimes)

	jobsPanel, refreshJobsView := newJobsView(w, svc)

	history := newHistoryView(&events)
	recordStartup := func(duration time.Duration, windowShown bool) {
		// Startup is recorded as an in-memory History event instead of being
		// persisted into jobs.yaml. It is session diagnostics, not durable job
		// state, and keeping it ephemeral avoids polluting the human-editable YAML
		// file with process-lifetime bookkeeping.
		detail := "Window shown in " + duration.Round(time.Millisecond).String()
		if !windowShown {
			detail = "Started in tray in " + duration.Round(time.Millisecond).String()
		}
		events = append(events, newEvent(0, "Application", "Started", detail))
		history.Refresh()
	}

	refresh := func() {
		refreshJobsView()
		history.Refresh()
	}

	// The Service announces every change through events. This single listener is
	// where the UI reacts: it appends run/activity records to History and redraws.
	// Events fire from two contexts — UI button handlers call into the Service
	// synchronously (main goroutine), while scheduled and manual run completions
	// emit from the run goroutine. fyne.Do marshals all of this widget work onto
	// the main thread in both cases, so the engine never mutates Fyne state off
	// the UI thread. This is the sole place events touch widgets. (Resolves #4.)
	svc.Subscribe(app.ObserverFunc(func(ev app.Event) {
		recorded, isRecorded := ev.(app.RunRecorded)
		errOccurred, isError := ev.(app.ErrorOccurred)
		fyne.Do(func() {
			if isRecorded {
				events = append(events, recorded.Record)
			}
			if isError {
				events = append(events, newEvent(0, "Service", "Error", errOccurred.Err.Error()))
			}
			refresh()
		})
	}))
	svc.Start()

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Jobs", theme.ListIcon(), jobsPanel),
		container.NewTabItemWithIcon("History", theme.HistoryIcon(), history),
		container.NewTabItemWithIcon("Settings", theme.SettingsIcon(), settingsView(w, svc)),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	return tabs, recordStartup
}
