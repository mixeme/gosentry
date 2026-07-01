# Roadmap

This file tracks planned GoSentry work that is larger than a single bug fix.
Completed work is recorded in [CHANGELOG.md](CHANGELOG.md), not here.

## Open Items

### Per-job command timeout

`runner.RunJob` applies a fixed **30s** timeout to every command (`commandTimeout`
in `src/runner/runner.go`). Long-running or interactive scripts need a longer
limit; quick health checks may need a shorter one.

Add an optional per-job timeout (seconds) on `domain.Job`, with a global default
in `gosentry.json` for jobs that leave the field empty — the same inherit pattern
as `overlap_policy`. Wire the value through `RunJob`; expose it in the job dialog
and Settings; validate on save. `StartOnly` jobs should keep measuring launch
latency only and remain unaffected by the run timeout.

### Window size persistence *(frozen)*

Window size is currently **not** saved on quit or close. Saving was disabled
because `w.Canvas().Size()` returns the maximized dimensions when the window is
maximized, which would corrupt the stored size on the next launch.

Re-enabling requires a cross-platform way to detect the maximized state before
saving. Fyne v2.x has no API for this; it needs per-OS native calls:
`IsZoomed` (Windows), `_NET_WM_STATE` (X11/Linux), `NSWindow.isZoomed`
(macOS). Unfreeze once that detection is in place.

**Disadvantages of a platform-specific approach:**

- *Three separate implementations.* Windows, macOS, and Linux each need their
  own file guarded by a build tag. Each adds CGO bindings or raw syscall
  wrappers that must be kept in sync as OS APIs evolve.
- *Linux is not one target.* X11 and Wayland have completely different window
  state models. `_NET_WM_STATE` is X11-only; under Wayland the compositor
  controls window decorations and there is no stable client-side API to query
  the maximized state. A single `linux` build tag cannot cover both correctly.
- *Native window handle is not exposed.* Fyne does not surface the underlying
  `HWND` / `NSWindow` / `XID` through its public API. Obtaining it requires
  either enumerating OS-level windows by PID (fragile, finds wrong windows when
  dialogs are open) or reaching into Fyne/GLFW internals (breaks on Fyne
  upgrades).
- *Thread-safety constraints.* Win32 and GLFW both require their calls to be
  made from the OS main thread. Tray-menu callbacks run on a separate goroutine,
  so any native call must be marshalled back to the main thread, adding
  synchronisation complexity.
- *Test coverage gap.* Maximized-state detection cannot be exercised by Fyne's
  headless test driver; it requires a real display and manual or screen-capture
  automation per platform.

### History tab — column filters (Trigger / Job / State)

Add dropdown filters above the History table so the user can narrow rows by
trigger source, job name, or run state. Blocked on Fyne native support: the
current `widget.Table` has no built-in filter API, and a filter bar built from
`widget.Select` widgets above the table feels visually out-of-place. Revisit
when Fyne adds first-class column filtering or a composable data-grid widget.
