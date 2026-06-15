package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

// The application icon is embedded into the binary instead of being loaded from
// an assets directory at runtime. That keeps the Windows/Linux distribution to a
// single executable and avoids the common failure mode where the app starts with
// a generic icon because a sidecar PNG was not copied with the binary.
//
// The blank import enables the compiler directive below; no runtime package
// initialization from embed is required.
//
//go:embed pysentry-icon.png
var iconBytes []byte

func Icon() fyne.Resource {
	// Fyne accepts resources from memory, so the same embedded PNG can be used
	// for the window icon and tray icon. The Windows Explorer icon is still added
	// by the build script through the .ico resource, because Explorer reads PE
	// resources rather than Fyne runtime state.
	return fyne.NewStaticResource("pysentry-icon.png", iconBytes)
}

func IconBytes() []byte {
	return append([]byte(nil), iconBytes...)
}
