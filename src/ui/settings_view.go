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
	store := svc.Store()
	// updateSaveState compares the form to the saved config and enables Save only
	// when something differs. It is defined below (once Save and every field
	// exist) but declared here so the field change handlers can reference it.
	var updateSaveState func()
	// loadFields populates every form control from the given config. It backs
	// both the initial load and the Cancel/Defaults buttons below.
	var loadFields func(domain.Config)
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
	themeSelect := widget.NewSelect([]string{themeLabelDefault, themeLabelGoSentry}, nil)
	themeSelect.SetSelected(themeLabel(store.Config.Theme))
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
	executionModeSelect.SetSelected(string(store.Config.ExecutionMode))
	executionModeSelect.OnChanged = func(string) { updateSaveState() }
	overlapPolicySelect := widget.NewSelect(
		[]string{string(domain.OverlapPolicySkip), string(domain.OverlapPolicyQueue)},
		nil,
	)
	overlapPolicySelect.SetSelected(string(store.Config.OverlapPolicy))
	overlapPolicySelect.OnChanged = func(string) { updateSaveState() }
	defaultTimeout := widget.NewEntry()
	defaultTimeout.SetPlaceHolder("0 = no timeout")
	defaultTimeout.SetText(strconv.Itoa(store.Config.DefaultTimeoutSeconds))
	defaultTimeout.OnChanged = func(string) { updateSaveState() }
	jobsFile := widget.NewEntry()
	jobsFile.SetText(store.Config.JobsFile)
	jobsFile.OnChanged = func(string) { updateSaveState() }
	// The picker only offers existing files; a jobs file that does not exist yet
	// is entered by typing its path, which Save then creates.
	jobsFileBrowse := widget.NewButtonWithIcon("Browse", theme.FileIcon(), func() {
		chooseJSONFile(w, jobsFile)
	})
	logsDir := widget.NewEntry()
	logsDir.SetText(store.Config.LogsDir)
	logsDir.OnChanged = func(string) { updateSaveState() }
	logsDirBrowse := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		chooseFolder(w, logsDir)
	})
	// Log files are read outside the app, so the folder gets a direct shortcut
	// beside its path instead of making the user copy the path into a file
	// manager. It reveals whatever the field currently holds, so an edit can be
	// checked before Save.
	logsDirOpen := widget.NewButtonWithIcon("Open", theme.FolderIcon(), func() {
		openFolder(w, settingsFolderPath(store.Paths.AppDir, logsDir.Text))
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
	autostartStatus.Truncation = fyne.TextTruncateClip
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
		config := store.Config
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
		if startOnLogin.Checked != store.Config.StartOnLogin {
			autostartStatus.SetText("Pending: save settings to apply")
		} else {
			refreshAutostartStatus()
		}
		settingsStatus.SetText("")
		updateSaveState()
	}
	cancelSettings := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		loadFields(store.Config)
	})
	restoreDefaults := widget.NewButtonWithIcon("Defaults", theme.MediaReplayIcon(), func() {
		loadFields(domain.DefaultConfig())
	})

	return newSettingsLayout(settingsFormFields{
		startOnLogin:        startOnLogin,
		autostartStatus:     autostartStatus,
		minimizeToTray:      minimizeToTray,
		notifications:       notifications,
		themeSelect:         themeSelect,
		executionModeSelect: executionModeSelect,
		overlapPolicySelect: overlapPolicySelect,
		defaultTimeout:      defaultTimeout,
		configPath:          store.Paths.ConfigPath,
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

// Theme dropdown labels. These are the human-facing captions; themeLabel and
// themeFromLabel translate between them and the stored domain.Theme values so the
// select never leaks the on-disk "default"/"gosentry" strings to the user.
const (
	themeLabelDefault  = "Default"
	themeLabelGoSentry = "GoSentry"
)

func themeLabel(choice domain.Theme) string {
	if choice == domain.ThemeDefault {
		return themeLabelDefault
	}
	return themeLabelGoSentry
}

func themeFromLabel(label string) domain.Theme {
	if label == themeLabelGoSentry {
		return domain.ThemeGoSentry
	}
	return domain.ThemeDefault
}
