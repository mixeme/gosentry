package ui

import (
	"strconv"
	"time"

	"gitea.mixdep.ru/mix/gosentry/assets"
	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

const runRecordTimeLayout = "2006-01-02 15:04:05"

// The UI package aliases domain types to keep widget callbacks short. The actual
// durable model still lives in src/domain, so UI code does not define a second
// copy of the scheduler data.
type job = domain.Job
type event = domain.RunRecord

func newMainView(w fyne.Window, svc *app.Service, tray *trayState) (fyne.CanvasObject, func(time.Duration, bool)) {
	// History is session-only: jobs.json never persists JobRuntime.Logs (see
	// domain.JobRuntime), so there is nothing to seed the History tab with at
	// startup. It starts empty and fills as events arrive.
	events := newHistoryLog(nil)

	jobsPanel, refreshJobsView := newJobsView(w, svc)

	history, refreshHistory := newHistoryView(events)
	recordStartup := func(duration time.Duration, windowShown bool) {
		// Startup is recorded as an in-memory History event instead of being
		// persisted into jobs.json. It is session diagnostics, not durable job
		// state, and keeping it ephemeral avoids polluting the human-editable JSON
		// file with process-lifetime bookkeeping.
		detail := "Window shown in " + duration.Round(time.Millisecond).String()
		if !windowShown {
			detail = "Started in tray in " + duration.Round(time.Millisecond).String()
		}
		events.add(newEvent(0, "Application", "Started", detail))
		refreshHistory()
	}

	refresh := func() {
		refreshJobsView()
		refreshHistory()
	}

	// The Service announces every change through events. This single listener is
	// where the UI reacts: it appends run/activity records to History and redraws.
	// Events fire from two contexts — UI button handlers call into the Service
	// synchronously (main goroutine), while scheduled and manual run completions
	// emit from the run goroutine. fyne.Do marshals all of this widget work onto
	// the main thread in both cases, so the engine never mutates Fyne state off
	// the UI thread. This is the sole place events touch widgets. (Resolves #4.)
	svc.Subscribe(app.ObserverFunc(func(ev app.Event) {
		fyne.Do(func() {
			// A type switch does not get compiler-enforced exhaustiveness (see
			// app.Event's doc comment) — JobChanged and SchedulerStateChanged
			// intentionally fall through to the unconditional refresh() below
			// without their own case, since a broad state re-read is all they need.
			switch e := ev.(type) {
			case app.RunRecorded:
				events.add(e.Record)
				if e.Record.State == "Failed" &&
					(e.Record.Trigger == "Manual" || e.Record.Trigger == "Schedule") &&
					svc.ShouldNotifyOnFailure() {
					timing := notificationTiming{
						JobName:   e.Record.JobName,
						EmittedAt: time.Now(),
					}
					if finished, err := time.ParseInLocation(runRecordTimeLayout, e.Record.Time, time.Local); err == nil {
						timing.RunFinished = finished
					}
					fyne.Do(func() {
						timing.UIQueuedAt = time.Now()
						fyne.CurrentApp().SendNotification(&fyne.Notification{
							Title:   "GoSentry: Job Failed",
							Content: e.Record.JobName + ": " + e.Record.Detail,
						})
						timing.AfterSendAt = time.Now()
						if err := appendNotificationTimingLog(svc.Paths().LogsDir, timing); err != nil {
							fyne.LogError("Failed to write notification timing log", err)
						}
					})
				}
			case app.ErrorOccurred:
				events.add(newEvent(0, "Service", "Error", e.Err.Error()))
			case app.JobsLoaded:
				// Selecting an existing jobs file replaces the job list without a
				// prompt, so History carries the receipt: how many jobs, from where.
				detail := strconv.Itoa(e.Count) + " jobs from " + e.Path
				events.add(newEvent(0, "Service", "Jobs loaded", detail))
			}
			refresh()
		})
	}))
	// Installed after Subscribe so a failure reaches History through
	// ErrorOccurred instead of being emitted to no listener.
	svc.InstallDesktopIcon(appID, assets.IconBytes())
	svc.Start()

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Jobs", theme.ListIcon(), jobsPanel),
		container.NewTabItemWithIcon("History", theme.HistoryIcon(), history),
		container.NewTabItemWithIcon("Settings", theme.SettingsIcon(), settingsView(w, svc, tray)),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	return tabs, recordStartup
}
