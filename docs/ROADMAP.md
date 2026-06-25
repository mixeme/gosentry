# Roadmap

This file tracks planned GoSentry work that is larger than a single bug fix.
Completed work is recorded in [CHANGELOG.md](CHANGELOG.md), not here.

## Open Items

### Window size persistence *(frozen)*

Window size is currently **not** saved on quit or close. Saving was disabled
because `w.Canvas().Size()` returns the maximized dimensions when the window is
maximized, which would corrupt the stored size on the next launch.

Re-enabling requires a cross-platform way to detect the maximized state before
saving. Fyne v2.x has no API for this; it needs per-OS native calls:
`IsZoomed` (Windows), `_NET_WM_STATE` (X11/Linux), `NSWindow.isZoomed`
(macOS). Unfreeze once that detection is in place.

### History tab — column filters (Trigger / Job / State)

Add dropdown filters above the History table so the user can narrow rows by
trigger source, job name, or run state. Blocked on Fyne native support: the
current `widget.Table` has no built-in filter API, and a filter bar built from
`widget.Select` widgets above the table feels visually out-of-place. Revisit
when Fyne adds first-class column filtering or a composable data-grid widget.
