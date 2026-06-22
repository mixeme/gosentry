package ui

import (
	"fmt"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const allFolders = "All"
const noFolder = "No folder"
const minJobsSidebarWidth float32 = 480

// newJobsView builds the Jobs tab: list sidebar, details panel, and toolbar.
// It returns the assembled panel and a refresh function the caller invokes
// whenever the service state may have changed (e.g., from the event subscriber
// in mainwindow.go). The refresh function re-reads the service snapshot and
// redraws all widgets in the jobs view; it does NOT touch history or settings.
func newJobsView(w fyne.Window, svc *app.Service) (fyne.CanvasObject, func()) {
	jobs := svc.Jobs()
	runtimes := make(map[int]*domain.JobRuntime, len(jobs))
	syncFromService := func() {
		jobs = svc.Jobs()
		for id := range runtimes {
			delete(runtimes, id)
		}
		for _, current := range jobs {
			if rt := svc.Runtime(current.ID); rt != nil {
				runtimes[current.ID] = rt
			}
		}
	}
	syncFromService()
	runtimeFor := func(index int) *domain.JobRuntime {
		if index < 0 || index >= len(jobs) {
			return &domain.JobRuntime{}
		}
		if rt := runtimes[jobs[index].ID]; rt != nil {
			return rt
		}
		return &domain.JobRuntime{}
	}

	selected := 0
	selectedFolder := allFolders
	schedulerPaused := false
	filteredJobs := filteredJobIndexes(jobs, selectedFolder)

	title := widget.NewLabelWithStyle(jobs[selected].Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Wrapping = fyne.TextWrapBreak
	folderLabel := newJobDetailLabel(jobs[selected].Folder)
	scheduleLabel := newJobDetailLabel(jobs[selected].Schedule)
	commandLabel := newJobDetailLabel(jobs[selected].Command)
	argumentsLabel := newJobDetailLabel(jobs[selected].Arguments)
	runModeLabel := newJobDetailLabel(app.DisplayRunMode(jobs[selected]))
	selectedRuntime := runtimeFor(selected)
	lastRunLabel := newJobDetailLabel(selectedRuntime.LastRun)
	nextRunLabel := newJobDetailLabel(selectedRuntime.NextRun)
	stateLabel := newJobDetailLabel(selectedRuntime.LastState)
	schedulerState := widget.NewLabel("Scheduler running")
	commandOutput := widget.NewTextGrid()
	commandOutput.SetText(selectedRuntime.Output)
	commandOutputScroll := container.NewScroll(commandOutput)
	// Command output can contain long lines and preserved whitespace. TextGrid is
	// used instead of Label so stdout/stderr remains readable and does not vanish
	// against the theme when it is placed inside a scroll container.
	commandOutputScroll.SetMinSize(fyne.NewSize(520, 160))

	selectedLogs := append([]event(nil), selectedRuntime.Logs...)
	jobLogs := widget.NewList(
		func() int { return len(selectedLogs) },
		func() fyne.CanvasObject { return widget.NewLabel("log") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			item.(*widget.Label).SetText(app.EventText(selectedLogs[id]))
		},
	)

	updateDetails := func(index int) {
		if index < 0 || index >= len(jobs) {
			// A folder filter can temporarily leave no selectable rows. Clearing
			// the details panel avoids showing stale information for a hidden job.
			title.SetText("No job selected")
			folderLabel.SetText("")
			scheduleLabel.SetText("")
			commandLabel.SetText("")
			argumentsLabel.SetText("")
			runModeLabel.SetText("")
			lastRunLabel.SetText("")
			nextRunLabel.SetText("")
			stateLabel.SetText("")
			commandOutput.SetText("")
			selectedLogs = nil
			return
		}
		selected = index
		current := jobs[selected]
		rt := runtimeFor(selected)
		title.SetText(current.Name)
		folderLabel.SetText(app.DisplayFolder(current.Folder))
		scheduleLabel.SetText(current.Schedule)
		commandLabel.SetText(current.Command)
		argumentsLabel.SetText(app.DisplayArguments(current.Arguments))
		runModeLabel.SetText(app.DisplayRunMode(current))
		lastRunLabel.SetText(rt.LastRun)
		nextRunLabel.SetText(rt.NextRun)
		stateLabel.SetText(rt.LastState)
		commandOutput.SetText(rt.Output)
		selectedLogs = append(selectedLogs[:0], rt.Logs...)
	}

	// list and folderSelect are declared early so closures below can reference
	// them before the widget.NewList / widget.NewSelect calls assign the values.
	var list *widget.List
	var folderSelect *widget.Select

	refreshView := func() {
		syncFromService()
		filteredJobs = filteredJobIndexes(jobs, selectedFolder)
		updateDetails(selected)
		jobLogs.Refresh()
		if list != nil {
			list.Refresh()
		}
	}

	list = widget.NewList(
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
			meta.SetText(app.DisplayFolder(current.Folder) + "    " + current.Schedule + "    " + app.DisplayInvocation(current))
			status.SetText(app.StatusText(current, runtimes[current.ID]))
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

	folderSelect = widget.NewSelect(folderOptions(jobs), func(value string) {
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
		refreshView()
	})
	folderSelect.SetSelected(selectedFolder)

	addButton := widget.NewButtonWithIcon("New job", theme.ContentAddIcon(), func() {
		showJobDialog(w, "New job", job{Schedule: "@every 1m", Command: "echo GoSentry job ran", Enabled: true}, func(saved job) {
			// The Service assigns the ID, stores the job, records the "Created"
			// activity, and emits events. The observer appends those to History; we
			// only refresh the snapshot and move the selection to the new job.
			created, err := svc.CreateJob(saved)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			syncFromService()
			folderSelect.Options = folderOptions(jobs)
			folderSelect.Refresh()
			targetFolder := filterValue(created.Folder)
			if selectedFolder != allFolders && selectedFolder != targetFolder {
				selectedFolder = targetFolder
				folderSelect.SetSelected(targetFolder)
			}
			selected = indexOfID(jobs, created.ID)
			filteredJobs = filteredJobIndexes(jobs, selectedFolder)
			list.Refresh()
			list.Select(app.DisplayIndex(filteredJobs, selected))
			refreshView()
		})
	})
	editButton := widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		if selected < 0 || selected >= len(jobs) {
			return
		}
		showJobDialog(w, "Edit job", jobs[selected], func(saved job) {
			// The job keeps its ID, so the Service preserves the runtime (keyed by
			// ID), reflects any enabled/disabled change, recomputes the next run, and
			// emits the "Updated" activity the observer records.
			saved.ID = jobs[selected].ID
			if err := svc.UpdateJob(saved); err != nil {
				dialog.ShowError(err, w)
				return
			}
			syncFromService()
			folderSelect.Options = folderOptions(jobs)
			folderSelect.Refresh()
			list.Refresh()
			refreshView()
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
		// RunNow refuses an already-running job (it returns an error); the UI has
		// always ignored that case silently, so the run simply does not start.
		if err := svc.RunNow(jobs[selected].ID); err != nil {
			return
		}
		list.Refresh()
		refreshView()
	})
	stopAllButton := widget.NewButtonWithIcon("Pause all", theme.MediaStopIcon(), nil)
	stopAllButton.OnTapped = func() {
		// SetGlobalPause flips the Service's pause flag, updates every job's
		// next-run text, and emits the activity record the observer logs. Mirror the
		// new state into the local flag and the controls; revert it if the save fails.
		schedulerPaused = !schedulerPaused
		if err := svc.SetGlobalPause(schedulerPaused); err != nil {
			schedulerPaused = !schedulerPaused
			dialog.ShowError(err, w)
			return
		}
		if schedulerPaused {
			schedulerState.SetText("Scheduler paused")
			stopAllButton.SetText("Resume all")
			stopAllButton.SetIcon(theme.MediaPlayIcon())
		} else {
			schedulerState.SetText("Scheduler running")
			stopAllButton.SetText("Pause all")
			stopAllButton.SetIcon(theme.MediaStopIcon())
		}
		list.Refresh()
		refreshView()
	}
	pauseButton := widget.NewButtonWithIcon("Pause", theme.MediaPauseIcon(), func() {
		if selected < 0 || selected >= len(jobs) {
			return
		}
		// SetEnabled toggles the job, updates its runtime/next-run, and records the
		// "Resumed"/"Paused" activity the observer logs.
		current := jobs[selected]
		if err := svc.SetEnabled(current.ID, !current.Enabled); err != nil {
			dialog.ShowError(err, w)
			return
		}
		syncFromService()
		list.Refresh()
		refreshView()
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
			// The Service removes the job and its runtime, persists, and records the
			// "Deleted" activity the observer logs; the UI re-reads the snapshot and
			// fixes up the folder filter and selection.
			if err := svc.DeleteJob(deleted.ID); err != nil {
				dialog.ShowError(err, w)
				return
			}
			syncFromService()
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
			list.Refresh()
			if selected >= 0 {
				list.Select(app.DisplayIndex(filteredJobs, selected))
			}
			refreshView()
		}, w)
	})

	toolbar := container.NewHBox(addButton, editButton, runButton, pauseButton, deleteButton, layout.NewSpacer())
	globalControls := container.NewHBox(stopAllButton, schedulerState, layout.NewSpacer())
	sidebarHeader := container.NewVBox(globalControls, widget.NewSeparator(), widget.NewLabelWithStyle("Folder", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), folderSelect, toolbar)
	sidebar := container.NewBorder(sidebarHeader, nil, nil, nil, list)

	details := container.NewVBox(
		title,
		widget.NewSeparator(),
		detailRow("Folder", folderLabel),
		detailRow("Schedule", scheduleLabel),
		detailRow("Command", commandLabel),
		detailRow("Arguments", argumentsLabel),
		detailRow("Run mode", runModeLabel),
		detailRow("Last run", lastRunLabel),
		detailRow("Next run", nextRunLabel),
		detailRow("State", stateLabel),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Command output", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		commandOutputScroll,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Selected job activity", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		jobLogs,
	)

	fixedSidebar := container.New(minWidthLayout{width: minJobsSidebarWidth}, sidebar)
	panel := container.NewBorder(nil, nil, fixedSidebar, nil, container.NewPadded(details))
	return panel, refreshView
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

func indexOfID(jobs []job, id int) int {
	for index, current := range jobs {
		if current.ID == id {
			return index
		}
	}
	return 0
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
