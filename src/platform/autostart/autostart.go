package autostart

// Manager controls platform autostart for the application.
type Manager interface {
	// Set writes or removes the platform autostart entry to match enabled.
	Set(enabled bool, executablePath, iconPath string) error
	// Status reports whether the platform autostart entry matches expectedEnabled.
	Status(expectedEnabled bool, executablePath string) (ok bool, message string)
}
