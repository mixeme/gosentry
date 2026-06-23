package ui

import (
	"runtime"

	"gitea.mixdep.ru/mix/gosentry/assets"

	"fyne.io/fyne/v2"
	fynedesktop "fyne.io/fyne/v2/driver/desktop"
)

func configureSystemTray(a fyne.App, w fyne.Window) {
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
			w.Show()
			w.RequestFocus()
		}),
		fyne.NewMenuItemSeparator(),
		quit,
	)
	desk.SetSystemTrayMenu(menu)
	w.SetCloseIntercept(func() {
		// Closing hides the window instead of quitting because scheduler tools are
		// expected to keep working in the background. The explicit Quit tray item
		// remains the way to stop the process.
		w.Hide()
	})
}
