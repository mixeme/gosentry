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

const settingsLabelWidth float32 = 140
const settingsControlWidth float32 = 330
const settingsStatusWidth float32 = 280
const projectRepositoryURL = "https://gitea.mixdep.ru/mix/gosentry"

// settingsRowSpacing is the (negative) gap between rows of the settings form,
// overlapping each control's built-in vertical padding so the column is tighter
// and more compact, matching the condensed job details panel.
const settingsRowSpacing float32 = -6

func settingsView(w fyne.Window, svc *app.Service) fyne.CanvasObject {
	store := svc.Store()
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
			return
		}
		refreshAutostartStatus()
	}
	refreshAutostartStatus()
	minimizeToTray := widget.NewCheck("Keep running in the system tray", nil)
	minimizeToTray.SetChecked(store.Config.KeepRunningInTray)
	notifications := widget.NewCheck("Show desktop notifications for failed jobs", nil)
	notifications.SetChecked(store.Config.NotifyOnFailure)
	executionModeSelect := widget.NewSelect(
		[]string{string(domain.ExecutionModeParallel), string(domain.ExecutionModeSequential)},
		nil,
	)
	executionModeSelect.SetSelected(string(store.Config.ExecutionMode))
	overlapPolicySelect := widget.NewSelect(
		[]string{string(domain.OverlapPolicySkip), string(domain.OverlapPolicyQueue)},
		nil,
	)
	overlapPolicySelect.SetSelected(string(store.Config.OverlapPolicy))
	jobsDir := widget.NewEntry()
	jobsDir.SetText(store.Config.JobsDir)
	jobsDirBrowse := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		chooseFolder(w, jobsDir)
	})
	logsDir := widget.NewEntry()
	logsDir.SetText(store.Config.LogsDir)
	logsDirBrowse := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		chooseFolder(w, logsDir)
	})
	maxLogFiles := widget.NewEntry()
	maxLogFiles.SetText(strconv.Itoa(store.Config.MaxLogFiles))
	maxLogAgeDays := widget.NewEntry()
	maxLogAgeDays.SetText(strconv.Itoa(store.Config.MaxLogAgeDays))
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
	})

	// The settings form is a tall fixed-height column. Wrapping it in a vertical
	// scroll keeps its minimum height small so it does not dictate the whole
	// window's minimum height (AppTabs sizes to the tallest tab); on short 720p
	// screens the window can shrink and the form scrolls instead.
	return container.NewVScroll(container.NewPadded(container.New(compactVBoxLayout{spacing: settingsRowSpacing},
		widget.NewLabelWithStyle("Application", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		settingsRowWithStatus("Autostart", startOnLogin, autostartStatus),
		settingsRow("Tray", container.New(minWidthLayout{width: settingsControlWidth}, minimizeToTray)),
		settingsRow("Notifications", container.New(minWidthLayout{width: settingsControlWidth}, notifications)),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Queue", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		settingsRow("Execution mode", container.New(minWidthLayout{width: settingsControlWidth}, executionModeSelect)),
		settingsRow("Default overlap policy", container.New(minWidthLayout{width: settingsControlWidth}, overlapPolicySelect)),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Storage", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		settingsRow("Config YAML", widget.NewLabel(store.Paths.ConfigPath)),
		settingsRow("Jobs directory", container.NewBorder(nil, nil, nil, jobsDirBrowse, jobsDir)),
		settingsRow("Logs directory", container.NewBorder(nil, nil, nil, logsDirBrowse, logsDir)),
		settingsRow("Max log files", maxLogFiles),
		settingsRow("Max log age days", maxLogAgeDays),
		saveSettings,
		settingsStatus,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("About", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		settingsRow("GoSentry", widget.NewLabel(app.Version)),
		settingsRow("Go", widget.NewLabel(runtime.Version())),
		settingsRow("Fyne", widget.NewLabel(fyneVersion())),
		settingsRow("Repository", widget.NewHyperlink(projectRepositoryURL, mustParseURL(projectRepositoryURL))),
	)))
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

func settingsRowWithStatus(label string, value fyne.CanvasObject, status fyne.CanvasObject) fyne.CanvasObject {
	valueBox := container.New(minWidthLayout{width: settingsControlWidth}, value)
	statusBox := container.New(minWidthLayout{width: settingsStatusWidth}, status)
	return settingsRow(label, container.NewBorder(nil, nil, valueBox, nil, statusBox))
}
