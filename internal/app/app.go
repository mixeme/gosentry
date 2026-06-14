package app

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const appID = "io.github.pysentry.desktop"

type job struct {
	Name      string
	Schedule  string
	Command   string
	Enabled   bool
	LastRun   string
	NextRun   string
	LastState string
}

type event struct {
	Time    string
	JobName string
	State   string
	Detail  string
}

func Run() {
	a := app.NewWithID(appID)
	a.SetIcon(theme.ComputerIcon())

	w := a.NewWindow("PySentry")
	w.Resize(fyne.NewSize(1120, 720))
	w.SetContent(newMainView(w))
	w.ShowAndRun()
}

func newMainView(w fyne.Window) fyne.CanvasObject {
	jobs := []job{
		{
			Name:      "Nightly backup",
			Schedule:  "0 2 * * *",
			Command:   "python scripts/backup.py",
			Enabled:   true,
			LastRun:   "Today 02:00",
			NextRun:   "Tomorrow 02:00",
			LastState: "OK",
		},
		{
			Name:      "Health check",
			Schedule:  "*/15 * * * *",
			Command:   "curl -fsS https://example.test/health",
			Enabled:   true,
			LastRun:   "21:00",
			NextRun:   "21:15",
			LastState: "OK",
		},
		{
			Name:      "Rotate logs",
			Schedule:  "30 1 * * 1",
			Command:   "pysentry rotate-logs",
			Enabled:   false,
			LastRun:   "Monday 01:30",
			NextRun:   "Paused",
			LastState: "Paused",
		},
	}
	events := []event{
		{Time: "21:00", JobName: "Health check", State: "OK", Detail: "Completed in 184 ms"},
		{Time: "20:45", JobName: "Health check", State: "OK", Detail: "Completed in 201 ms"},
		{Time: "02:00", JobName: "Nightly backup", State: "OK", Detail: "Completed in 42 s"},
		{Time: "Yesterday 01:30", JobName: "Rotate logs", State: "Paused", Detail: "Skipped because the job is paused"},
	}

	selected := 0
	title := widget.NewLabelWithStyle(jobs[selected].Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	schedule := widget.NewLabel(jobs[selected].Schedule)
	command := widget.NewLabel(jobs[selected].Command)
	lastRun := widget.NewLabel(jobs[selected].LastRun)
	nextRun := widget.NewLabel(jobs[selected].NextRun)
	state := widget.NewLabel(jobs[selected].LastState)
	recentEvents := widget.NewList(
		func() int {
			if len(events) < 5 {
				return len(events)
			}
			return 5
		},
		func() fyne.CanvasObject { return widget.NewLabel("event") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			item.(*widget.Label).SetText(eventText(events[id]))
		},
	)
	history := newHistoryView(&events)

	updateDetails := func(index int) {
		if index < 0 || index >= len(jobs) {
			title.SetText("No job selected")
			schedule.SetText("")
			command.SetText("")
			lastRun.SetText("")
			nextRun.SetText("")
			state.SetText("")
			return
		}
		selected = index
		current := jobs[selected]
		title.SetText(current.Name)
		schedule.SetText(current.Schedule)
		command.SetText(current.Command)
		lastRun.SetText(current.LastRun)
		nextRun.SetText(current.NextRun)
		state.SetText(current.LastState)
	}
	refresh := func() {
		updateDetails(selected)
		recentEvents.Refresh()
		history.Refresh()
	}

	list := widget.NewList(
		func() int { return len(jobs) },
		func() fyne.CanvasObject {
			name := widget.NewLabelWithStyle("Job name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			meta := widget.NewLabel("schedule")
			status := widget.NewLabel("status")
			return container.NewVBox(name, meta, status)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			row := item.(*fyne.Container)
			name := row.Objects[0].(*widget.Label)
			meta := row.Objects[1].(*widget.Label)
			status := row.Objects[2].(*widget.Label)

			current := jobs[id]
			name.SetText(current.Name)
			meta.SetText(current.Schedule + "    " + current.Command)
			status.SetText(statusText(current))
		},
	)
	list.OnSelected = updateDetails
	list.Select(selected)

	addButton := widget.NewButtonWithIcon("New job", theme.ContentAddIcon(), func() {
		showJobDialog(w, "New job", job{Enabled: true, LastRun: "Never", NextRun: "After save", LastState: "Ready"}, func(saved job) {
			jobs = append(jobs, saved)
			selected = len(jobs) - 1
			events = append([]event{newEvent(saved.Name, "Created", "Job was added")}, events...)
			list.Refresh()
			list.Select(selected)
			refresh()
		})
	})
	editButton := widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		if selected < 0 || selected >= len(jobs) {
			return
		}
		showJobDialog(w, "Edit job", jobs[selected], func(saved job) {
			jobs[selected] = saved
			events = append([]event{newEvent(saved.Name, "Updated", "Job settings changed")}, events...)
			list.Refresh()
			refresh()
		})
	})
	runButton := widget.NewButtonWithIcon("Run now", theme.MediaPlayIcon(), func() {
		if selected < 0 || selected >= len(jobs) {
			return
		}
		jobs[selected].LastRun = "Just now"
		jobs[selected].LastState = "OK"
		if jobs[selected].Enabled {
			jobs[selected].NextRun = "Waiting for scheduler"
		}
		events = append([]event{newEvent(jobs[selected].Name, "OK", "Manual run simulated")}, events...)
		list.Refresh()
		refresh()
	})
	pauseButton := widget.NewButtonWithIcon("Pause", theme.MediaPauseIcon(), func() {
		if selected < 0 || selected >= len(jobs) {
			return
		}
		current := &jobs[selected]
		current.Enabled = !current.Enabled
		if current.Enabled {
			current.LastState = "Ready"
			current.NextRun = "Waiting for scheduler"
			events = append([]event{newEvent(current.Name, "Resumed", "Job was enabled")}, events...)
		} else {
			current.LastState = "Paused"
			current.NextRun = "Paused"
			events = append([]event{newEvent(current.Name, "Paused", "Job was disabled")}, events...)
		}
		list.Refresh()
		refresh()
	})

	toolbar := container.NewHBox(addButton, editButton, runButton, pauseButton, layout.NewSpacer())
	sidebar := container.NewBorder(toolbar, nil, nil, nil, list)

	details := container.NewVBox(
		title,
		widget.NewSeparator(),
		detailRow("Schedule", schedule),
		detailRow("Command", command),
		detailRow("Last run", lastRun),
		detailRow("Next run", nextRun),
		detailRow("State", state),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Recent events", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		recentEvents,
	)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Jobs", theme.ListIcon(), container.NewHSplit(sidebar, container.NewPadded(details))),
		container.NewTabItemWithIcon("History", theme.HistoryIcon(), history),
		container.NewTabItemWithIcon("Settings", theme.SettingsIcon(), settingsView()),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	return tabs
}

func statusText(j job) string {
	if !j.Enabled {
		return "Paused"
	}
	return j.LastState
}

func newEvent(jobName string, state string, detail string) event {
	return event{
		Time:    time.Now().Format("15:04:05"),
		JobName: jobName,
		State:   state,
		Detail:  detail,
	}
}

func eventText(e event) string {
	return fmt.Sprintf("%s  %s  %s  %s", e.Time, e.JobName, e.State, e.Detail)
}

func detailRow(label string, value fyne.CanvasObject) fyne.CanvasObject {
	caption := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	caption.Wrapping = fyne.TextTruncate
	return container.NewGridWithColumns(2, caption, value)
}

func showJobDialog(w fyne.Window, title string, current job, onSave func(job)) {
	name := widget.NewEntry()
	name.SetPlaceHolder("Nightly backup")
	name.SetText(current.Name)
	schedule := widget.NewEntry()
	schedule.SetPlaceHolder("0 2 * * *")
	schedule.SetText(current.Schedule)
	command := widget.NewEntry()
	command.SetPlaceHolder("python scripts/backup.py")
	command.SetText(current.Command)
	enabled := widget.NewCheck("Enabled", nil)
	enabled.SetChecked(current.Enabled)

	form := dialog.NewForm(
		title,
		"Save",
		"Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Name", name),
			widget.NewFormItem("Schedule", schedule),
			widget.NewFormItem("Command", command),
			widget.NewFormItem("", enabled),
		},
		func(saved bool) {
			if !saved {
				return
			}
			if strings.TrimSpace(name.Text) == "" || strings.TrimSpace(schedule.Text) == "" || strings.TrimSpace(command.Text) == "" {
				dialog.ShowError(fmt.Errorf("name, schedule, and command are required"), w)
				return
			}
			current.Name = strings.TrimSpace(name.Text)
			current.Schedule = strings.TrimSpace(schedule.Text)
			current.Command = strings.TrimSpace(command.Text)
			current.Enabled = enabled.Checked
			if current.LastRun == "" {
				current.LastRun = "Never"
			}
			if current.Enabled {
				current.NextRun = "Waiting for scheduler"
				if current.LastState == "" || current.LastState == "Paused" {
					current.LastState = "Ready"
				}
			} else {
				current.NextRun = "Paused"
				current.LastState = "Paused"
			}
			onSave(current)
		},
		w,
	)
	form.Resize(fyne.NewSize(560, 280))
	form.Show()
}

func newHistoryView(events *[]event) *fyne.Container {
	list := widget.NewList(
		func() int { return len(*events) },
		func() fyne.CanvasObject { return widget.NewLabel("event") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			item.(*widget.Label).SetText(eventText((*events)[id]))
		},
	)
	return container.NewPadded(list)
}

func settingsView() fyne.CanvasObject {
	runOnStartup := widget.NewCheck("Start PySentry when I sign in", nil)
	minimizeToTray := widget.NewCheck("Keep running in the system tray", nil)
	notifications := widget.NewCheck("Show desktop notifications for failed jobs", nil)
	notifications.SetChecked(true)

	return container.NewPadded(container.NewVBox(
		widget.NewLabelWithStyle("Application", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		runOnStartup,
		minimizeToTray,
		notifications,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Scheduler", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("The scheduler service, job storage, and cron parser come next."),
	))
}
