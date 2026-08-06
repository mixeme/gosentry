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

// newList builds the sidebar's job list. Rows are drawn from jobsViewState's
// filtered view, so the row index the widget reports is a position in the
// filter, never an index into the job snapshot.
func (v *jobsView) newList() *widget.List {
	list := widget.NewList(
		func() int { return len(v.state.filtered) },
		func() fyne.CanvasObject {
			name := widget.NewLabelWithStyle("Job name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			// Truncating stops a long name from pushing the compact row's status
			// off the right-hand edge. Labels default to TextWrapOff, which grows
			// the widget to fit instead.
			name.Truncation = fyne.TextTruncateClip
			inlineStatus := widget.NewLabel("status")
			meta := widget.NewLabel("schedule")
			status := widget.NewLabel("status")
			v.applyRowMode(inlineStatus, meta, status)
			nameLine := container.NewBorder(nil, nil, nil, inlineStatus, name)
			return container.New(layout.NewCustomPaddedVBoxLayout(rowOverlap()), nameLine, meta, status)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			current, ok := v.state.jobAt(int(id))
			if !ok {
				return
			}
			row := item.(*fyne.Container)
			// NewBorder keeps the center object first and appends the border slots
			// after it, so nameLine is [name, inlineStatus].
			nameLine := row.Objects[0].(*fyne.Container)
			name := nameLine.Objects[0].(*widget.Label)
			inlineStatus := nameLine.Objects[1].(*widget.Label)
			meta := row.Objects[1].(*widget.Label)
			status := row.Objects[2].(*widget.Label)

			name.SetText(current.Name)
			// Keep each row compact: folder, schedule, and command are shown in one
			// metadata line so the left pane stays useful even with many jobs.
			meta.SetText(app.DisplayFolder(current.Folder) + "    " + current.Schedule + "    " + app.DisplayInvocation(current))
			statusText := app.StatusText(current, v.state.runtime(current.ID))
			status.SetText(statusText)
			inlineStatus.SetText(statusText)
			// A full Refresh reuses rows built under the previous mode, so
			// visibility cannot be left to the create callback alone.
			v.applyRowMode(inlineStatus, meta, status)
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		v.state.selectRow(int(id))
		v.updateDetails()
	}
	return list
}

// applyRowMode expresses the current view mode as visibility on the row's
// four labels. widget.List caches the row template's MinSize, and
// list.Refresh() re-creates the template and recomputes it, so hiding lines
// is what actually shrinks the rows: layout.NewCustomPaddedVBoxLayout and the
// border layout both skip hidden children when measuring.
func (v *jobsView) applyRowMode(inlineStatus, meta, status fyne.CanvasObject) {
	if v.listView.IsCompact() {
		inlineStatus.Show()
		meta.Hide()
		status.Hide()
		return
	}
	inlineStatus.Hide()
	meta.Show()
	status.Show()
}

// newViewToggle builds the compact/detailed switch that sits at the right edge
// of the filter row.
func (v *jobsView) newViewToggle() *widget.Button {
	button := widget.NewButtonWithIcon(viewToggleText(v.listView), viewToggleIcon(v.listView), nil)
	button.OnTapped = func() {
		next := nextJobListView(v.listView)
		v.listView = next
		if err := v.svc.SetJobListView(next); err != nil {
			// Roll the mode back and leave the button as it was, so the button
			// never claims a preference that did not reach disk.
			v.listView = nextJobListView(next)
			dialog.ShowError(err, v.w)
			return
		}
		button.SetText(viewToggleText(v.listView))
		button.SetIcon(viewToggleIcon(v.listView))
		// Refresh re-creates the row template, which is what recomputes the
		// cached row height for the new mode. Selection is untouched.
		v.list.Refresh()
	}
	return button
}

// viewToggleIcon pairs with viewToggleText: both name the action the button
// performs, not the state it is in, matching stopAllButton's convention.
func viewToggleIcon(current domain.JobListView) fyne.Resource {
	if current.IsCompact() {
		return theme.ViewFullScreenIcon()
	}
	return theme.ListIcon()
}
