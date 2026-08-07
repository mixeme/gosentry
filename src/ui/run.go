package ui

import (
	"runtime"
	"time"

	"gitea.mixdep.ru/mix/gosentry/assets"
	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/storage"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const appID = "ru.mixeme.gosentry.desktop"

// defaultWindowWidth and defaultWindowHeight are the size the window opens at
// on every launch. Window size persistence is frozen (see ROADMAP.md), so
// there is no saved size to restore. Fyne enforces the assembled content's
// MinSize as a hard floor over these, so they only take effect if the content
// actually fits within them.
const defaultWindowWidth = 1024
const defaultWindowHeight = 660

// Run is the application entry point. It owns the process lifecycle — single
// instance arbitration, Fyne app + window construction, tray wiring, and the
// startup-timing record — and delegates all view construction to newMainView in
// mainwindow.go. Keeping lifecycle here and the view there is the run.go /
// mainwindow.go split keeps lifecycle separate from view construction.
func Run(startInTray bool) {
	started := time.Now()
	keepInTray := storage.PeekKeepRunningInTray()
	startHidden := resolveStartHidden(startInTray, keepInTray)
	instanceListener, primary := acquireSingleInstance(!startHidden)
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
	setWindowsNotificationIcon()
	w.Resize(fyne.NewSize(defaultWindowWidth, defaultWindowHeight))
	svc, err := app.Open()
	if err != nil {
		w.SetContent(container.NewPadded(widget.NewLabel("Failed to load GoSentry configuration: " + err.Error())))
		a.Run()
		return
	}
	config := svc.Config()
	keepInTray = config.KeepRunningInTray
	startHidden = resolveStartHidden(startInTray, keepInTray)
	tray := &trayState{}
	tray.apply(a, w, keepInTray, false)
	// Apply the persisted theme before building content so the window renders in
	// the chosen theme from the first frame rather than flashing the default one.
	applyTheme(a, config.Theme)
	content, recordStartup := newMainView(w, svc, tray)
	w.SetContent(content)
	serveSingleInstance(instanceListener, w, tray)
	if startHidden {
		// Autostart launches intentionally stay hidden, so "window shown" would be
		// a misleading metric. Record a separate startup event for the tray path
		// instead of forcing one timing definition onto two different UX flows.
		recordStartup(time.Since(started), false)
		a.Run()
		svc.Stop()
		return
	}
	// Show the window before recording startup time. Measuring earlier, during
	// widget construction, looked cheaper in History than the user-perceived
	// startup really was. The current point is less abstract: it ends when the
	// window has actually been handed to the desktop for display.
	w.Show()
	recordStartup(time.Since(started), true)
	a.Run()
	// a.Run() blocks until the tray's Quit item or a window close calls a.Quit().
	// Stopping here — rather than not at all — cancels the run context so an
	// in-flight run's os/exec call sees ctx.Done() instead of being orphaned, and
	// stops the scheduler goroutine before the process exits.
	svc.Stop()
}

// setWindowsNotificationIcon supplies App.Icon for Fyne desktop notifications
// without touching the window or taskbar icon. On Windows those come from the PE
// gosentry.ico resource, so run.go must not call SetIcon. Fyne's NewWindow ends
// with SetIcon(nil), which adopts App.Icon when it is already set — metadata
// must therefore be registered only after the window is created. The tray icon
// is set separately in tray.go via SetSystemTrayIcon.
func setWindowsNotificationIcon() {
	if runtime.GOOS != "windows" {
		return
	}
	fyneapp.SetMetadata(fyne.AppMetadata{
		ID:   appID,
		Name: "GoSentry",
		Icon: assets.Icon(),
	})
}
