package gui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pysentry/pysentry/assets"
	"github.com/pysentry/pysentry/src/core"

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
const minJobsSidebarWidth float32 = 480

// The GUI package aliases core types to keep widget callbacks short. The actual
// durable model still lives in src/core, so GUI code does not define a second
// copy of the scheduler data.
type job = core.Job
type event = core.RunRecord

func Run() {
	started := time.Now()
	// A stable app ID lets Fyne persist desktop preferences consistently across
	// launches and gives tray/window integration a predictable identity.
	a := app.NewWithID(appID)
	a.SetIcon(loadAppIcon())

	w := a.NewWindow("PySentry " + core.Version)
	configureSystemTray(a, w)
	w.Resize(fyne.NewSize(1120, 720))
	w.SetContent(newMainView(w, started))
	w.ShowAndRun()
}

func loadAppIcon() fyne.Resource {
	return assets.Icon()
}

func configureSystemTray(a fyne.App, w fyne.Window) {
	desk, ok := a.(desktop.App)
	if !ok {
		// Not every Fyne driver exposes desktop tray features. Returning silently
		// keeps the same binary usable on platforms or sessions without a tray.
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
		// Closing hides the window instead of quitting because scheduler tools are
		// expected to keep working in the background. The explicit Quit tray item
		// remains the way to stop the process.
		w.Hide()
	})
}

func newMainView(w fyne.Window, started time.Time) fyne.CanvasObject {
	store, jobs, err := core.OpenStore()
	if err != nil {
		return container.NewPadded(widget.NewLabel("Failed to load PySentry configuration: " + err.Error()))
	}
	if iconPath, err := core.InstallDesktopIntegration(appID, store.Paths.ExecutablePath, assets.IconBytes()); err == nil {
		store.Paths.DesktopIcon = iconPath
	}
	startupDuration := time.Since(started).Round(time.Millisecond)
	events := append([]event{newEvent(0, "Application", "Started", "Startup completed in "+startupDuration.String())}, collectActivity(jobs)...)

	// The GUI keeps the loaded jobs slice in memory and persists changes after
	// each edit/run. This keeps the first version responsive and easy to reason
	// about; a database would be unnecessary overhead for one YAML file.
	nextJobID := nextID(jobs)
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
	commandOutput := widget.NewTextGrid()
	commandOutput.SetText(jobs[selected].Output)
	commandOutputScroll := container.NewScroll(commandOutput)
	// Command output can contain long lines and preserved whitespace. TextGrid is
	// used instead of Label so stdout/stderr remains readable and does not vanish
	// against the theme when it is placed inside a scroll container.
	commandOutputScroll.SetMinSize(fyne.NewSize(520, 160))
	history := newHistoryView(&events)
	selectedLogs := append([]event(nil), jobs[selected].Logs...)
	jobLogs := widget.NewList(
		func() int {
			return len(selectedLogs)
		},
		func() fyne.CanvasObject { return widget.NewLabel("log") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			item.(*widget.Label).SetText(eventText(selectedLogs[id]))
		},
	)

	updateDetails := func(index int) {
		if index < 0 || index >= len(jobs) {
			// A folder filter can temporarily leave no selectable rows. Clearing
			// the details panel avoids showing stale information for a hidden job.
			title.SetText("No job selected")
			folder.SetText("")
			schedule.SetText("")
			command.SetText("")
			lastRun.SetText("")
			nextRun.SetText("")
			state.SetText("")
			commandOutput.SetText("")
			selectedLogs = nil
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
		selectedLogs = append(selectedLogs[:0], current.Logs...)
	}
	refresh := func() {
		// Several callbacks mutate jobs, filters, and event history. A single
		// refresh closure keeps the different widgets synchronized after each
		// mutation without introducing a heavier state-management layer.
		filteredJobs = filteredJobIndexes(jobs, selectedFolder)
		updateDetails(selected)
		jobLogs.Refresh()
		history.Refresh()
	}
	var scheduler *core.Scheduler

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
			// Keep each row compact: folder, schedule, and command are shown in one
			// metadata line so the left pane stays useful even with many jobs.
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
			// The "No folder" filter is intentionally allowed to be empty. It is a
			// real filter choice, not an error state, so the selection is cleared.
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
		showJobDialog(w, "New job", job{Schedule: "@every 1m", Command: "echo PySentry job ran", Enabled: true, LastRun: "Never", NextRun: "After save", LastState: "Ready"}, func(saved job) {
			saved.ID = nextJobID
			nextJobID++
			jobs = append(jobs, saved)
			selected = len(jobs) - 1
			created := newEvent(saved.ID, saved.Name, "Created", "Job was added")
			// UI events are kept in memory for the current session. They explain
			// user actions in History, while command output remains in log files.
			jobs[selected].Logs = append([]event{created}, jobs[selected].Logs...)
			events = append([]event{created}, events...)
			_ = store.SaveJobs(jobs)
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
			if scheduler != nil {
				scheduler.RefreshSchedule(selected)
			}
			_ = store.SaveJobs(jobs)
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
			// The global pause is treated as an emergency stop for all execution,
			// including manual "Run now", so the user has one reliable switch.
			dialog.ShowInformation("Scheduler paused", "Global pause is active. Resume the scheduler before running jobs.", w)
			return
		}
		if !scheduler.RunNow(selected) {
			return
		}
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
			if scheduler != nil {
				scheduler.SetPaused(true)
			}
			events = append([]event{newEvent(0, "Scheduler", "Paused", "All job execution paused")}, events...)
		} else {
			schedulerState.SetText("Scheduler running")
			stopAllButton.SetText("Pause all")
			stopAllButton.SetIcon(theme.MediaStopIcon())
			for index := range jobs {
				if jobs[index].Enabled && jobs[index].NextRun == "Scheduler paused" {
					// The scheduler will calculate the exact next run when it is
					// resumed; this interim text prevents a stale paused timestamp.
					jobs[index].NextRun = "Waiting for scheduler"
				}
			}
			if scheduler != nil {
				scheduler.SetPaused(false)
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
			if scheduler != nil {
				scheduler.RefreshSchedule(selected)
			}
		} else {
			current.LastState = "Paused"
			current.NextRun = "Paused"
			paused := newEvent(current.ID, current.Name, "Paused", "Job was disabled")
			current.Logs = append([]event{paused}, current.Logs...)
			events = append([]event{paused}, events...)
			if scheduler != nil {
				scheduler.RefreshSchedule(selected)
			}
		}
		_ = store.SaveJobs(jobs)
		list.Refresh()
		refresh()
	})
	deleteButton := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		if selected < 0 || selected >= len(jobs) {
			return
		}
		deleted := jobs[selected]
		// Deletion is confirmed because jobs can represent real system actions.
		// There is no undo yet, so accidental removal should require one more click.
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
			_ = store.SaveJobs(jobs)
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
		commandOutputScroll,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Selected job activity", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		jobLogs,
	)

	scheduler = core.NewScheduler(store, &jobs, func(record core.RunRecord) {
		// Scheduled runs happen on the scheduler goroutine. The callback updates
		// the shared in-memory event list so History reflects background activity.
		events = append([]event{record}, events...)
		refresh()
	})
	scheduler.Start()

	fixedSidebar := container.New(minWidthLayout{width: minJobsSidebarWidth}, sidebar)
	jobsView := container.NewBorder(nil, nil, fixedSidebar, nil, container.NewPadded(details))
	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Jobs", theme.ListIcon(), jobsView),
		container.NewTabItemWithIcon("History", theme.HistoryIcon(), history),
		container.NewTabItemWithIcon("Settings", theme.SettingsIcon(), settingsView(w, store, &jobs)),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	return tabs
}

type minWidthLayout struct {
	width float32
}

func (layout minWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	width := layout.width
	var height float32
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		min := object.MinSize()
		if min.Width > width {
			width = min.Width
		}
		if min.Height > height {
			height = min.Height
		}
	}
	return fyne.NewSize(width, height)
}

func (layout minWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		object.Move(fyne.NewPos(0, 0))
		object.Resize(size)
	}
}

func statusText(j job) string {
	if !j.Enabled {
		return "Paused"
	}
	return j.LastState
}

func newEvent(jobID int, jobName string, state string, detail string) event {
	// UI events use a short time because they are session-local activity markers.
	// Command runs use full timestamps from core.RunJob and have log files.
	return event{
		Time:    time.Now().Format("15:04:05"),
		JobID:   jobID,
		JobName: jobName,
		Trigger: "UI",
		State:   state,
		Detail:  detail,
	}
}

func eventText(e event) string {
	trigger := e.Trigger
	if trigger == "" {
		trigger = "Unknown"
	}
	if e.LogFile != "" {
		return fmt.Sprintf("%s  %s  %s  %s  %s  %s", e.Time, trigger, e.JobName, e.State, e.Detail, e.LogFile)
	}
	return fmt.Sprintf("%s  %s  %s  %s  %s", e.Time, trigger, e.JobName, e.State, e.Detail)
}

func collectActivity(jobs []job) []event {
	var events []event
	for _, current := range jobs {
		// At startup this is usually empty because jobs.yaml does not persist
		// runtime logs. The function still centralizes the merge for future
		// history loading from log metadata.
		events = append(events, current.Logs...)
	}
	return events
}

func nextID(jobs []job) int {
	next := 1
	for _, current := range jobs {
		if current.ID >= next {
			next = current.ID + 1
		}
	}
	return next
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
	// "All" and "No folder" are always present so the filter UI is stable even
	// before the user creates folders.
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
	schedule.SetPlaceHolder("@every 1m")
	schedule.SetText(current.Schedule)
	command := widget.NewEntry()
	command.SetPlaceHolder("echo PySentry job ran")
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
				// These three fields are the minimum executable job definition.
				// Folder is optional because ungrouped jobs are a supported workflow.
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

func settingsView(w fyne.Window, store *core.Store, jobs *[]job) fyne.CanvasObject {
	startOnLogin := widget.NewCheck("Start PySentry when I sign in", nil)
	startOnLogin.SetChecked(store.Config.StartOnLogin)
	autostartStatus := widget.NewLabel("")
	refreshAutostartStatus := func() {
		ok, message := core.AutostartStatus(store.Config.StartOnLogin, store.Paths.ExecutablePath)
		if ok {
			autostartStatus.SetText("OK: " + message)
			return
		}
		autostartStatus.SetText("Problem: " + message)
	}
	startOnLogin.OnChanged = func(bool) {
		if startOnLogin.Checked != store.Config.StartOnLogin {
			autostartStatus.SetText("Pending: save settings to apply")
			return
		}
		refreshAutostartStatus()
	}
	refreshAutostartStatus()
	minimizeToTray := widget.NewCheck("Keep running in the system tray", nil)
	minimizeToTray.SetChecked(store.Config.KeepRunningInTray)
	notifications := widget.NewCheck("Show desktop notifications for failed jobs", nil)
	notifications.SetChecked(store.Config.NotifyOnFailure)
	jobsDir := widget.NewEntry()
	jobsDir.SetText(store.Config.JobsDir)
	jobsDirBrowse := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		chooseFolder(w, jobsDir)
	})
	logsDir := widget.NewEntry()
	logsDir.SetText(store.Config.LogsDir)
	logsDirBrowse := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		chooseFolder(w, logsDir)
	})
	maxLogFiles := widget.NewEntry()
	maxLogFiles.SetText(strconv.Itoa(store.Config.MaxLogFiles))
	maxLogAgeDays := widget.NewEntry()
	maxLogAgeDays.SetText(strconv.Itoa(store.Config.MaxLogAgeDays))
	settingsStatus := widget.NewLabel("")

	saveSettings := widget.NewButtonWithIcon("Save settings", theme.DocumentSaveIcon(), func() {
		files, err := strconv.Atoi(strings.TrimSpace(maxLogFiles.Text))
		if err != nil || files <= 0 {
			settingsStatus.SetText("Max log files must be a positive number")
			return
		}
		days, err := strconv.Atoi(strings.TrimSpace(maxLogAgeDays.Text))
		if err != nil || days <= 0 {
			settingsStatus.SetText("Max log age days must be a positive number")
			return
		}
		store.Config.LogsDir = strings.TrimSpace(logsDir.Text)
		if strings.TrimSpace(jobsDir.Text) == "" {
			settingsStatus.SetText("Jobs directory is required")
			return
		}
		if strings.TrimSpace(logsDir.Text) == "" {
			settingsStatus.SetText("Logs directory is required")
			return
		}
		store.Config.JobsDir = strings.TrimSpace(jobsDir.Text)
		store.Config.MaxLogFiles = files
		store.Config.MaxLogAgeDays = days
		store.Config.StartOnLogin = startOnLogin.Checked
		store.Config.KeepRunningInTray = minimizeToTray.Checked
		store.Config.NotifyOnFailure = notifications.Checked
		if err := store.SaveConfig(); err != nil {
			settingsStatus.SetText("Save failed: " + err.Error())
			return
		}
		if err := core.SetAutostart(store.Config.StartOnLogin, store.Paths.ExecutablePath, store.Paths.DesktopIcon); err != nil {
			refreshAutostartStatus()
			settingsStatus.SetText("Saved, autostart failed: " + err.Error())
			return
		}
		refreshAutostartStatus()
		// When the jobs directory changes, save the currently loaded jobs to the
		// newly resolved path immediately. That makes the setting visible on disk
		// without requiring a restart or a separate migration command.
		if err := store.SaveJobs(*jobs); err != nil {
			settingsStatus.SetText("Jobs save failed: " + err.Error())
			return
		}
		// Cleanup runs on settings save so a user who tightens retention limits
		// sees the new policy take effect right away.
		if err := core.CleanupLogs(store.Paths.LogsDir, store.Config.MaxLogFiles, store.Config.MaxLogAgeDays); err != nil {
			settingsStatus.SetText("Saved, cleanup failed: " + err.Error())
			return
		}
		settingsStatus.SetText("Saved")
	})

	return container.NewPadded(container.NewVBox(
		widget.NewLabelWithStyle("Application", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		detailRow("Version", widget.NewLabel(core.Version)),
		detailRow("Start on login", container.NewBorder(nil, nil, nil, autostartStatus, startOnLogin)),
		minimizeToTray,
		notifications,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Storage", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		detailRow("Config YAML", widget.NewLabel(store.Paths.ConfigPath)),
		detailRow("Jobs directory", container.NewBorder(nil, nil, nil, jobsDirBrowse, jobsDir)),
		detailRow("Logs directory", container.NewBorder(nil, nil, nil, logsDirBrowse, logsDir)),
		detailRow("Max log files", maxLogFiles),
		detailRow("Max log age days", maxLogAgeDays),
		saveSettings,
		settingsStatus,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Scheduler", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Current core supports @every schedules and standard 5-field cron expressions."),
	))
}

func chooseFolder(w fyne.Window, target *widget.Entry) {
	folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		target.SetText(uri.Path())
	}, w)
	// The default folder picker can be cramped on Windows. A larger size makes
	// long paths readable and avoids forcing the user to resize it every time.
	folderDialog.Resize(fyne.NewSize(900, 640))
	folderDialog.Show()
}
