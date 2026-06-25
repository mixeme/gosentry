package ui

import (
	"net/url"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// settingsLabelWidth is wide enough to show the longest caption ("Default
// overlap policy") in full; the captions truncate, so a narrower width would
// clip it. All rows share this width so their value controls stay aligned.
const settingsLabelWidth float32 = 180
const settingsControlWidth float32 = 330
const projectRepositoryURL = "https://gitea.mixdep.ru/mix/gosentry"

// settingsRowSpacing is the (negative) gap between rows of the settings form,
// overlapping each control's built-in vertical padding so the column is tighter
// and more compact, matching the condensed job details panel.
const settingsRowSpacing float32 = -6

func settingsView(w fyne.Window, svc *app.Service) fyne.CanvasObject {
	store := svc.Store()
	// updateSaveState compares the form to the saved config and enables Save only
	// when something differs. It is defined below (once Save and every field
	// exist) but declared here so the field change handlers can reference it.
	var updateSaveState func()
	startOnLogin := widget.NewCheck("Start on login", nil)
	startOnLogin.SetChecked(store.Config.StartOnLogin)
	autostartStatus := widget.NewLabel("")
	refreshAutostartStatus := func() {
		ok, message := svc.AutostartStatus()
		if ok {
			autostartStatus.SetText("OK: " + message)
			return
		}
		autostartStatus.SetText("Problem: " + message)
	}
	startOnLogin.OnChanged = func(bool) {
		if startOnLogin.Checked != store.Config.StartOnLogin {
			autostartStatus.SetText("Pending: save settings to apply")
		} else {
			refreshAutostartStatus()
		}
		updateSaveState()
	}
	refreshAutostartStatus()
	minimizeToTray := widget.NewCheck("Keep running in the system tray", nil)
	minimizeToTray.SetChecked(store.Config.KeepRunningInTray)
	minimizeToTray.OnChanged = func(bool) { updateSaveState() }
	notifications := widget.NewCheck("Show desktop notifications for failed jobs", nil)
	notifications.SetChecked(store.Config.NotifyOnFailure)
	notifications.OnChanged = func(bool) { updateSaveState() }
	executionModeSelect := widget.NewSelect(
		[]string{string(domain.ExecutionModeParallel), string(domain.ExecutionModeSequential)},
		nil,
	)
	executionModeSelect.SetSelected(string(store.Config.ExecutionMode))
	executionModeSelect.OnChanged = func(string) { updateSaveState() }
	overlapPolicySelect := widget.NewSelect(
		[]string{string(domain.OverlapPolicySkip), string(domain.OverlapPolicyQueue)},
		nil,
	)
	overlapPolicySelect.SetSelected(string(store.Config.OverlapPolicy))
	overlapPolicySelect.OnChanged = func(string) { updateSaveState() }
	jobsDir := widget.NewEntry()
	jobsDir.SetText(store.Config.JobsDir)
	jobsDir.OnChanged = func(string) { updateSaveState() }
	jobsDirBrowse := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		chooseFolder(w, jobsDir)
	})
	logsDir := widget.NewEntry()
	logsDir.SetText(store.Config.LogsDir)
	logsDir.OnChanged = func(string) { updateSaveState() }
	logsDirBrowse := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		chooseFolder(w, logsDir)
	})
	maxLogFiles := widget.NewEntry()
	maxLogFiles.SetText(strconv.Itoa(store.Config.MaxLogFiles))
	maxLogFiles.OnChanged = func(string) { updateSaveState() }
	maxLogAgeDays := widget.NewEntry()
	maxLogAgeDays.SetText(strconv.Itoa(store.Config.MaxLogAgeDays))
	maxLogAgeDays.OnChanged = func(string) { updateSaveState() }
	// Autostart status sits on its own row beneath the checkbox (rather than
	// beside it) so the Application section fits within a half-width column.
	// Truncating keeps a long status message from forcing the column wider.
	autostartStatus.Wrapping = fyne.TextTruncate
	settingsStatus := widget.NewLabel("")

	saveSettings := widget.NewButtonWithIcon("Save settings", theme.DocumentSaveIcon(), func() {
		files, err := strconv.Atoi(strings.TrimSpace(maxLogFiles.Text))
		if err != nil || files <= 0 {
			settingsStatus.SetText("Max log files must be a positive number")
			return
		}
		days, err := strconv.Atoi(strings.TrimSpace(maxLogAgeDays.Text))
		if err != nil || days <= 0 {
			settingsStatus.SetText("Max log age days must be a positive number")
			return
		}
		if strings.TrimSpace(jobsDir.Text) == "" {
			settingsStatus.SetText("Jobs directory is required")
			return
		}
		if strings.TrimSpace(logsDir.Text) == "" {
			settingsStatus.SetText("Logs directory is required")
			return
		}
		// Build the new config from the form and hand it to the Service, which
		// validates it, persists config and jobs to the (possibly new) directory,
		// and runs log cleanup so tightened retention limits take effect at once.
		config := store.Config
		config.JobsDir = strings.TrimSpace(jobsDir.Text)
		config.LogsDir = strings.TrimSpace(logsDir.Text)
		config.MaxLogFiles = files
		config.MaxLogAgeDays = days
		config.StartOnLogin = startOnLogin.Checked
		config.KeepRunningInTray = minimizeToTray.Checked
		config.NotifyOnFailure = notifications.Checked
		config.ExecutionMode = domain.ExecutionMode(executionModeSelect.Selected)
		config.OverlapPolicy = domain.OverlapPolicy(overlapPolicySelect.Selected)
		if err := svc.UpdateSettings(config); err != nil {
			settingsStatus.SetText("Save failed: " + err.Error())
			return
		}
		if err := svc.ApplyAutostart(); err != nil {
			refreshAutostartStatus()
			settingsStatus.SetText("Saved, autostart failed: " + err.Error())
			return
		}
		refreshAutostartStatus()
		settingsStatus.SetText("Saved")
		// The form now matches the persisted config, so disable Save again.
		updateSaveState()
	})

	// Save stays disabled until a field differs from the saved config, so the
	// button only invites a click when there is something to persist. The numeric
	// fields compare against their canonical string form; any unparsable text
	// counts as a change so the user can click Save and see the validation error.
	updateSaveState = func() {
		c := store.Config
		changed := startOnLogin.Checked != c.StartOnLogin ||
			minimizeToTray.Checked != c.KeepRunningInTray ||
			notifications.Checked != c.NotifyOnFailure ||
			executionModeSelect.Selected != string(c.ExecutionMode) ||
			overlapPolicySelect.Selected != string(c.OverlapPolicy) ||
			strings.TrimSpace(jobsDir.Text) != c.JobsDir ||
			strings.TrimSpace(logsDir.Text) != c.LogsDir ||
			strings.TrimSpace(maxLogFiles.Text) != strconv.Itoa(c.MaxLogFiles) ||
			strings.TrimSpace(maxLogAgeDays.Text) != strconv.Itoa(c.MaxLogAgeDays)
		if changed {
			saveSettings.Enable()
		} else {
			saveSettings.Disable()
		}
	}
	updateSaveState()

	// The form is split into two columns so a wide window uses its horizontal
	// space instead of stretching into one tall strip. The left column holds the
	// toggles (Application, Queue); the right holds the editable Storage fields and
	// the read-only About block. Save spans the full width below both columns.
	leftColumn := container.NewVBox(
		settingsSection("Application",
			settingsRow("Autostart", container.New(minWidthLayout{width: settingsControlWidth}, startOnLogin)),
			// Autostart status sits on its own row, aligned under the checkbox via an
			// empty caption, so the Application section fits in a half-width column.
			settingsRow("", autostartStatus),
			settingsRow("Tray", container.New(minWidthLayout{width: settingsControlWidth}, minimizeToTray)),
			settingsRow("Notifications", container.New(minWidthLayout{width: settingsControlWidth}, notifications)),
		),
		widget.NewSeparator(),
		// Queue holds the execution mode and overlap policy comboboxes. Like
		// Storage, it uses the default VBox spacing (not the condensed section
		// layout) so the comboboxes keep a visible gap between them.
		container.NewVBox(
			widget.NewLabelWithStyle("Queue", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			settingsRow("Execution mode", container.New(minWidthLayout{width: settingsControlWidth}, executionModeSelect)),
			settingsRow("Default overlap policy", container.New(minWidthLayout{width: settingsControlWidth}, overlapPolicySelect)),
		),
	)
	rightColumn := container.NewVBox(
		// Storage holds editable entry fields. It uses the default VBox spacing
		// (not the condensed section layout) so the entry boxes keep a visible
		// gap between them instead of merging into one block.
		container.NewVBox(
			widget.NewLabelWithStyle("Storage", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			settingsRow("Config YAML", widget.NewLabel(store.Paths.ConfigPath)),
			settingsRow("Jobs directory", container.NewBorder(nil, nil, nil, jobsDirBrowse, jobsDir)),
			settingsRow("Logs directory", container.NewBorder(nil, nil, nil, logsDirBrowse, logsDir)),
			settingsRow("Max log files", maxLogFiles),
			settingsRow("Max log age days", maxLogAgeDays),
		),
		widget.NewSeparator(),
		settingsSection("About",
			settingsRow("GoSentry", widget.NewLabel(app.Version)),
			settingsRow("Go", widget.NewLabel(runtime.Version())),
			settingsRow("Fyne", widget.NewLabel(fyneVersion())),
			settingsRow("Repository", widget.NewHyperlink(projectRepositoryURL, mustParseURL(projectRepositoryURL))),
		),
	)

	// The two columns sit in a top-aligned grid; Save spans the full width below.
	// Wrapping the whole thing in a vertical scroll keeps its minimum height small
	// so it does not dictate the window's minimum height (AppTabs sizes to the
	// tallest tab) and it scrolls on short 720p screens.
	return container.NewVScroll(container.NewPadded(container.NewVBox(
		container.NewGridWithColumns(2, leftColumn, rightColumn),
		widget.NewSeparator(),
		// Save button and its status share one row so an empty status (the common
		// case) does not leave a blank line above the separator. The status appears
		// beside the button once a save reports a result.
		container.NewHBox(saveSettings, settingsStatus),
	)))
}

// settingsSection groups a bold header above its rows using the tight
// settingsRowSpacing so a block of label rows reads as one compact unit. The
// caller keeps separators and entry-heavy sections in the surrounding VBox so
// they retain the theme's normal spacing.
func settingsSection(title string, rows ...fyne.CanvasObject) fyne.CanvasObject {
	children := make([]fyne.CanvasObject, 0, len(rows)+1)
	children = append(children, widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	children = append(children, rows...)
	return container.New(compactVBoxLayout{spacing: settingsRowSpacing}, children...)
}

func fyneVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dependency := range info.Deps {
		if dependency.Path == "fyne.io/fyne/v2" {
			if dependency.Replace != nil && dependency.Replace.Version != "" {
				return dependency.Replace.Version
			}
			if dependency.Version != "" {
				return dependency.Version
			}
			return "local"
		}
	}
	return "unknown"
}

func mustParseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		return &url.URL{}
	}
	return parsed
}

func chooseFile(w fyne.Window, target *widget.Entry) {
	fileDialog := dialog.NewFileOpen(func(uri fyne.URIReadCloser, err error) {
		if err != nil || uri == nil {
			return
		}
		target.SetText(uri.URI().Path())
	}, w)
	fileDialog.Resize(fyne.NewSize(900, 640))
	fileDialog.Show()
}

func chooseFolder(w fyne.Window, target *widget.Entry) {
	folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		target.SetText(uri.Path())
	}, w)
	// The default folder picker can be cramped on Windows. A larger size makes
	// long paths readable and avoids forcing the user to resize it every time.
	folderDialog.Resize(fyne.NewSize(900, 640))
	folderDialog.Show()
}

func settingsRow(label string, value fyne.CanvasObject) fyne.CanvasObject {
	caption := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	caption.Wrapping = fyne.TextTruncate
	captionBox := container.New(minWidthLayout{width: settingsLabelWidth}, caption)
	return container.NewBorder(nil, nil, captionBox, nil, value)
}

