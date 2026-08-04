package ui

import (
	"fmt"

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

// maxJobActivityRows caps the "Selected job activity" panel to the most recent
// entries. The full per-job history (up to maxJobLogs) remains in the History
// view; this panel is a quick at-a-glance summary anchored below the output.
const maxJobActivityRows = 3

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
	if len(jobs) == 0 {
		selected = -1
	}
	selectedFolder := allFolders
	schedulerPaused := svc.Store().Config.Paused
	listView := svc.Store().Config.JobListView
	filteredJobs := filteredJobIndexes(jobs, selectedFolder)

	dp := newDetailsPanel(job{}, &domain.JobRuntime{}, svc.Store().Config.OverlapPolicy, svc.Store().Config.DefaultTimeoutSeconds)
	if selected >= 0 {
		dp.update(jobs[selected], runtimeFor(selected), svc.Store().Config.OverlapPolicy, svc.Store().Config.DefaultTimeoutSeconds)
	} else {
		dp.clear()
	}

	updateDetails := func(index int) {
		if index < 0 || index >= len(jobs) {
			// A folder filter can temporarily leave no selectable rows. Clearing
			// the details panel avoids showing stale information for a hidden job.
			dp.clear()
			return
		}
		selected = index
		dp.update(jobs[selected], runtimeFor(selected), svc.Store().Config.OverlapPolicy, svc.Store().Config.DefaultTimeoutSeconds)
	}

	// list and folderSelect are declared early so closures below can reference
	// them before the widget.NewList / widget.NewSelect calls assign the values.
	var list *widget.List
	var folderSelect *widget.Select

	refreshView := func() {
		syncFromService()
		filteredJobs = filteredJobIndexes(jobs, selectedFolder)
		updateDetails(selected)
		dp.logs.Refresh()
		if list != nil {
			list.Refresh()
		}
	}

	// applyRowMode expresses the current view mode as visibility on the row's
	// four labels. widget.List caches the row template's MinSize, and
	// list.Refresh() re-creates the template and recomputes it, so hiding lines
	// is what actually shrinks the rows: layout.NewCustomPaddedVBoxLayout and the
	// border layout both skip hidden children when measuring.
	applyRowMode := func(inlineStatus, meta, status fyne.CanvasObject) {
		if listView.IsCompact() {
			inlineStatus.Show()
			meta.Hide()
			status.Hide()
			return
		}
		inlineStatus.Hide()
		meta.Show()
		status.Show()
	}

	list = widget.NewList(
		func() int { return len(filteredJobs) },
		func() fyne.CanvasObject {
			name := widget.NewLabelWithStyle("Job name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			// Truncating stops a long name from pushing the compact row's status
			// off the right-hand edge. Labels default to TextWrapOff, which grows
			// the widget to fit instead.
			name.Truncation = fyne.TextTruncateClip
			inlineStatus := widget.NewLabel("status")
			meta := widget.NewLabel("schedule")
			status := widget.NewLabel("status")
			applyRowMode(inlineStatus, meta, status)
			nameLine := container.NewBorder(nil, nil, nil, inlineStatus, name)
			return container.New(layout.NewCustomPaddedVBoxLayout(rowOverlap()), nameLine, meta, status)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			row := item.(*fyne.Container)
			// NewBorder keeps the center object first and appends the border slots
			// after it, so nameLine is [name, inlineStatus].
			nameLine := row.Objects[0].(*fyne.Container)
			name := nameLine.Objects[0].(*widget.Label)
			inlineStatus := nameLine.Objects[1].(*widget.Label)
			meta := row.Objects[1].(*widget.Label)
			status := row.Objects[2].(*widget.Label)

			current := jobs[filteredJobs[id]]
			name.SetText(current.Name)
			// Keep each row compact: folder, schedule, and command are shown in one
			// metadata line so the left pane stays useful even with many jobs.
			meta.SetText(app.DisplayFolder(current.Folder) + "    " + current.Schedule + "    " + app.DisplayInvocation(current))
			statusText := app.StatusText(current, runtimes[current.ID])
			status.SetText(statusText)
			inlineStatus.SetText(statusText)
			// A full Refresh reuses rows built under the previous mode, so
			// visibility cannot be left to the create callback alone.
			applyRowMode(inlineStatus, meta, status)
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(filteredJobs) {
			updateDetails(-1)
			return
		}
		updateDetails(filteredJobs[id])
	}
	if len(filteredJobs) > 0 && selected >= 0 {
		list.Select(app.DisplayIndex(filteredJobs, selected))
	}

	folderSelect = widget.NewSelect(folderOptions(jobs), func(value string) {
		if value == "" {
			return
		}
		selectedFolder = value
		filteredJobs = filteredJobIndexes(jobs, selectedFolder)
		if len(filteredJobs) == 0 {
			// The "No folder" filter is intentionally allowed to be empty. It is a
			// real filter choice, not an error state, so the selection is cleared.
			// This path returns without reaching refreshView(), so it is the one
			// place the list has to be redrawn by hand.
			selected = -1
			updateDetails(-1)
			list.Refresh()
			return
		}
		selected = filteredJobs[0]
		list.Select(0)
		refreshView()
	})
	folderSelect.SetSelected(selectedFolder)

	// viewToggleIcon pairs with viewToggleText: both name the action the button
	// performs, not the state it is in, matching stopAllButton's convention.
	viewToggleIcon := func(current domain.JobListView) fyne.Resource {
		if current.IsCompact() {
			return theme.ViewFullScreenIcon()
		}
		return theme.ListIcon()
	}
	viewButton := widget.NewButtonWithIcon(viewToggleText(listView), viewToggleIcon(listView), nil)
	viewButton.OnTapped = func() {
		next := nextJobListView(listView)
		listView = next
		if err := svc.SetJobListView(next); err != nil {
			// Roll the mode back and leave the button as it was, so the button
			// never claims a preference that did not reach disk.
			listView = nextJobListView(next)
			dialog.ShowError(err, w)
			return
		}
		viewButton.SetText(viewToggleText(listView))
		viewButton.SetIcon(viewToggleIcon(listView))
		// Refresh re-creates the row template, which is what recomputes the
		// cached row height for the new mode. Selection is untouched.
		list.Refresh()
	}

	addButton := widget.NewButtonWithIcon("New job", theme.ContentAddIcon(), func() {
		showJobDialog(w, "New job", job{Schedule: "@every 1m", Command: "echo GoSentry job ran", Enabled: true}, func(saved job) {
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
			list.Select(app.DisplayIndex(filteredJobs, selected))
			refreshView()
		})
	})
	editButton := widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		if selected < 0 || selected >= len(jobs) {
			return
		}
		showJobDialog(w, "Edit job", jobs[selected], func(saved job) {
			saved.ID = jobs[selected].ID
			if err := svc.UpdateJob(saved); err != nil {
				dialog.ShowError(err, w)
				return
			}
			syncFromService()
			folderSelect.Options = folderOptions(jobs)
			folderSelect.Refresh()
			refreshView()
		})
	})
	runButton := widget.NewButtonWithIcon("Run now", theme.MediaPlayIcon(), func() {
		if selected < 0 || selected >= len(jobs) {
			return
		}
		// A manual run is allowed even while the scheduler is paused: pause only
		// stops automatic scheduled runs, not the user's explicit "Run now".
		if err := svc.RunNow(jobs[selected].ID); err != nil {
			dialog.ShowError(err, w)
			return
		}
		refreshView()
	})

	stopAllText, stopAllIcon := "Disable auto", theme.MediaPauseIcon()
	if schedulerPaused {
		stopAllText, stopAllIcon = "Enable auto", theme.MediaPlayIcon()
	}
	schedulerStateText := "Scheduler running"
	if schedulerPaused {
		schedulerStateText = "Scheduler paused"
	}
	schedulerState := widget.NewLabel(schedulerStateText)
	stopAllButton := widget.NewButtonWithIcon(stopAllText, stopAllIcon, nil)
	stopAllButton.OnTapped = func() {
		// SetGlobalPause flips the pause flag, updates every job's next-run text,
		// and emits the activity record the observer logs. Revert if the save fails.
		schedulerPaused = !schedulerPaused
		if err := svc.SetGlobalPause(schedulerPaused); err != nil {
			schedulerPaused = !schedulerPaused
			dialog.ShowError(err, w)
			return
		}
		if schedulerPaused {
			schedulerState.SetText("Scheduler paused")
			stopAllButton.SetText("Enable auto")
			stopAllButton.SetIcon(theme.MediaPlayIcon())
		} else {
			schedulerState.SetText("Scheduler running")
			stopAllButton.SetText("Disable auto")
			stopAllButton.SetIcon(theme.MediaPauseIcon())
		}
		refreshView()
	}
	pauseButton := widget.NewButtonWithIcon("Pause", theme.MediaPauseIcon(), func() {
		if selected < 0 || selected >= len(jobs) {
			return
		}
		current := jobs[selected]
		if err := svc.SetEnabled(current.ID, !current.Enabled); err != nil {
			dialog.ShowError(err, w)
			return
		}
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
			if selected >= 0 {
				list.Select(app.DisplayIndex(filteredJobs, selected))
			}
			refreshView()
		}, w)
	})

	toolbar := container.NewHBox(addButton, editButton, runButton, pauseButton, deleteButton, layout.NewSpacer())
	// The row sits directly under the tab bar with no AppTabs inset, while the
	// default VBox gap below it is one theme padding — add the same on top so
	// the button is not flush against the tabs.
	globalControls := container.New(
		layout.NewCustomPaddedLayout(theme.Padding(), 0, 0, 0),
		container.NewHBox(stopAllButton, schedulerState, layout.NewSpacer()),
	)
	// The whole filter is one row: caption on the left, view toggle on the right,
	// select filling what is left. The border layout gives both edges their
	// MinSize, so the header is a line shorter than a stacked caption would make it.
	folderCaption := widget.NewLabelWithStyle("Folder", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	filterRow := container.NewBorder(nil, nil, folderCaption, viewButton, folderSelect)
	sidebarHeader := container.NewVBox(globalControls, widget.NewSeparator(), filterRow, toolbar)
	sidebar := container.NewBorder(sidebarHeader, nil, nil, nil, list)

	// A split rather than a Border left slot: the border pinned the sidebar at its
	// MinSize forever, so the user could never trade list width for detail width.
	// The divider lets either pane grow, and neither can be dragged below its own
	// content minimum.
	panel := container.NewHSplit(sidebar, container.NewPadded(dp.container()))
	panel.SetOffset(initialSplitOffset(sidebar.MinSize().Width))
	return panel, refreshView
}
