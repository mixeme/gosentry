package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// textWidth measures how wide s renders at the theme's current body text size.
func textWidth(s string) float32 {
	return fyne.MeasureText(s, theme.TextSize(), fyne.TextStyle{}).Width
}

// cellPadding is the horizontal space a table cell reserves around its text.
// It replaces a hand-tuned pixel constant with the theme's own inner padding
// doubled (one side each), so it follows text size and DPI.
func cellPadding() float32 { return 2 * theme.InnerPadding() }

// textColumnMinWidth/textColumnMaxWidth bound every content-measured History
// column: the minimum keeps a column readable when its values are short or
// absent, the maximum stops one very long value from dominating the table
// (the table still scrolls horizontally past it). Expressed as measured text
// rather than raw pixels so both follow the theme instead of drifting from it.
func textColumnMinWidth() float32 { return textWidth(strings.Repeat("0", 10)) + cellPadding() }
func textColumnMaxWidth() float32 { return textWidth(strings.Repeat("0", 30)) + cellPadding() }

// textColumnWidth measures the widest of samples so a table column can be
// sized to fit its content, clamped to [min, max]. Fyne tables do not
// auto-size columns, so without this a fixed width clips values like
// "20260601-100000_SomeJobName.log" in the Log column.
func textColumnWidth(samples []string, min, max float32) float32 {
	width := min
	for _, text := range samples {
		if text == "" {
			continue
		}
		if w := textWidth(text) + cellPadding(); w > width {
			width = w
		}
	}
	if width > max {
		width = max
	}
	return width
}

// historyTriggerSamples is the closed set of Trigger values History ever
// shows (see newEvent and app.operations.go/app.run.go, which produce "UI",
// "Manual" and "Schedule"; historyCellText falls back to "Unknown"). Add a new
// trigger here too if one is introduced there, or the column may clip it.
var historyTriggerSamples = []string{"Schedule", "Manual", "UI", "Unknown"}

// historyStateSamples is the closed set of State values History ever shows:
// "OK" and "Failed" come from runner.RunJob (runStateDetail/startJobOnly);
// "Started", "Error" and "Jobs loaded" are recorded directly in mainwindow.go.
// Add a new state here too if one is introduced in either place.
var historyStateSamples = []string{"OK", "Failed", "Started", "Error", "Jobs loaded"}

// historyTimeSample is the rendered form of the timestamp layout every event
// uses (see newEvent), so the Time column needs no content scan: its width is
// fixed by the format string.
const historyTimeSample = "2026-01-02 15:04:05"

// historyColumnWidths computes every column's width from the current sorted
// rows. Time, Trigger and State are fixed-shape or closed-set columns; Job,
// Detail and Log are free text, so their width tracks the values actually
// present, bounded the same way the Log column always was.
func historyColumnWidths(rows []event) [6]float32 {
	var content [3][]string
	for i := range content {
		content[i] = make([]string, 0, len(rows))
	}
	for _, current := range rows {
		for i, value := range historyContentValues(current) {
			content[i] = append(content[i], value)
		}
	}
	min, max := textColumnMinWidth(), textColumnMaxWidth()
	widths := [6]float32{
		0: textWidth(historyTimeSample) + cellPadding(),
		1: textColumnWidth(historyTriggerSamples, min, max),
		3: textColumnWidth(historyStateSamples, min, max),
	}
	for i, col := range historyContentCols {
		widths[col] = textColumnWidth(content[i], min, max)
	}
	return widths
}

// historyContentCols are the columns whose width follows the values actually
// present, in the order historyContentValues returns them. Both the
// incremental fold in add and the full scan in historyColumnWidths go through
// this pair, so they cannot disagree about which columns follow content.
var historyContentCols = [3]int{2, 4, 5}

func historyContentValues(record event) [3]string {
	return [3]string{record.JobName, record.Detail, logFileName(record.LogFile)}
}
