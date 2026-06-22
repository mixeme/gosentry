package ui

import (
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
