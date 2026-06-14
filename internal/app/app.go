package app

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const appID = "io.github.pysentry.desktop"
const allFolders = "All"
const noFolder = "No folder"

type job struct {
	ID        int
	Name      string
	Folder    string
	Schedule  string
	Command   string
	Enabled   bool
	LastRun   string
	NextRun   string
	LastState string
	Logs      []event
	Output    string
}

type event struct {
	Time    string
	JobID   int
	JobName string
	State   string
	Detail  string
}

func Run() {
	a := app.NewWithID(appID)
	a.SetIcon(theme.ComputerIcon())

	w := a.NewWindow("PySentry")
	configureSystemTray(a, w)
	w.Resize(fyne.NewSize(1120, 720))
	w.SetContent(newMainView(w))
	w.ShowAndRun()
}

func configureSystemTray(a fyne.App, w fyne.Window) {
	desk, ok := a.(desktop.App)
	if !ok {
		return
	}

	menu := fyne.NewMenu("PySentry",
		fyne.NewMenuItem("Show", func() {
			w.Show()
			w.RequestFocus()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			a.Quit()
		}),
	)
	desk.SetSystemTrayMenu(menu)
	w.SetCloseIntercept(func() {
		w.Hide()
	})
}

func newMainView(w fyne.Window) fyne.CanvasObject {
	jobs := []job{
		{
			ID:        1,
			Name:      "Nightly backup",
			Folder:    "Maintenance",
			Schedule:  "0 2 * * *",
			Command:   "python scripts/backup.py",
			Enabled:   true,
			LastRun:   "Today 02:00",
			NextRun:   "Tomorrow 02:00",
			LastState: "OK",
			Output:    "stdout: backup archive created\nstderr: <empty>",
			Logs: []event{
				{Time: "Today 02:00", JobID: 1, JobName: "Nightly backup", State: "OK", Detail: "Completed in 42 s"},
				{Time: "Yesterday 02:00", JobID: 1, JobName: "Nightly backup", State: "OK", Detail: "Completed in 39 s"},
			},
		},
		{
			ID:        2,
			Name:      "Health check",
			Folder:    "Monitoring",
			Schedule:  "*/15 * * * *",
			Command:   "curl -fsS https://example.test/health",
			Enabled:   true,
			LastRun:   "21:00",
			NextRun:   "21:15",
			LastState: "OK",
			Output:    "stdout: HTTP 200 OK\nstderr: <empty>",
			Logs: []event{
				{Time: "21:00", JobID: 2, JobName: "Health check", State: "OK", Detail: "Completed in 184 ms"},
				{Time: "20:45", JobID: 2, JobName: "Health check", State: "OK", Detail: "Completed in 201 ms"},
			},
		},
		{
			ID:        3,
			Name:      "Rotate logs",
			Schedule:  "30 1 * * 1",
			Command:   "pysentry rotate-logs",
			Enabled:   false,
			LastRun:   "Monday 01:30",
			NextRun:   "Paused",
			LastState: "Paused",
			Output:    "No command output captured yet.",
			Logs: []event{
				{Time: "Yesterday 01:30", JobID: 3, JobName: "Rotate logs", State: "Paused", Detail: "Skipped because the job is paused"},
			},
		},
	}
	events := []event{
		{Time: "21:00", JobID: 2, JobName: "Health check", State: "OK", Detail: "Completed in 184 ms"},
		{Time: "20:45", JobID: 2, JobName: "Health check", State: "OK", Detail: "Completed in 201 ms"},
		{Time: "02:00", JobID: 1, JobName: "Nightly backup", State: "OK", Detail: "Completed in 42 s"},
		{Time: "Yesterday 01:30", JobID: 3, JobName: "Rotate logs", State: "Paused", Detail: "Skipped because the job is paused"},
	}

	nextJobID := 4
	selected := 0
	selectedFolder := allFolders
	schedulerPaused := false
	filteredJobs := filteredJobIndexes(jobs, selectedFolder)
	title := widget.NewLabelWithStyle(jobs[selected].Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	folder := widget.NewLabel(jobs[selected].Folder)
	schedule := widget.NewLabel(jobs[selected].Schedule)
	command := widget.NewLabel(jobs[selected].Command)
	lastRun := widget.NewLabel(jobs[selected].LastRun)
	nextRun := widget.NewLabel(jobs[selected].NextRun)
	state := widget.NewLabel(jobs[selected].LastState)
	schedulerState := widget.NewLabel("Scheduler running")
	commandOutput := widget.NewMultiLineEntry()
	commandOutput.SetText(jobs[selected].Output)
	commandOutput.Disable()
	history := newHistoryView(&events)
	jobLogs := widget.NewList(
		func() int {
			if selected < 0 || selected >= len(jobs) {
				return 0
			}
			return len(jobs[selected].Logs)
		},
		func() fyne.CanvasObject { return widget.NewLabel("log") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			item.(*widget.Label).SetText(eventText(jobs[selected].Logs[id]))
		},
	)

	updateDetails := func(index int) {
		if index < 0 || index >= len(jobs) {
			title.SetText("No job selected")
			folder.SetText("")
			schedule.SetText("")
			command.SetText("")
			lastRun.SetText("")
			nextRun.SetText("")
			state.SetText("")
			commandOutput.SetText("")
			return
		}
		selected = index
		current := jobs[selected]
		title.SetText(current.Name)
		folder.SetText(displayFolder(current.Folder))
		schedule.SetText(current.Schedule)
		command.SetText(current.Command)
		lastRun.SetText(current.LastRun)
		nextRun.SetText(current.NextRun)
		state.SetText(current.LastState)
		commandOutput.SetText(current.Output)
	}
	refresh := func() {
		filteredJobs = filteredJobIndexes(jobs, selectedFolder)
		updateDetails(selected)
		jobLogs.Refresh()
		history.Refresh()
	}

	list := widget.NewList(
		func() int { return len(filteredJobs) },
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

			current := jobs[filteredJobs[id]]
			name.SetText(current.Name)
			meta.SetText(displayFolder(current.Folder) + "    " + current.Schedule + "    " + current.Command)
			status.SetText(statusText(current))
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(filteredJobs) {
			updateDetails(-1)
			return
		}
		updateDetails(filteredJobs[id])
	}
	list.Select(selected)

	folderSelect := widget.NewSelect(folderOptions(jobs), func(value string) {
		if value == "" {
			return
		}
		selectedFolder = value
		filteredJobs = filteredJobIndexes(jobs, selectedFolder)
		list.Refresh()
		if len(filteredJobs) == 0 {
			selected = -1
			updateDetails(-1)
			return
		}
		selected = filteredJobs[0]
		list.Select(0)
		refresh()
	})
	folderSelect.SetSelected(selectedFolder)

	addButton := widget.NewButtonWithIcon("New job", theme.ContentAddIcon(), func() {
		showJobDialog(w, "New job", job{Enabled: true, LastRun: "Never", NextRun: "After save", LastState: "Ready"}, func(saved job) {
			saved.ID = nextJobID
			nextJobID++
			jobs = append(jobs, saved)
			selected = len(jobs) - 1
			created := newEvent(saved.ID, saved.Name, "Created", "Job was added")
			jobs[selected].Logs = append([]event{created}, jobs[selected].Logs...)
			events = append([]event{created}, events...)
			folderSelect.Options = folderOptions(jobs)
			folderSelect.Refresh()
			targetFolder := filterValue(saved.Folder)
			if selectedFolder != allFolders && selectedFolder != targetFolder {
				selectedFolder = targetFolder
				folderSelect.SetSelected(targetFolder)
			}
			filteredJobs = filteredJobIndexes(jobs, selectedFolder)
			list.Refresh()
			list.Select(displayIndex(filteredJobs, selected))
			refresh()
		})
	})
	editButton := widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		if selected < 0 || selected >= len(jobs) {
			return
		}
		showJobDialog(w, "Edit job", jobs[selected], func(saved job) {
			saved.ID = jobs[selected].ID
			saved.Logs = jobs[selected].Logs
			saved.Output = jobs[selected].Output
			jobs[selected] = saved
			updated := newEvent(saved.ID, saved.Name, "Updated", "Job settings changed")
			jobs[selected].Logs = append([]event{updated}, jobs[selected].Logs...)
			events = append([]event{updated}, events...)
			folderSelect.Options = folderOptions(jobs)
			folderSelect.Refresh()
			list.Refresh()
			refresh()
		})
	})
	runButton := widget.NewButtonWithIcon("Run now", theme.MediaPlayIcon(), func() {
		if selected < 0 || selected >= len(jobs) {
			return
		}
		if schedulerPaused {
			dialog.ShowInformation("Scheduler paused", "Global pause is active. Resume the scheduler before running jobs.", w)
			return
		}
		jobs[selected].LastRun = "Just now"
		jobs[selected].LastState = "OK"
		jobs[selected].Output = "stdout: manual run simulated\nstderr: <empty>"
		if jobs[selected].Enabled {
			jobs[selected].NextRun = "Waiting for scheduler"
		}
		ran := newEvent(jobs[selected].ID, jobs[selected].Name, "OK", "Manual run simulated")
		jobs[selected].Logs = append([]event{ran}, jobs[selected].Logs...)
		events = append([]event{ran}, events...)
		list.Refresh()
		refresh()
	})
	stopAllButton := widget.NewButtonWithIcon("Pause all", theme.MediaStopIcon(), nil)
	stopAllButton.OnTapped = func() {
		schedulerPaused = !schedulerPaused
		if schedulerPaused {
			schedulerState.SetText("Scheduler paused")
			stopAllButton.SetText("Resume all")
			stopAllButton.SetIcon(theme.MediaPlayIcon())
			for index := range jobs {
				if jobs[index].Enabled {
					jobs[index].NextRun = "Scheduler paused"
				}
			}
			events = append([]event{newEvent(0, "Scheduler", "Paused", "All job execution paused")}, events...)
		} else {
			schedulerState.SetText("Scheduler running")
			stopAllButton.SetText("Pause all")
			stopAllButton.SetIcon(theme.MediaStopIcon())
			for index := range jobs {
				if jobs[index].Enabled && jobs[index].NextRun == "Scheduler paused" {
					jobs[index].NextRun = "Waiting for scheduler"
				}
			}
			events = append([]event{newEvent(0, "Scheduler", "Resumed", "All job execution resumed")}, events...)
		}
		list.Refresh()
		refresh()
	}
	pauseButton := widget.NewButtonWithIcon("Pause", theme.MediaPauseIcon(), func() {
		if selected < 0 || selected >= len(jobs) {
			return
		}
		current := &jobs[selected]
		current.Enabled = !current.Enabled
		if current.Enabled {
			current.LastState = "Ready"
			current.NextRun = "Waiting for scheduler"
			resumed := newEvent(current.ID, current.Name, "Resumed", "Job was enabled")
			current.Logs = append([]event{resumed}, current.Logs...)
			events = append([]event{resumed}, events...)
		} else {
			current.LastState = "Paused"
			current.NextRun = "Paused"
			paused := newEvent(current.ID, current.Name, "Paused", "Job was disabled")
			current.Logs = append([]event{paused}, current.Logs...)
			events = append([]event{paused}, events...)
		}
		list.Refresh()
		refresh()
	})
	deleteButton := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		if selected < 0 || selected >= len(jobs) {
			return
		}
		deleted := jobs[selected]
		dialog.ShowConfirm("Delete job", fmt.Sprintf("Delete %q?", deleted.Name), func(confirm bool) {
			if !confirm {
				return
			}
			jobs = append(jobs[:selected], jobs[selected+1:]...)
			folderSelect.Options = folderOptions(jobs)
			folderSelect.Refresh()
			filteredJobs = filteredJobIndexes(jobs, selectedFolder)
			if len(filteredJobs) == 0 && selectedFolder != allFolders {
				selectedFolder = allFolders
				folderSelect.SetSelected(allFolders)
				filteredJobs = filteredJobIndexes(jobs, selectedFolder)
			}
			if len(filteredJobs) == 0 {
				selected = -1
			} else {
				selected = filteredJobs[0]
			}
			events = append([]event{newEvent(deleted.ID, deleted.Name, "Deleted", "Job was removed")}, events...)
			list.Refresh()
			if selected >= 0 {
				list.Select(displayIndex(filteredJobs, selected))
			}
			refresh()
		}, w)
	})

	toolbar := container.NewHBox(addButton, editButton, runButton, pauseButton, deleteButton, layout.NewSpacer())
	globalControls := container.NewHBox(stopAllButton, schedulerState, layout.NewSpacer())
	sidebarHeader := container.NewVBox(globalControls, widget.NewSeparator(), widget.NewLabelWithStyle("Folder", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), folderSelect, toolbar)
	sidebar := container.NewBorder(sidebarHeader, nil, nil, nil, list)

	details := container.NewVBox(
		title,
		widget.NewSeparator(),
		detailRow("Folder", folder),
		detailRow("Schedule", schedule),
		detailRow("Command", command),
		detailRow("Last run", lastRun),
		detailRow("Next run", nextRun),
		detailRow("State", state),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Command output", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		commandOutput,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Selected job activity", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		jobLogs,
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

func newEvent(jobID int, jobName string, state string, detail string) event {
	return event{
		Time:    time.Now().Format("15:04:05"),
		JobID:   jobID,
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

func filteredJobIndexes(jobs []job, folder string) []int {
	indexes := make([]int, 0, len(jobs))
	for index, current := range jobs {
		if folder == allFolders || filterValue(current.Folder) == folder {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func folderOptions(jobs []job) []string {
	options := []string{allFolders, noFolder}
	seen := map[string]bool{allFolders: true, noFolder: true}
	for _, current := range jobs {
		folder := strings.TrimSpace(current.Folder)
		if folder == "" || seen[folder] {
			continue
		}
		seen[folder] = true
		options = append(options, folder)
	}
	return options
}

func filterValue(folder string) string {
	if strings.TrimSpace(folder) == "" {
		return noFolder
	}
	return strings.TrimSpace(folder)
}

func displayFolder(folder string) string {
	if strings.TrimSpace(folder) == "" {
		return "(" + noFolder + ")"
	}
	return strings.TrimSpace(folder)
}

func displayIndex(indexes []int, jobIndex int) int {
	for display, index := range indexes {
		if index == jobIndex {
			return display
		}
	}
	return 0
}

func showJobDialog(w fyne.Window, title string, current job, onSave func(job)) {
	name := widget.NewEntry()
	name.SetPlaceHolder("Nightly backup")
	name.SetText(current.Name)
	folder := widget.NewEntry()
	folder.SetPlaceHolder("Maintenance")
	folder.SetText(current.Folder)
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
			widget.NewFormItem("Folder", folder),
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
			current.Folder = strings.TrimSpace(folder.Text)
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
	minimizeToTray.SetChecked(true)
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
