package ui

import (
	"fmt"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// overlapPolicyInherit is the display label used when a job should inherit the
// global overlap policy. It maps to an empty Job.OverlapPolicy on save.
const overlapPolicyInherit = "(Use global default)"

// showJobDialog opens a create/edit form for a single job. onSave is called
// with the populated job only when the user clicks Save and all fields pass
// validation.
func showJobDialog(w fyne.Window, title string, current job, onSave func(job)) {
	name := widget.NewEntry()
	name.SetPlaceHolder("Nightly backup")
	name.SetText(current.Name)
	folderEntry := widget.NewEntry()
	folderEntry.SetPlaceHolder("Maintenance")
	folderEntry.SetText(current.Folder)
	scheduleEntry := widget.NewEntry()
	scheduleEntry.SetPlaceHolder("@every 1m")
	scheduleEntry.SetText(current.Schedule)
	commandEntry := widget.NewEntry()
	commandEntry.SetPlaceHolder(`C:\Program Files\App\App.exe`)
	commandEntry.SetText(current.Command)
	commandBrowse := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		chooseFile(w, commandEntry)
	})
	commandRow := container.NewBorder(nil, nil, nil, commandBrowse, commandEntry)
	argumentsEntry := widget.NewMultiLineEntry()
	argumentsEntry.SetPlaceHolder(`D:\Local\Jobs\Auto.ffs_batch`)
	argumentsEntry.SetText(current.Arguments)
	startOnly := widget.NewCheck("Start only, do not wait for exit", nil)
	startOnly.SetChecked(current.StartOnly)
	enabled := widget.NewCheck("Enabled", nil)
	enabled.SetChecked(current.Enabled)
	overlapSelect := widget.NewSelect(
		[]string{overlapPolicyInherit, string(domain.OverlapPolicySkip), string(domain.OverlapPolicyQueue)},
		nil,
	)
	overlapSelected := overlapPolicyInherit
	if current.OverlapPolicy != "" {
		overlapSelected = current.OverlapPolicy
	}
	overlapSelect.SetSelected(overlapSelected)

	form := dialog.NewForm(
		title,
		"Save",
		"Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Name", name),
			widget.NewFormItem("Folder", folderEntry),
			widget.NewFormItem("Schedule", scheduleEntry),
			widget.NewFormItem("Command", commandRow),
			widget.NewFormItem("Arguments", argumentsEntry),
			widget.NewFormItem("", startOnly),
			widget.NewFormItem("Overlap policy", overlapSelect),
			widget.NewFormItem("", enabled),
		},
		func(saved bool) {
			if !saved {
				return
			}
			if strings.TrimSpace(name.Text) == "" || strings.TrimSpace(scheduleEntry.Text) == "" || strings.TrimSpace(commandEntry.Text) == "" {
				// These three fields are the minimum executable job definition.
				// Folder is optional because ungrouped jobs are a supported workflow.
				dialog.ShowError(fmt.Errorf("name, schedule, and command are required"), w)
				return
			}
			if err := domain.Validate(strings.TrimSpace(scheduleEntry.Text)); err != nil {
				dialog.ShowError(fmt.Errorf("invalid schedule: %w", err), w)
				return
			}
			current.Name = strings.TrimSpace(name.Text)
			current.Folder = strings.TrimSpace(folderEntry.Text)
			current.Schedule = strings.TrimSpace(scheduleEntry.Text)
			current.Command = strings.TrimSpace(commandEntry.Text)
			current.Arguments = strings.TrimSpace(argumentsEntry.Text)
			current.StartOnly = startOnly.Checked
			current.Enabled = enabled.Checked
			current.OverlapPolicy = overlapSelect.Selected
			if current.OverlapPolicy == overlapPolicyInherit {
				current.OverlapPolicy = ""
			}
			// The dialog only edits durable configuration. Runtime status is
			// initialized (new jobs) or updated (edits) by the caller against the
			// runtime map, keyed by job ID.
			onSave(current)
		},
		w,
	)
	form.Resize(fyne.NewSize(640, 460))
	form.Show()
}
