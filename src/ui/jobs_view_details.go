package ui

import (
	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// detailsPanel holds all widgets in the job details pane and knows how to
// assemble, populate, and clear them. Extracting it here keeps newJobsView
// focused on list, toolbar, and layout wiring without embedding 100+ lines of
// widget construction and update logic.
type detailsPanel struct {
	title         *widget.Label
	folder        *widget.Label
	schedule      *widget.Label
	command       *widget.Label
	arguments     *widget.Label
	runMode       *widget.Label
	overlapPolicy *widget.Label
	lastRun       *widget.Label
	nextRun       *widget.Label
	state         *widget.Label
	stats         *widget.Label

	commandOutput       *widget.TextGrid
	commandOutputScroll *container.Scroll

	logs         *widget.List
	selectedLogs []event
}

func newDetailsPanel(firstJob job, rt *domain.JobRuntime, globalOverlapPolicy domain.OverlapPolicy) *detailsPanel {
	d := &detailsPanel{
		title:         widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		folder:        newJobDetailLabel(""),
		schedule:      newJobDetailLabel(""),
		command:       newJobDetailLabel(""),
		arguments:     newJobDetailLabel(""),
		runMode:       newJobDetailLabel(""),
		overlapPolicy: newJobDetailLabel(""),
		lastRun:       newJobDetailLabel(""),
		nextRun:       newJobDetailLabel(""),
		state:         newJobDetailLabel(""),
		stats:         newJobDetailLabel(""),
		commandOutput: widget.NewTextGrid(),
	}
	d.title.Wrapping = fyne.TextWrapBreak
	d.commandOutputScroll = container.NewScroll(d.commandOutput)
	// Command output can contain long lines and preserved whitespace. TextGrid is
	// used instead of Label so stdout/stderr remains readable and does not vanish
	// against the theme when it is placed inside a scroll container.
	d.commandOutputScroll.SetMinSize(fyne.NewSize(460, 120))
	d.logs = widget.NewList(
		func() int { return len(d.selectedLogs) },
		func() fyne.CanvasObject {
			l := widget.NewLabel("log")
			l.Wrapping = fyne.TextTruncate
			return l
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			item.(*widget.Label).SetText(app.EventLine(d.selectedLogs[id]))
		},
	)
	d.update(firstJob, rt, globalOverlapPolicy)
	return d
}

func (d *detailsPanel) update(j job, rt *domain.JobRuntime, globalOverlapPolicy domain.OverlapPolicy) {
	d.title.SetText(j.Name)
	d.folder.SetText(app.DisplayFolder(j.Folder))
	d.schedule.SetText(j.Schedule)
	d.command.SetText(j.Command)
	d.arguments.SetText(app.DisplayArguments(j.Arguments))
	d.runMode.SetText(app.DisplayRunMode(j))
	d.overlapPolicy.SetText(app.DisplayOverlapPolicy(j, globalOverlapPolicy))
	d.lastRun.SetText(rt.LastRun)
	d.nextRun.SetText(rt.NextRun)
	d.state.SetText(rt.LastState)
	d.stats.SetText(app.DisplayStats(rt))
	d.commandOutput.SetText(rt.Output)
	d.selectedLogs = lastJobLogs(rt.Logs)
}

func (d *detailsPanel) clear() {
	d.title.SetText("No job selected")
	d.folder.SetText("")
	d.schedule.SetText("")
	d.command.SetText("")
	d.arguments.SetText("")
	d.runMode.SetText("")
	d.overlapPolicy.SetText("")
	d.lastRun.SetText("")
	d.nextRun.SetText("")
	d.state.SetText("")
	d.stats.SetText("")
	d.commandOutput.SetText("")
	d.selectedLogs = nil
}

// container assembles the details pane layout: metadata rows pin to the top,
// the activity panel pins to the bottom, and command output fills the remainder.
func (d *detailsPanel) container() fyne.CanvasObject {
	top := container.NewVBox(
		d.title,
		widget.NewSeparator(),
		detailRow("Folder", d.folder),
		detailRow("Schedule", d.schedule),
		detailRow("Command", d.command),
		detailRow("Arguments", d.arguments),
		detailRow("Run mode", d.runMode),
		detailRow("Overlap policy", d.overlapPolicy),
		detailRow("Last run", d.lastRun),
		detailRow("Next run", d.nextRun),
		detailRow("State", d.state),
		detailRow("Statistics", d.stats),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Command output", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)
	activity := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Selected job activity", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.New(fixedHeightLayout{height: jobActivityHeight}, d.logs),
	)
	return container.NewBorder(top, activity, nil, nil, d.commandOutputScroll)
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
