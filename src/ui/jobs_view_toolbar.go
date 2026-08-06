package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// newToolbar builds the per-job button row under the folder filter. Every
// handler works from the selected job — never from a row index — and ends in
// refresh, which is what re-reads the Service and redraws the row, the details
// pane, and the list highlight.
func (v *jobsView) newToolbar() fyne.CanvasObject {
	return container.NewHBox(
		v.newAddButton(),
		v.newEditButton(),
		v.newRunButton(),
		v.newPauseButton(),
		v.newDeleteButton(),
		layout.NewSpacer(),
	)
}

func (v *jobsView) newAddButton() *widget.Button {
	return widget.NewButtonWithIcon("New job", theme.ContentAddIcon(), func() {
		blank := job{Schedule: "@every 1m", Command: "echo GoSentry job ran", Enabled: true}
		showJobDialog(v.w, "New job", blank, func(saved job) {
			created, err := v.svc.CreateJob(saved)
			if err != nil {
				dialog.ShowError(err, v.w)
				return
			}
			v.state.sync()
			// The new job may have introduced a folder, so the options are rebuilt
			// before the filter is pointed at it.
			v.rebuildFolders()
			v.state.selectByID(created.ID)
			if target := filterValue(created.Folder); v.state.folder != allFolders && v.state.folder != target {
				// The current filter would hide the job the user just created. Switch
				// to its folder; SetSelected fires OnChanged, which applies the filter
				// and refreshes.
				v.folderSelect.SetSelected(target)
			}
			v.refresh()
		})
	})
}

func (v *jobsView) newEditButton() *widget.Button {
	return widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		current, ok := v.state.selected()
		if !ok {
			return
		}
		showJobDialog(v.w, "Edit job", current, func(saved job) {
			// The ID comes from the job the dialog was opened on, so a list that
			// changed underneath the open dialog cannot redirect the save.
			saved.ID = current.ID
			if err := v.svc.UpdateJob(saved); err != nil {
				dialog.ShowError(err, v.w)
				return
			}
			v.state.sync()
			// An edit can rename the job's folder, add a new one, or empty the last
			// job out of an existing one.
			v.rebuildFolders()
			v.refresh()
		})
	})
}

func (v *jobsView) newRunButton() *widget.Button {
	return widget.NewButtonWithIcon("Run now", theme.MediaPlayIcon(), func() {
		current, ok := v.state.selected()
		if !ok {
			return
		}
		// A manual run is allowed even while the scheduler is paused: pause only
		// stops automatic scheduled runs, not the user's explicit "Run now".
		if err := v.svc.RunNow(current.ID); err != nil {
			dialog.ShowError(err, v.w)
			return
		}
		v.refresh()
	})
}

func (v *jobsView) newPauseButton() *widget.Button {
	return widget.NewButtonWithIcon("Pause", theme.MediaPauseIcon(), func() {
		current, ok := v.state.selected()
		if !ok {
			return
		}
		if err := v.svc.SetEnabled(current.ID, !current.Enabled); err != nil {
			dialog.ShowError(err, v.w)
			return
		}
		v.refresh()
	})
}

func (v *jobsView) newDeleteButton() *widget.Button {
	return widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		deleted, ok := v.state.selected()
		if !ok {
			return
		}
		// Deletion is confirmed because jobs can represent real system actions.
		// There is no undo yet, so accidental removal should require one more click.
		dialog.ShowConfirm("Delete job", fmt.Sprintf("Delete %q?", deleted.Name), func(confirm bool) {
			if !confirm {
				return
			}
			if err := v.svc.DeleteJob(deleted.ID); err != nil {
				dialog.ShowError(err, v.w)
				return
			}
			// sync drops the deleted job's selection and falls back to the first row
			// the filter still shows.
			v.state.sync()
			v.rebuildFolders()
			if len(v.state.filtered) == 0 && v.state.folder != allFolders {
				// The deleted job was the last one in its folder, and that folder is
				// no longer an option. Fall back to "All" rather than leaving the user
				// on an empty filter they did not choose.
				v.folderSelect.SetSelected(allFolders)
			}
			v.refresh()
		}, v.w)
	})
}
