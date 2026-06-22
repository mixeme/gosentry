package ui

import (
	"time"

	"gitea.mixdep.ru/mix/gosentry/assets"
	"gitea.mixdep.ru/mix/gosentry/src/app"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
)

const appID = "ru.mixdep.gosentry.desktop"

// Run is the application entry point. It owns the process lifecycle — single
// instance arbitration, Fyne app + window construction, tray wiring, and the
// startup-timing record — and delegates all view construction to newMainView in
// mainwindow.go. Keeping lifecycle here and the view there is the run.go /
// mainwindow.go split introduced in T4.1.
func Run(startInTray bool) {
	started := time.Now()
	instanceListener, primary := acquireSingleInstance(!startInTray)
	if !primary {
		return
	}
	if instanceListener != nil {
		defer instanceListener.Close()
	}

	// A stable app ID lets Fyne persist desktop preferences consistently across
	// launches and gives tray/window integration a predictable identity.
	a := fyneapp.NewWithID(appID)
	a.SetIcon(loadAppIcon())

	w := a.NewWindow("GoSentry " + app.Version)
	configureSystemTray(a, w)
	w.Resize(fyne.NewSize(1120, 720))
	content, recordStartup := newMainView(w)
	w.SetContent(content)
	serveSingleInstance(instanceListener, w)
	if startInTray {
		// Autostart launches intentionally stay hidden, so "window shown" would be
		// a misleading metric. Record a separate startup event for the tray path
		// instead of forcing one timing definition onto two different UX flows.
		recordStartup(time.Since(started), false)
		a.Run()
		return
	}
	// Show the window before recording startup time. Measuring earlier, during
	// widget construction, looked cheaper in History than the user-perceived
	// startup really was. The current point is less abstract: it ends when the
	// window has actually been handed to the desktop for display.
	w.Show()
	recordStartup(time.Since(started), true)
	a.Run()
}

func loadAppIcon() fyne.Resource {
	return assets.Icon()
}
