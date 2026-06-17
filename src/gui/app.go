package gui

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/gosentry/assets"
	"gitea.mixdep.ru/mix/gosentry/src/core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const appID = "ru.mixdep.gosentry.desktop"
const allFolders = "All"
const noFolder = "No folder"
const minJobsSidebarWidth float32 = 480
const settingsLabelWidth float32 = 140
const settingsControlWidth float32 = 330
const settingsStatusWidth float32 = 280
const projectRepositoryURL = "https://gitea.mixdep.ru/mix/gosentry"
const singleInstanceAddress = "127.0.0.1:37653"
const singleInstanceShowCommand = "show"

// The GUI package aliases core types to keep widget callbacks short. The actual
// durable model still lives in src/core, so GUI code does not define a second
// copy of the scheduler data.
type job = core.Job
type event = core.RunRecord

func Run(startInTray bool) {
	started := time.Now()
	instanceListener, primary := acquireSingleInstance(!startInTray)
	if !primary {
		return
	}
	if instanceListener != nil {
		defer instanceListener.Close()
	}

	// A stable app ID lets Fyne persist desktop preferences consistently across
	// launches and gives tray/window integration a predictable identity.
	a := app.NewWithID(appID)
	a.SetIcon(loadAppIcon())

	w := a.NewWindow("GoSentry " + core.Version)
	configureSystemTray(a, w)
	w.Resize(fyne.NewSize(1120, 720))
	content, recordStartup := newMainView(w)
	w.SetContent(content)
	serveSingleInstance(instanceListener, w)
	if startInTray {
		// Autostart launches intentionally stay hidden, so "window shown" would be
		// a misleading metric. Record a separate startup event for the tray path
		// instead of forcing one timing definition onto two different UX flows.
		recordStartup(time.Since(started), false)
		a.Run()
		return
	}
	// Show the window before recording startup time. Measuring earlier, during
	// widget construction, looked cheaper in History than the user-perceived
	// startup really was. The current point is less abstract: it ends when the
	// window has actually been handed to the desktop for display.
	w.Show()
	recordStartup(time.Since(started), true)
	a.Run()
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

	menu := fyne.NewMenu("GoSentry",
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

func acquireSingleInstance(showExisting bool) (net.Listener, bool) {
	listener, err := net.Listen("tcp", singleInstanceAddress)
	if err == nil {
		return listener, true
	}

	connection, dialErr := net.DialTimeout("tcp", singleInstanceAddress, time.Second)
	if dialErr == nil {
		// The first instance listens only on localhost and understands one tiny
		// command: "show". That keeps the implementation dependency-free and easy
		// to inspect, which matters more here than introducing a named-pipe or
		// platform-specific IPC abstraction just to focus an existing window.
		if showExisting {
			_, _ = io.WriteString(connection, singleInstanceShowCommand)
		}
		_ = connection.Close()
		return nil, false
	}

	// If the port is unavailable but does not answer as GoSentry, continue
	// startup instead of making the application impossible to open because of an
	// unrelated local listener. In the normal duplicate-start case the dial above
	// succeeds and this process exits after waking the first instance.
	return nil, true
}

func serveSingleInstance(listener net.Listener, w fyne.Window) {
	if listener == nil {
		return
	}
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			command, _ := io.ReadAll(io.LimitReader(connection, 32))
			_ = connection.Close()
			if strings.TrimSpace(string(command)) != singleInstanceShowCommand {
				continue
			}
			w.Show()
			w.RequestFocus()
		}
	}()
}

func newMainView(w fyne.Window) (fyne.CanvasObject, func(time.Duration, bool)) {
	store, jobs, err := core.OpenStore()
	if err != nil {
		return container.NewPadded(widget.NewLabel("Failed to load GoSentry configuration: " + err.Error())), func(time.Duration, bool) {}
	}
	if iconPath, err := core.InstallDesktopIntegration(appID, store.Paths.ExecutablePath, assets.IconBytes()); err == nil {
		store.Paths.DesktopIcon = iconPath
	}
	events := collectActivity(jobs)

	// The GUI keeps the loaded jobs slice in memory and persists changes after
	// each edit/run. This keeps the first version responsive and easy to reason
	// about; a database would be unnecessary overhead for one YAML file.
	nextJobID := nextID(jobs)
	selected := 0
	selectedFolder := allFolders
	schedulerPaused := false
	filteredJobs := filteredJobIndexes(jobs, selectedFolder)
	title := widget.NewLabelWithStyle(jobs[selected].Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Wrapping = fyne.TextWrapBreak
	folder := newJobDetailLabel(jobs[selected].Folder)
	schedule := newJobDetailLabel(jobs[selected].Schedule)
	command := newJobDetailLabel(jobs[selected].Command)
	lastRun := newJobDetailLabel(jobs[selected].LastRun)
	nextRun := newJobDetailLabel(jobs[selected].NextRun)
	state := newJobDetailLabel(jobs[selected].LastState)
	schedulerState := widget.NewLabel("Scheduler running")
	commandOutput := widget.NewTextGrid()
	commandOutput.SetText(jobs[selected].Output)
	commandOutputScroll := container.NewScroll(commandOutput)
	// Command output can contain long lines and preserved whitespace. TextGrid is
	// used instead of Label so stdout/stderr remains readable and does not vanish
	// against the theme when it is placed inside a scroll container.
	commandOutputScroll.SetMinSize(fyne.NewSize(520, 160))
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
		showJobDialog(w, "New job", job{Schedule: "@every 1m", Command: "echo GoSentry job ran", Enabled: true, LastRun: "Never", NextRun: "After save", LastState: "Ready"}, func(saved job) {
			saved.ID = nextJobID
			nextJobID++
			jobs = append(jobs, saved)
			selected = len(jobs) - 1
			created := newEvent(saved.ID, saved.Name, "Created", "Job was added")
			// UI events are kept in memory for the current session. They explain
			// user actions in History, while command output remains in log files.
			jobs[selected].Logs = append([]event{created}, jobs[selected].Logs...)
			events = append(events, created)
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
			events = append(events, updated)
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
			events = append(events, newEvent(0, "Scheduler", "Paused", "All job execution paused"))
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
			events = append(events, newEvent(0, "Scheduler", "Resumed", "All job execution resumed"))
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
			events = append(events, resumed)
			if scheduler != nil {
				scheduler.RefreshSchedule(selected)
			}
		} else {
			current.LastState = "Paused"
			current.NextRun = "Paused"
			paused := newEvent(current.ID, current.Name, "Paused", "Job was disabled")
			current.Logs = append([]event{paused}, current.Logs...)
			events = append(events, paused)
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
			events = append(events, newEvent(deleted.ID, deleted.Name, "Deleted", "Job was removed"))
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
		events = append(events, record)
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

	return tabs, recordStartup
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
	// Use the same timestamp shape as command run records so the History tab is
	// visually consistent across startup, UI actions, manual runs, and schedules.
	return event{
		Time:    time.Now().Format("2006-01-02 15:04:05"),
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
	sort.SliceStable(events, func(left int, right int) bool {
		return events[left].Time < events[right].Time
	})
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

func newJobDetailLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	// Job names, commands, and paths can be much wider than the details panel.
	// Breaking long runs of text keeps Label.MinSize stable when the selection
	// changes, so the right panel does not force the whole window to resize.
	label.Wrapping = fyne.TextWrapBreak
	return label
}

func settingsRow(label string, value fyne.CanvasObject) fyne.CanvasObject {
	caption := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	caption.Wrapping = fyne.TextTruncate
	captionBox := container.New(minWidthLayout{width: settingsLabelWidth}, caption)
	return container.NewBorder(nil, nil, captionBox, nil, value)
}

func settingsRowWithStatus(label string, value fyne.CanvasObject, status fyne.CanvasObject) fyne.CanvasObject {
	valueBox := container.New(minWidthLayout{width: settingsControlWidth}, value)
	statusBox := container.New(minWidthLayout{width: settingsStatusWidth}, status)
	return settingsRow(label, container.NewBorder(nil, nil, valueBox, nil, statusBox))
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
	command.SetPlaceHolder("echo GoSentry job ran")
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
	descending := false
	headerText := func(id widget.TableCellID) string {
		headers := []string{"Time", "Trigger", "Job", "State", "Detail", "Log"}
		if id.Row < 0 && id.Col == 0 {
			if descending {
				return "Time desc"
			}
			return "Time asc"
		}
		if id.Row < 0 && id.Col >= 0 && id.Col < len(headers) {
			return headers[id.Col]
		}
		return ""
	}
	sortedEvents := func() []event {
		result := append([]event(nil), (*events)...)
		sort.SliceStable(result, func(left int, right int) bool {
			if descending {
				return result[left].Time > result[right].Time
			}
			return result[left].Time < result[right].Time
		})
		return result
	}

	table := widget.NewTable(
		func() (int, int) {
			return len(*events), 6
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Wrapping = fyne.TextTruncate
			return label
		},
		func(id widget.TableCellID, item fyne.CanvasObject) {
			label := item.(*widget.Label)
			label.SetText(historyCellText(id, sortedEvents()))
			label.TextStyle = fyne.TextStyle{}
			label.Refresh()
		},
	)
	table.ShowHeaderRow = true
	table.CreateHeader = func() fyne.CanvasObject {
		label := widget.NewLabel("")
		label.Wrapping = fyne.TextTruncate
		return label
	}
	table.UpdateHeader = func(id widget.TableCellID, item fyne.CanvasObject) {
		label := item.(*widget.Label)
		label.SetText(headerText(id))
		label.TextStyle = fyne.TextStyle{Bold: true}
		label.Refresh()
	}
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row < 0 && id.Col == 0 {
			descending = !descending
			table.Refresh()
		}
		table.Unselect(id)
	}
	table.SetColumnWidth(0, 150)
	table.SetColumnWidth(1, 90)
	table.SetColumnWidth(2, 170)
	table.SetColumnWidth(3, 90)
	table.SetColumnWidth(4, 260)
	table.SetColumnWidth(5, 240)
	return container.NewPadded(table)
}

func historyCellText(id widget.TableCellID, events []event) string {
	if id.Row < 0 || id.Row >= len(events) {
		return ""
	}
	current := events[id.Row]
	trigger := current.Trigger
	if trigger == "" {
		trigger = "Unknown"
	}
	switch id.Col {
	case 0:
		return current.Time
	case 1:
		return trigger
	case 2:
		return current.JobName
	case 3:
		return current.State
	case 4:
		return current.Detail
	case 5:
		return logFileName(current.LogFile)
	default:
		return ""
	}
}

func logFileName(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.ReplaceAll(path, "\\", "/")
	if slash := strings.LastIndex(path, "/"); slash >= 0 {
		return path[slash+1:]
	}
	return path
}

func settingsView(w fyne.Window, store *core.Store, jobs *[]job) fyne.CanvasObject {
	startOnLogin := widget.NewCheck("Start on login", nil)
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
		settingsRowWithStatus("Autostart", startOnLogin, autostartStatus),
		settingsRow("Tray", container.New(minWidthLayout{width: settingsControlWidth}, minimizeToTray)),
		settingsRow("Notifications", container.New(minWidthLayout{width: settingsControlWidth}, notifications)),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Storage", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		settingsRow("Config YAML", widget.NewLabel(store.Paths.ConfigPath)),
		settingsRow("Jobs directory", container.NewBorder(nil, nil, nil, jobsDirBrowse, jobsDir)),
		settingsRow("Logs directory", container.NewBorder(nil, nil, nil, logsDirBrowse, logsDir)),
		settingsRow("Max log files", maxLogFiles),
		settingsRow("Max log age days", maxLogAgeDays),
		saveSettings,
		settingsStatus,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("About", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		settingsRow("GoSentry", widget.NewLabel(core.Version)),
		settingsRow("Go", widget.NewLabel(runtime.Version())),
		settingsRow("Fyne", widget.NewLabel(fyneVersion())),
		settingsRow("Repository", widget.NewHyperlink(projectRepositoryURL, mustParseURL(projectRepositoryURL))),
	))
}

func fyneVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dependency := range info.Deps {
		if dependency.Path == "fyne.io/fyne/v2" {
			if dependency.Replace != nil && dependency.Replace.Version != "" {
				return dependency.Replace.Version
			}
			if dependency.Version != "" {
				return dependency.Version
			}
			return "local"
		}
	}
	return "unknown"
}

func mustParseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		return &url.URL{}
	}
	return parsed
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
