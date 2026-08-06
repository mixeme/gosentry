package ui

import (
	"strconv"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const projectRepositoryURL = "https://github.com/mixeme/gosentry"

// settingsCaptions lists every settingsRow caption in the tab, in no
// particular order. settingsView measures this once with captionColumnWidth
// so every row's value column starts at the same x; a caption added to a row
// below without being added here is the one way a row would silently fall out
// of alignment.
var settingsCaptions = []string{
	"Autostart", "Tray", "Notifications", "Theme",
	"Execution mode", "Default overlap policy", "Default timeout (s)",
	"Config JSON", "Jobs file", "Logs directory", "Max log files", "Max log age days",
	"GoSentry", "Go", "Fyne", "Repository",
}

func settingsView(w fyne.Window, svc *app.Service) fyne.CanvasObject {
	// saved mirrors the config as last persisted (or freshly loaded at
	// construction); it is a local copy the closures below compare the form
	// against and reassign after a successful save, rather than holding onto
	// the live *storage.Store the Service owns (see app.Service.Config).
	// paths never changes after construction of this view — AppDir and
	// ConfigPath are fixed for the process — so it is read once, not refreshed.
	saved := svc.Config()
	paths := svc.Paths()
	// updateSaveState compares the form to the saved config and enables Save only
	// when something differs. It is defined below (once Save and every field
	// exist) but declared here so the field change handlers can reference it.
	var updateSaveState func()
	// loadFields populates every form control from the given config. It backs
	// both the initial load and the Cancel/Defaults buttons below.
	var loadFields func(domain.Config)
	startOnLogin := widget.NewCheck("Start on login", nil)
	startOnLogin.SetChecked(saved.StartOnLogin)
	minimizeToTray := widget.NewCheck("Keep running in the system tray", nil)
	minimizeToTray.SetChecked(saved.KeepRunningInTray)
	autostartStatus := widget.NewLabel("")
	trayRestartHint := widget.NewLabel("")
	trayRestartHint.Truncation = fyne.TextTruncateClip
	refreshAutostartStatus := func() {
		if settingsPendingAutostart(startOnLogin, minimizeToTray, saved) {
			autostartStatus.SetText("Pending: save settings to apply")
			return
		}
		ok, message := svc.AutostartStatus()
		if ok {
			autostartStatus.SetText("OK: " + message)
			return
		}
		autostartStatus.SetText("Problem: " + message)
	}
	refreshTrayRestartHint := func(pending bool) {
		if pending {
			trayRestartHint.SetText("Pending: restart GoSentry after save for the tray icon change to take effect.")
			return
		}
		trayRestartHint.SetText("")
	}
	startOnLogin.OnChanged = func(bool) {
		refreshAutostartStatus()
		updateSaveState()
	}
	minimizeToTray.OnChanged = func(bool) {
		refreshAutostartStatus()
		refreshTrayRestartHint(minimizeToTray.Checked != saved.KeepRunningInTray)
		updateSaveState()
	}
	refreshAutostartStatus()
	notifications := widget.NewCheck("Show desktop notifications for failed jobs", nil)
	notifications.SetChecked(saved.NotifyOnFailure)
	notifications.OnChanged = func(bool) { updateSaveState() }
	themeSelect := widget.NewSelect([]string{themeLabelSystem, themeLabelGoSentry}, nil)
	themeSelect.SetSelected(themeLabel(saved.Theme))
	// Preview the theme the moment it is picked so the choice is visible before
	// saving; Save persists it. Reverting the selection reverts the preview, and
	// closing without saving falls back to the stored theme on next launch.
	themeSelect.OnChanged = func(string) {
		applyTheme(fyne.CurrentApp(), themeFromLabel(themeSelect.Selected))
		updateSaveState()
	}
	executionModeSelect := widget.NewSelect(
		[]string{string(domain.ExecutionModeParallel), string(domain.ExecutionModeSequential)},
		nil,
	)
	executionModeSelect.SetSelected(string(saved.ExecutionMode))
	executionModeSelect.OnChanged = func(string) { updateSaveState() }
	overlapPolicySelect := widget.NewSelect(
		[]string{string(domain.OverlapPolicySkip), string(domain.OverlapPolicyQueue)},
		nil,
	)
	overlapPolicySelect.SetSelected(string(saved.OverlapPolicy))
	overlapPolicySelect.OnChanged = func(string) { updateSaveState() }
	defaultTimeout := widget.NewEntry()
	defaultTimeout.SetPlaceHolder("0 = no timeout")
	defaultTimeout.SetText(strconv.Itoa(saved.DefaultTimeoutSeconds))
	defaultTimeout.OnChanged = func(string) { updateSaveState() }
	jobsFile := widget.NewEntry()
	jobsFile.SetText(saved.JobsFile)
	jobsFile.OnChanged = func(string) { updateSaveState() }
	// The picker only offers existing files; a jobs file that does not exist yet
	// is entered by typing its path, which Save then creates.
	jobsFileBrowse := widget.NewButtonWithIcon("Browse", theme.FileIcon(), func() {
		chooseJSONFile(w, jobsFile)
	})
	logsDir := widget.NewEntry()
	logsDir.SetText(saved.LogsDir)
	logsDir.OnChanged = func(string) { updateSaveState() }
	logsDirBrowse := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		chooseFolder(w, logsDir)
	})
	// Log files are read outside the app, so the folder gets a direct shortcut
	// beside its path instead of making the user copy the path into a file
	// manager. It reveals whatever the field currently holds, so an edit can be
	// checked before Save.
	logsDirOpen := widget.NewButtonWithIcon("Open", theme.FolderIcon(), func() {
		openFolder(w, settingsFolderPath(paths.AppDir, logsDir.Text))
	})
	maxLogFiles := widget.NewEntry()
	maxLogFiles.SetPlaceHolder("0 = unlimited")
	maxLogFiles.SetText(strconv.Itoa(saved.MaxLogFiles))
	maxLogFiles.OnChanged = func(string) { updateSaveState() }
	maxLogAgeDays := widget.NewEntry()
	maxLogAgeDays.SetPlaceHolder("0 = unlimited")
	maxLogAgeDays.SetText(strconv.Itoa(saved.MaxLogAgeDays))
	maxLogAgeDays.OnChanged = func(string) { updateSaveState() }
	// Autostart status sits on its own row beneath the checkbox (rather than
	// beside it) so the Application section fits within a half-width column.
	// Truncating keeps a long status message from forcing the column wider.
	autostartStatus.Truncation = fyne.TextTruncateClip
	settingsStatus := widget.NewLabel("")

	saveSettings := widget.NewButtonWithIcon("Save settings", theme.DocumentSaveIcon(), func() {
		files, err := strconv.Atoi(strings.TrimSpace(maxLogFiles.Text))
		if err != nil || files < 0 {
			settingsStatus.SetText("Max log files must be zero (unlimited) or a positive number")
			return
		}
		days, err := strconv.Atoi(strings.TrimSpace(maxLogAgeDays.Text))
		if err != nil || days < 0 {
			settingsStatus.SetText("Max log age days must be zero (unlimited) or a positive number")
			return
		}
		if strings.TrimSpace(jobsFile.Text) == "" {
			settingsStatus.SetText("Jobs file is required")
			return
		}
		if strings.TrimSpace(logsDir.Text) == "" {
			settingsStatus.SetText("Logs directory is required")
			return
		}
		timeout, err := strconv.Atoi(strings.TrimSpace(defaultTimeout.Text))
		if err != nil || timeout < 0 {
			settingsStatus.SetText("Default timeout must not be negative (0 = no timeout)")
			return
		}
		// Build the new config from the form and hand it to the Service, which
		// validates it, persists config and jobs to the (possibly new) directory,
		// and runs log cleanup so tightened retention limits take effect at once.
		config := saved
		config.JobsFile = strings.TrimSpace(jobsFile.Text)
		config.LogsDir = strings.TrimSpace(logsDir.Text)
		config.MaxLogFiles = files
		config.MaxLogAgeDays = days
		config.StartOnLogin = startOnLogin.Checked
		config.KeepRunningInTray = minimizeToTray.Checked
		config.NotifyOnFailure = notifications.Checked
		config.ExecutionMode = domain.ExecutionMode(executionModeSelect.Selected)
		config.OverlapPolicy = domain.OverlapPolicy(overlapPolicySelect.Selected)
		config.DefaultTimeoutSeconds = timeout
		config.Theme = themeFromLabel(themeSelect.Selected)
		previousKeepInTray := saved.KeepRunningInTray
		if err := svc.UpdateSettings(config); err != nil {
			settingsStatus.SetText("Save failed: " + err.Error())
			return
		}
		// UpdateSettings may re-resolve paths (a jobs-file switch adopts a
		// different directory), so pick up the fresh copy rather than assuming
		// config is exactly what landed.
		saved = svc.Config()
		paths = svc.Paths()
		if err := svc.ApplyAutostart(); err != nil {
			refreshAutostartStatus()
			settingsStatus.SetText("Saved, autostart failed: " + err.Error())
			return
		}
		refreshAutostartStatus()
		applyTrayBehavior(fyne.CurrentApp(), w, config.KeepRunningInTray, true)
		if previousKeepInTray != config.KeepRunningInTray {
			trayRestartHint.SetText(trayRestartHintText)
		} else {
			refreshTrayRestartHint(false)
		}
		settingsStatus.SetText("Saved")
		// The form now matches the persisted config, so disable Save again.
		updateSaveState()
	})

	// Save stays disabled until a field differs from the saved config, so the
	// button only invites a click when there is something to persist. The numeric
	// fields compare against their canonical string form; any unparsable text
	// counts as a change so the user can click Save and see the validation error.
	updateSaveState = func() {
		c := saved
		changed := startOnLogin.Checked != c.StartOnLogin ||
			minimizeToTray.Checked != c.KeepRunningInTray ||
			notifications.Checked != c.NotifyOnFailure ||
			executionModeSelect.Selected != string(c.ExecutionMode) ||
			overlapPolicySelect.Selected != string(c.OverlapPolicy) ||
			strings.TrimSpace(defaultTimeout.Text) != strconv.Itoa(c.DefaultTimeoutSeconds) ||
			strings.TrimSpace(jobsFile.Text) != c.JobsFile ||
			strings.TrimSpace(logsDir.Text) != c.LogsDir ||
			strings.TrimSpace(maxLogFiles.Text) != strconv.Itoa(c.MaxLogFiles) ||
			strings.TrimSpace(maxLogAgeDays.Text) != strconv.Itoa(c.MaxLogAgeDays) ||
			themeSelect.Selected != themeLabel(c.Theme)
		if changed {
			saveSettings.Enable()
		} else {
			saveSettings.Disable()
		}
	}
	updateSaveState()

	// loadFields populates every form control from a config without saving it,
	// backing both the Cancel button (reload the saved config, discarding edits)
	// and the Defaults button (load the built-in defaults for review before
	// Save is clicked).
	loadFields = func(c domain.Config) {
		startOnLogin.SetChecked(c.StartOnLogin)
		minimizeToTray.SetChecked(c.KeepRunningInTray)
		notifications.SetChecked(c.NotifyOnFailure)
		themeSelect.SetSelected(themeLabel(c.Theme))
		applyTheme(fyne.CurrentApp(), themeFromLabel(themeSelect.Selected))
		executionModeSelect.SetSelected(string(c.ExecutionMode))
		overlapPolicySelect.SetSelected(string(c.OverlapPolicy))
		defaultTimeout.SetText(strconv.Itoa(c.DefaultTimeoutSeconds))
		jobsFile.SetText(c.JobsFile)
		logsDir.SetText(c.LogsDir)
		maxLogFiles.SetText(strconv.Itoa(c.MaxLogFiles))
		maxLogAgeDays.SetText(strconv.Itoa(c.MaxLogAgeDays))
		if settingsPendingAutostart(startOnLogin, minimizeToTray, saved) {
			autostartStatus.SetText("Pending: save settings to apply")
		} else {
			refreshAutostartStatus()
		}
		refreshTrayRestartHint(minimizeToTray.Checked != saved.KeepRunningInTray)
		settingsStatus.SetText("")
		updateSaveState()
	}
	cancelSettings := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		loadFields(saved)
	})
	restoreDefaults := widget.NewButtonWithIcon("Defaults", theme.MediaReplayIcon(), func() {
		loadFields(domain.DefaultConfig())
	})

	return newSettingsLayout(settingsFormFields{
		startOnLogin:        startOnLogin,
		autostartStatus:     autostartStatus,
		minimizeToTray:      minimizeToTray,
		trayRestartHint:     trayRestartHint,
		notifications:       notifications,
		themeSelect:         themeSelect,
		executionModeSelect: executionModeSelect,
		overlapPolicySelect: overlapPolicySelect,
		defaultTimeout:      defaultTimeout,
		configPath:          paths.ConfigPath,
		jobsFile:            jobsFile,
		jobsFileBrowse:      jobsFileBrowse,
		logsDir:             logsDir,
		logsDirOpen:         logsDirOpen,
		logsDirBrowse:       logsDirBrowse,
		maxLogFiles:         maxLogFiles,
		maxLogAgeDays:       maxLogAgeDays,
		saveSettings:        saveSettings,
		cancelSettings:      cancelSettings,
		restoreDefaults:     restoreDefaults,
		settingsStatus:      settingsStatus,
	})
}

func settingsPendingAutostart(startOnLogin, minimizeToTray *widget.Check, saved domain.Config) bool {
	return startOnLogin.Checked != saved.StartOnLogin ||
		minimizeToTray.Checked != saved.KeepRunningInTray
}

// Theme dropdown labels. These are the human-facing captions; themeLabel and
// themeFromLabel translate between them and the stored domain.Theme values so the
// select never leaks the on-disk "system"/"gosentry" strings to the user.
const (
	themeLabelSystem   = "System"
	themeLabelGoSentry = "GoSentry"
)

func themeLabel(choice domain.Theme) string {
	if choice == domain.ThemeSystem {
		return themeLabelSystem
	}
	return themeLabelGoSentry
}

func themeFromLabel(label string) domain.Theme {
	if label == themeLabelGoSentry {
		return domain.ThemeGoSentry
	}
	return domain.ThemeSystem
}
