package ui

import (
	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
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

func settingsView(w fyne.Window, svc *app.Service, tray *trayState) fyne.CanvasObject {
	return newSettingsLayout(buildSettingsForm(w, svc, tray))
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
