package autostart

// Manager controls platform autostart for the application.
type Manager interface {
	// Set writes or removes the platform autostart entry to match enabled.
	// When enabled, startInTray selects whether the entry passes --start-in-tray.
	Set(enabled, startInTray bool, executablePath, iconPath string) error
	// Status reports whether the platform autostart entry matches expectedEnabled
	// and startInTray.
	Status(expectedEnabled, startInTray bool, executablePath string) (ok bool, message string)
}
