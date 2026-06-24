package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

// Icons are embedded into the binary instead of being loaded from an assets
// directory at runtime. That keeps the Windows/Linux distribution to a single
// executable and avoids the common failure mode where the app starts with a
// generic icon because a sidecar PNG was not copied with the binary. The blank
// "embed" import enables the //go:embed directives below.
//
// # Cross-platform icon strategy
//
// The hard constraint: Fyne's a.SetIcon and SetSystemTrayIcon each take ONE
// image, which the OS then scales to every size it needs — titlebar (~16px),
// taskbar/dock (~32-48px), and tray. Neither source survives that scaling:
// downscaling the 1254px gosentry-icon-big.png to 16px is muddy, and upscaling
// the 16px icon to 32px is blurry. The fix is to feed each surface a
// size-appropriate source — which differs per platform because each platform
// exposes different icon channels.
//
// Source assets (all have a transparent boundary; note that a *binary* white-key
// leaves the anti-aliased edge fully opaque as a light halo that reads as a
// border on a dark taskbar/tray, so the background is removed with feathered
// color-to-alpha instead):
//   - gosentry-icon-large.png  detailed large artwork (teal rounded-tile emblem)
//   - gosentry-icon-small.png  hand-tuned for legibility at 16px
//   - gosentry.ico             multi-size 16/32/48/256 (16 = the hand-tuned PNG,
//                              the rest downscaled from large). Embedded into the PE
//                              binary by windres (see scripts/build-windows.bat),
//                              NOT via Go embed.
//   - gosentry-icon-small.ico  single 16px frame, for the Windows tray
//
// Windows:
//   - Window titlebar + taskbar: the multi-size gosentry.ico, embedded by the .rc
//     under the resource name GLFW_ICON (packaging/windows/gosentry.rc). GLFW uses
//     it as the window's default icon and selects the right frame per size — the
//     hand-tuned 16 for the titlebar, a larger frame for the taskbar. For this to
//     work, src/ui/run.go must NOT call a.SetIcon on Windows: a single SetIcon
//     resource overrides GLFW_ICON and would be scaled to both sizes.
//   - Tray: SetSystemTrayIcon(IconSmallICO()). The notification area is ICO-native
//     and renders at 16-24px; a single-frame 16x16 .ico pins the hand-tuned glyph
//     (a multi-size .ico made the tray pick and downscale a larger frame).
//
// Linux / other non-Windows (no PE icon resource exists):
//   - Window titlebar: a.SetIcon(IconSmall()) in run.go feeds the resource to
//     _NET_WM_ICON, which the window manager renders ~16px in the titlebar, so the
//     hand-tuned 16x16 keeps it crisp.
//   - Dock/launcher: the larger icon comes from the .desktop entry's Icon=, written
//     by InstallDesktopIcon (src/platform/desktop) from the big artwork.
//   - Tray: SetSystemTrayIcon(Icon()). StatusNotifierItem renders 22-48px and takes
//     a PNG, so the big artwork scales down cleanly (the 16x16 would look tiny).

//go:embed gosentry-icon-small.png
var iconSmallBytes []byte

//go:embed gosentry-icon-large.png
var iconLargeBytes []byte

//go:embed gosentry-icon-small.ico
var iconSmallICOBytes []byte

// IconSmall returns the hand-tuned 16x16 PNG. It is the Linux window-titlebar
// icon (via a.SetIcon -> _NET_WM_ICON, which the WM renders at ~16px). On Windows
// the titlebar comes from gosentry.ico instead; see the package strategy above.
func IconSmall() fyne.Resource {
	return fyne.NewStaticResource("gosentry-icon-small.png", iconSmallBytes)
}

// IconSmallICO returns a single-frame 16x16 Windows .ico of the hand-tuned small
// icon, used for the Windows system tray. The notification area is ICO-native, and
// pinning a single 16x16 frame keeps the hand-tuned glyph crisp at tray size — a
// multi-size .ico lets the tray pick and downscale a larger, muddier frame.
func IconSmallICO() fyne.Resource {
	return fyne.NewStaticResource("gosentry-icon-small.ico", iconSmallICOBytes)
}

// Icon returns the large artwork PNG. It is the Linux tray icon (StatusNotifierItem
// renders 22-48px) and the source for the Linux .desktop dock icon via IconBytes.
// The Windows window/taskbar icon comes from gosentry.ico, not this resource.
func Icon() fyne.Resource {
	return fyne.NewStaticResource("gosentry-icon-large.png", iconLargeBytes)
}

// IconBytes is the large artwork as raw PNG bytes for InstallDesktopIcon, which
// writes the Linux .desktop launcher/dock icon.
func IconBytes() []byte {
	return append([]byte(nil), iconLargeBytes...)
}
