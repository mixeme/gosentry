package ui

import (
	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
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
	// The height here is only a floor: the scroll grows to fill whatever space the
	// border layout gives it, so keep the minimum small so the whole window can be
	// shrunk on short (720p) screens. Long output stays reachable by scrolling.
	d.commandOutputScroll.SetMinSize(fyne.NewSize(460, 70))
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
	// The activity list renders d.selectedLogs but, unlike the labels above whose
	// SetText refreshes them, only its backing slice changed. Refresh it here so
	// switching jobs immediately redraws the panel instead of keeping stale rows.
	d.logs.Refresh()
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
	d.logs.Refresh()
}

// container assembles the details pane layout: metadata rows pin to the top,
// the activity panel pins to the bottom, and command output fills the remainder.
func (d *detailsPanel) container() fyne.CanvasObject {
	// Metadata is laid out in two columns so the block stays half as tall,
	// keeping the details pane usable on 720p screens where a single column of
	// ten rows pushes the minimum window height past the available space.
	capW := detailCaptionWidth()
	rows := container.New(compactVBoxLayout{spacing: detailRowSpacing},
		detailRowPair(capW, "Folder", d.folder, "Schedule", d.schedule),
		detailRowPair(capW, "Command", d.command, "Arguments", d.arguments),
		detailRowPair(capW, "Run mode", d.runMode, "Overlap policy", d.overlapPolicy),
		detailRowPair(capW, "Last run", d.lastRun, "Next run", d.nextRun),
		detailRowPair(capW, "State", d.state, "Statistics", d.stats),
	)
	top := container.NewVBox(
		d.title,
		widget.NewSeparator(),
		rows,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Command output", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)
	activity := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Selected job activity", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.New(fixedHeightLayout{height: activityRowsHeight(maxJobActivityRows)}, d.logs),
	)
	return container.NewBorder(top, activity, nil, nil, d.commandOutputScroll)
}

// activityRowsHeight returns the fixed height needed to show the given number of
// activity-list rows without scrolling. It mirrors widget.List's own content
// height — (itemHeight + padding) per row, less one separator — using the same
// label template the list builds its rows from, so it tracks the theme's text
// size and DPI instead of relying on a hand-tuned constant. The trailing pixel
// absorbs sub-pixel rounding so the last row is never clipped behind a scrollbar.
func activityRowsHeight(rows int) float32 {
	sample := widget.NewLabel("log")
	sample.Wrapping = fyne.TextTruncate
	itemHeight := sample.MinSize().Height
	padding := theme.Padding()
	return (itemHeight+padding)*float32(rows) - padding + 1
}

// detailCaptionWidth returns the width reserved for every metadata caption,
// derived from the widest caption label so the value columns all start at the
// same x and no caption truncates. Measuring a real label keeps it DPI- and
// theme-aware instead of relying on a hand-tuned constant.
func detailCaptionWidth() float32 {
	captions := []string{
		"Folder", "Schedule", "Command", "Arguments", "Run mode",
		"Overlap policy", "Last run", "Next run", "State", "Statistics",
	}
	var width float32
	for _, c := range captions {
		if w := widget.NewLabelWithStyle(c, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}).MinSize().Width; w > width {
			width = w
		}
	}
	return width
}

// detailRowPair places two label/value pairs side by side, producing the
// four-column caption|value|caption|value rows the compact metadata grid uses.
func detailRowPair(captionWidth float32, l1 string, v1 fyne.CanvasObject, l2 string, v2 fyne.CanvasObject) fyne.CanvasObject {
	return container.NewGridWithColumns(2, detailRow(captionWidth, l1, v1), detailRow(captionWidth, l2, v2))
}

func detailRow(captionWidth float32, label string, value fyne.CanvasObject) fyne.CanvasObject {
	caption := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	caption.Wrapping = fyne.TextTruncate
	// A fixed caption width (rather than an even split) means widening the window
	// feeds the extra space to the value, not the short caption.
	return container.New(captionValueLayout{captionWidth: captionWidth}, caption, value)
}

func newJobDetailLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextTruncate
	return label
}
