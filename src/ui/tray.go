package ui

import (
	"runtime"

	"gitea.mixdep.ru/mix/gosentry/assets"
	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	fynedesktop "fyne.io/fyne/v2/driver/desktop"
)

// trayState tracks the two pieces of tray-related process state that Fyne
// itself does not expose: whether this process has registered the tray icon
// (Fyne cannot add or remove it mid-session, so toggling KeepRunningInTray in
// Settings updates close behavior immediately but shows a restart hint for the
// icon itself) and whether the primary window is currently hidden via the tray
// close intercept (Fyne exposes no Window.Visible API). Run owns one instance
// and passes it to every call site of apply — settingsView's Save handler is
// the other one — so the coupling between them is explicit instead of hidden
// behind package-level globals that no test can reset.
type trayState struct {
	registered bool
	hidden     bool
}

const trayRestartHintText = "Restart GoSentry for the tray icon change to take effect."

func resolveStartHidden(cliStartInTray, keepInTray bool) bool {
	return domain.ResolveStartHidden(cliStartInTray, keepInTray)
}

// apply configures window close handling for KeepRunningInTray. When
// revealIfHidden is true and the tray is off, a hidden window is shown so the
// user can still reach the app after disabling the tray mid-session.
func (t *trayState) apply(a fyne.App, w fyne.Window, keepInTray bool, revealIfHidden bool) {
	if keepInTray && !t.registered {
		t.registerSystemTray(a, w)
		t.registered = true
	}
	t.setWindowCloseBehavior(w, keepInTray)
	if !keepInTray && revealIfHidden && t.hidden {
		t.hidden = false
		w.Show()
		w.RequestFocus()
	}
}

func (t *trayState) registerSystemTray(a fyne.App, w fyne.Window) {
	desk, ok := a.(fynedesktop.App)
	if !ok {
		// Not every Fyne driver exposes desktop tray features. Returning silently
		// keeps the same binary usable on platforms or sessions without a tray.
		return
	}

	// The tray icon is platform-specific. The Windows notification area is
	// ICO-native and renders at 16-24px, so a single 16x16 .ico frame keeps the
	// hand-tuned glyph crisp with correct alpha. Linux/macOS trays
	// (StatusNotifierItem) render much larger (22-48px) and take a PNG, so the
	// full-size artwork scales down cleanly instead of upscaling the tiny 16x16.
	if runtime.GOOS == "windows" {
		desk.SetSystemTrayIcon(assets.IconSmallICO())
	} else {
		desk.SetSystemTrayIcon(assets.Icon())
	}

	// IsQuit marks this as the tray's quit item. Without it Fyne's
	// addMissingQuitForMenu appends a second, localized Quit (e.g. "Выход" on a
	// Russian system) because it only recognizes an existing quit by matching the
	// localized label — which our literal "Quit" does not. Setting IsQuit makes
	// Fyne reuse this item instead of adding a duplicate, regardless of locale.

	quit := fyne.NewMenuItem("Quit", func() {
		a.Quit()
	})
	quit.IsQuit = true
	menu := fyne.NewMenu("GoSentry",
		fyne.NewMenuItem("Show", func() {
			t.hidden = false
			w.Show()
			w.RequestFocus()
		}),
		fyne.NewMenuItemSeparator(),
		quit,
	)
	desk.SetSystemTrayMenu(menu)
	desk.SetSystemTrayWindow(w)
}

func (t *trayState) setWindowCloseBehavior(w fyne.Window, keepInTray bool) {
	if keepInTray {
		w.SetCloseIntercept(func() {
			// Closing hides the window instead of quitting because scheduler tools are
			// expected to keep working in the background. The explicit Quit tray item
			// remains the way to stop the process.
			t.hidden = true
			w.Hide()
		})
		return
	}
	t.hidden = false
	w.SetCloseIntercept(nil)
}
