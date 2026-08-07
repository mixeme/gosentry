package ui

import (
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

// jobsView owns the Jobs tab: the widgets, the view-only preferences they draw
// (list mode and the scheduler pause label), and the jobsViewState the widgets
// read. It replaces a single constructor whose dozen closures shared seven
// mutable locals — the state each handler touches is now named on the struct
// rather than captured, and the invariants that used to be maintained by hand in
// five places live on jobsViewState.
type jobsView struct {
	w     fyne.Window
	svc   *app.Service
	state *jobsViewState
	dp    *detailsPanel

	list           *widget.List
	folderSelect   *widget.Select
	viewButton     *widget.Button
	stopAllButton  *widget.Button
	schedulerState *widget.Label

	// listView and paused mirror Service-owned config so the widgets can be
	// relabelled without a round trip. Both are re-read from the Service on every
	// refresh; neither is a source of truth.
	listView domain.JobListView
	paused   bool
}

// newJobsView builds the Jobs tab: list sidebar, details panel, and toolbar.
// It returns the assembled panel and a refresh function the caller invokes
// whenever the service state may have changed (e.g., from the event subscriber
// in mainwindow.go). The refresh function re-reads the service snapshot and
// redraws all widgets in the jobs view; it does NOT touch history or settings.
func newJobsView(w fyne.Window, svc *app.Service) (fyne.CanvasObject, func()) {
	config := svc.Config()
	v := &jobsView{
		w:        w,
		svc:      svc,
		state:    newJobsViewState(svc),
		listView: config.JobListView,
		paused:   config.Paused,
	}
	v.dp = newDetailsPanel(job{}, &domain.JobRuntime{}, config.OverlapPolicy, config.DefaultTimeoutSeconds)
	v.updateDetails()

	// Build order follows what refresh() touches: the folder select fires its
	// OnChanged from SetSelected below, which refreshes, so every widget that
	// refresh() reaches has to exist by then.
	v.list = v.newList()
	v.viewButton = v.newViewToggle()
	globalControls := v.newGlobalControls()
	v.folderSelect = v.newFolderSelect()
	v.folderSelect.SetSelected(v.state.folder)
	v.syncListSelection()

	return v.assemble(globalControls), v.refresh
}

// refresh re-reads the Service and redraws the whole view. It is the single
// entry point for "something changed": the toolbar handlers call it after a
// successful operation, and mainwindow's event observer calls it for everything
// else.
func (v *jobsView) refresh() {
	v.state.sync()
	// The pause state is Service-owned and can change from outside this view, so
	// it is re-read here rather than mirrored from the tap handler alone — that is
	// what makes this view a consumer of SchedulerStateChanged.
	v.applySchedulerState(v.svc.Config().Paused)
	// updateDetails already ends in a d.logs.Refresh() (both its update and clear
	// paths do), so refreshing the activity list again here would redraw it twice
	// per call.
	v.updateDetails()
	v.list.Refresh()
	v.syncListSelection()
}

// updateDetails repopulates the details pane from the current selection.
func (v *jobsView) updateDetails() {
	current, ok := v.state.selected()
	if !ok {
		// A folder filter can temporarily leave no selectable rows. Clearing the
		// details panel avoids showing stale information for a hidden job.
		v.dp.clear()
		return
	}
	// Overlap policy and the default timeout are global settings that can change
	// from the Settings tab while this view is open, so they are re-read on every
	// update rather than captured once at construction.
	config := v.svc.Config()
	v.dp.update(current, v.state.runtime(current.ID), config.OverlapPolicy, config.DefaultTimeoutSeconds)
}

// syncListSelection points the list's highlight at the selected job. It is what
// keeps the highlight and the details pane describing the same job when the row
// a job sits in moves — a job created or deleted above it, a folder filter
// applied, or a different jobs file adopted. widget.List.Select returns early
// when the row is already highlighted, so calling this on every refresh does not
// fight the user's scrolling.
func (v *jobsView) syncListSelection() {
	row := v.state.displayRow()
	if row < 0 {
		v.list.UnselectAll()
		return
	}
	v.list.Select(row)
}

// rebuildFolders re-derives the folder filter's options from the current jobs.
// Creating, editing, and deleting a job can all add or remove a folder.
func (v *jobsView) rebuildFolders() {
	v.folderSelect.Options = folderOptions(v.state.jobs)
	v.folderSelect.Refresh()
}

// assemble puts the sidebar (global controls, folder filter, toolbar, list) and
// the details pane into the master/detail split the tab shows.
func (v *jobsView) assemble(globalControls fyne.CanvasObject) fyne.CanvasObject {
	// The whole filter is one row: caption on the left, view toggle on the right,
	// select filling what is left. The border layout gives both edges their
	// MinSize, so the header is a line shorter than a stacked caption would make it.
	folderCaption := widget.NewLabelWithStyle("Folder", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	filterRow := container.NewBorder(nil, nil, folderCaption, v.viewButton, v.folderSelect)
	sidebarHeader := container.NewVBox(globalControls, widget.NewSeparator(), filterRow, v.newToolbar())
	sidebar := container.NewBorder(sidebarHeader, nil, nil, nil, v.list)

	// A split rather than a Border left slot: the border pinned the sidebar at its
	// MinSize forever, so the user could never trade list width for detail width.
	// The divider lets either pane grow, and neither can be dragged below its own
	// content minimum.
	panel := container.NewHSplit(sidebar, container.NewPadded(v.dp.container()))
	panel.SetOffset(initialSplitOffset(sidebar.MinSize().Width))
	return panel
}

// newFolderSelect builds the folder filter. Selecting a folder narrows the list
// and, when the selected job is no longer visible, moves the selection to the
// first row that is (see jobsViewState.applyFilter).
func (v *jobsView) newFolderSelect() *widget.Select {
	return widget.NewSelect(folderOptions(v.state.jobs), func(value string) {
		if value == "" {
			return
		}
		v.state.applyFilter(value)
		v.refresh()
	})
}

// newGlobalControls builds the pause control row that sits above the filter.
func (v *jobsView) newGlobalControls() fyne.CanvasObject {
	v.schedulerState = widget.NewLabel("")
	v.stopAllButton = widget.NewButtonWithIcon("", nil, nil)
	v.applySchedulerState(v.paused)
	v.stopAllButton.OnTapped = func() {
		// SetGlobalPause flips the pause flag, updates every job's next-run text,
		// and emits the activity record the observer logs. refresh re-derives the
		// pause state from the Service, so a failed save leaves the control showing
		// what actually happened.
		if err := v.svc.SetGlobalPause(!v.paused); err != nil {
			dialog.ShowError(err, v.w)
			return
		}
		v.refresh()
	}
	// The row sits directly under the tab bar with no AppTabs inset, while the
	// default VBox gap below it is one theme padding — add the same on top so
	// the button is not flush against the tabs.
	return container.New(
		layout.NewCustomPaddedLayout(theme.Padding(), 0, 0, 0),
		container.NewHBox(v.stopAllButton, v.schedulerState, layout.NewSpacer()),
	)
}

// applySchedulerState is the one place that draws the pause control and its
// status text from a pause value, so refresh can drive it from whatever the
// Service reports instead of only the tap handler mirroring its own toggle.
func (v *jobsView) applySchedulerState(paused bool) {
	v.paused = paused
	if paused {
		v.schedulerState.SetText("Scheduler paused")
		v.stopAllButton.SetText("Enable auto")
		v.stopAllButton.SetIcon(theme.MediaPlayIcon())
		return
	}
	v.schedulerState.SetText("Scheduler running")
	v.stopAllButton.SetText("Disable auto")
	v.stopAllButton.SetIcon(theme.MediaPauseIcon())
}
