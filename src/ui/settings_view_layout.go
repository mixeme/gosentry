package ui

import (
	"runtime"

	"gitea.mixdep.ru/mix/gosentry/src/app"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// settingsFormFields groups every widget the Settings tab's two-column layout
// arranges. It exists so newSettingsLayout takes one argument instead of
// twenty, and so a widget added in settingsView is added to exactly one
// struct literal rather than threaded through a long parameter list.
type settingsFormFields struct {
	startOnLogin        *widget.Check
	autostartStatus     *widget.Label
	minimizeToTray      *widget.Check
	notifications       *widget.Check
	themeSelect         *widget.Select
	executionModeSelect *widget.Select
	overlapPolicySelect *widget.Select
	defaultTimeout      *widget.Entry
	configPath          string
	jobsFile            *widget.Entry
	jobsFileBrowse      *widget.Button
	logsDir             *widget.Entry
	logsDirOpen         *widget.Button
	logsDirBrowse       *widget.Button
	maxLogFiles         *widget.Entry
	maxLogAgeDays       *widget.Entry
	saveSettings        *widget.Button
	cancelSettings      *widget.Button
	restoreDefaults     *widget.Button
	settingsStatus      *widget.Label
}

// newSettingsLayout assembles the Settings tab from its fields: a two-column
// grid (Application/Queue on the left, Storage/About on the right) with the
// Save/Cancel/Defaults row below.
func newSettingsLayout(f settingsFormFields) fyne.CanvasObject {
	// capW is measured once from every caption in the tab so all rows —
	// Application, Queue, Storage, About — share one value column start,
	// instead of each settingsRow call re-measuring its own single caption.
	capW := captionColumnWidth(settingsCaptions...)

	// The form is split into two columns so a wide window uses its horizontal
	// space instead of stretching into one tall strip. The left column holds the
	// toggles (Application, Queue); the right holds the editable Storage fields and
	// the read-only About block. Save spans the full width below both columns.
	leftColumn := container.NewVBox(
		settingsSection("Application", rowOverlap(),
			settingsRow(capW, "Autostart", f.startOnLogin),
			// Autostart status sits on its own row, aligned under the checkbox via an
			// empty caption, so the Application section fits in a half-width column.
			settingsRow(capW, "", f.autostartStatus),
			settingsRow(capW, "Tray", f.minimizeToTray),
			settingsRow(capW, "Notifications", f.notifications),
			settingsRow(capW, "Theme", f.themeSelect),
		),
		widget.NewSeparator(),
		// Queue used to inline its own container.NewVBox at the theme's default
		// spacing; settingsSection now takes that spacing explicitly so both
		// idioms for "a titled block of rows" collapse into one constructor.
		settingsSection("Queue", theme.Padding(),
			settingsRow(capW, "Execution mode", f.executionModeSelect),
			settingsRow(capW, "Default overlap policy", f.overlapPolicySelect),
			settingsRow(capW, "Default timeout (s)", f.defaultTimeout),
		),
	)
	// Truncating keeps a long config path from forcing the Settings tab's
	// minimum width to track the path length instead of the layout itself.
	configPathLabel := widget.NewLabel(f.configPath)
	configPathLabel.Truncation = fyne.TextTruncateClip
	rightColumn := container.NewVBox(
		settingsSection("Storage", theme.Padding(),
			settingsRow(capW, "Config JSON", configPathLabel),
			settingsRow(capW, "Jobs file", container.NewBorder(nil, nil, nil, f.jobsFileBrowse, f.jobsFile)),
			// Browse stays rightmost so it lines up with the Jobs file row
			// above it; Open sits between it and the path it opens.
			settingsRow(capW, "Logs directory", container.NewBorder(nil, nil, nil, container.NewHBox(f.logsDirOpen, f.logsDirBrowse), f.logsDir)),
			settingsRow(capW, "Max log files", f.maxLogFiles),
			settingsRow(capW, "Max log age days", f.maxLogAgeDays),
		),
		widget.NewSeparator(),
		settingsSection("About", rowOverlap(),
			settingsRow(capW, "GoSentry", widget.NewLabel(app.Version)),
			settingsRow(capW, "Go", widget.NewLabel(runtime.Version())),
			settingsRow(capW, "Fyne", widget.NewLabel(fyneVersion())),
			settingsRow(capW, "Repository", widget.NewHyperlink(projectRepositoryURL, mustParseURL(projectRepositoryURL))),
		),
	)

	// The two columns sit in a top-aligned grid; Save spans the full width below.
	// Wrapping the whole thing in a vertical scroll keeps its minimum height small
	// so it does not dictate the window's minimum height (AppTabs sizes to the
	// tallest tab) and it scrolls on short 720p screens.
	// The button row sits right below the separator's hairline, which reads as
	// tighter than the other vertical gaps in the tab (those separate whole
	// sections, not a single thin line from a row of buttons). A top pad the
	// height of the default padding closes that gap up to match, and a left pad
	// indents the buttons 4px from the edge the layout promises.
	return container.NewVScroll(container.NewPadded(container.NewVBox(
		container.NewGridWithColumns(2, leftColumn, rightColumn),
		widget.NewSeparator(),
		container.New(
			layout.NewCustomPaddedLayout(2*theme.Padding(), 0, theme.Padding(), 0),
			// Save/Cancel/Defaults share one row with the status so an empty status
			// (the common case) does not leave a blank line above the separator. The
			// status appears beside the buttons once a save reports a result.
			container.NewHBox(f.saveSettings, f.cancelSettings, f.restoreDefaults, f.settingsStatus),
		),
	)))
}

// settingsSection groups a bold header above its rows, using the given
// vertical spacing between them. Application and About pass rowOverlap() so
// the block reads as one compact unit; Queue and Storage pass theme.Padding()
// (the same spacing container.NewVBox would use) so their entry-heavy rows
// keep a visible gap. One constructor for "a titled block of rows" rather
// than two spellings of it.
func settingsSection(title string, spacing float32, rows ...fyne.CanvasObject) fyne.CanvasObject {
	children := make([]fyne.CanvasObject, 0, len(rows)+1)
	children = append(children, widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	children = append(children, rows...)
	return container.New(layout.NewCustomPaddedVBoxLayout(spacing), children...)
}

func settingsRow(captionWidth float32, label string, value fyne.CanvasObject) fyne.CanvasObject {
	caption := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	caption.Truncation = fyne.TextTruncateClip
	captionBox := container.New(minWidthLayout{width: captionWidth}, caption)
	return container.NewBorder(nil, nil, captionBox, nil, value)
}
