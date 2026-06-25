# Roadmap

This file tracks planned GoSentry work that is larger than a single bug fix.
Completed work is recorded in [CHANGELOG.md](CHANGELOG.md), not here.

## Open Items

### Window size — skip saving when maximized

When the window is closed or the app quits while maximized, the maximized
dimensions are persisted and the window opens at that size on the next launch.
The fix requires detecting the window's maximized state via the native OS API
(`IsZoomed` on Windows, `_NET_WM_STATE` on X11/Linux, `NSWindow.isZoomed` on
macOS). A platform-specific implementation per OS is needed; a cross-platform
Fyne API for this does not exist in v2.x.

### History tab — column filters (Trigger / Job / State)

Add dropdown filters above the History table so the user can narrow rows by
trigger source, job name, or run state. Blocked on Fyne native support: the
current `widget.Table` has no built-in filter API, and a filter bar built from
`widget.Select` widgets above the table feels visually out-of-place. Revisit
when Fyne adds first-class column filtering or a composable data-grid widget.
