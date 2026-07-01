package ui

import (
	"runtime"
	"time"

	"gitea.mixdep.ru/mix/gosentry/assets"
	"gitea.mixdep.ru/mix/gosentry/src/app"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const appID = "ru.mixeme.gosentry.desktop"

// Run is the application entry point. It owns the process lifecycle — single
// instance arbitration, Fyne app + window construction, tray wiring, and the
// startup-timing record — and delegates all view construction to newMainView in
// mainwindow.go. Keeping lifecycle here and the view there is the run.go /
// mainwindow.go split keeps lifecycle separate from view construction.
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
	// On Windows the multi-resolution gosentry.ico (embedded under the GLFW_ICON
	// resource name) drives the window: GLFW picks the hand-tuned 16x16 for the
	// titlebar and the large artwork for the bigger taskbar icon — size-appropriate
	// in a way a single Fyne SetIcon resource cannot be, since one PNG would be
	// scaled to both sizes.
	//
	// Other platforms have no PE icon. Fyne's single SetIcon resource feeds
	// _NET_WM_ICON, which the window manager renders small (~16px) in the titlebar,
	// so use the hand-tuned small icon there to keep it crisp. The larger
	// dock/launcher icon comes from the .desktop entry installed by
	// InstallDesktopIcon, which uses the big artwork.
	if runtime.GOOS != "windows" {
		a.SetIcon(assets.IconSmall())
	}

	w := a.NewWindow("GoSentry " + app.Version)
	configureSystemTray(a, w)
	prefs := a.Preferences()
	winW := float32(prefs.FloatWithFallback("window.width", 1024))
	winH := float32(prefs.FloatWithFallback("window.height", 660))
	w.Resize(fyne.NewSize(winW, winH))
	svc, err := app.Open()
	if err != nil {
		w.SetContent(container.NewPadded(widget.NewLabel("Failed to load GoSentry configuration: " + err.Error())))
		a.Run()
		return
	}
	content, recordStartup := newMainView(w, svc)
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
