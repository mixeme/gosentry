# Roadmap

This file tracks planned GoSentry work that is larger than a single bug fix.

## Refactoring Follow-Ups

Loose ends found while verifying the [refactoring plan](REFACTORING.md) against
its Definition of done. The architecture target is reached and verified on
Windows, but the items below remain.

- **Linux test build is broken (correctness, not cosmetic).** `src/runner/runner_test.go`
  is a shared (untagged) test file that references Windows-only symbols
  (`SysProcAttr.HideWindow`, `SysProcAttr.CmdLine`, `windowsShellCommandLine`).
  The `runtime.GOOS != "windows"` guards are runtime skips and cannot save a file
  that does not *compile*, so `go test ./...` fails to build on Linux. This
  contradicts T5.4 ("go test -race clean on both platforms") and the DoD's
  "green on Windows and Linux." Fix: move the Windows-only tests
  (`TestShellCommandHidesWindow`, `TestShellCommandUsesWindowsSafeQuoting`, and any
  peers touching `SysProcAttr` / `windowsShellCommandLine`) into a new
  `src/runner/runner_windows_test.go` guarded by `//go:build windows`.
- **File-size guidelines exceeded.** The DoD asks for no `src/ui` file over ~250
  lines and no single file over ~400:
  - `src/ui/jobs_view.go` — 415 lines (over both the ~250 UI target and the ~400 cap).
  - `src/app/operations.go` — 486 lines (over ~400).
  - `src/app/operations_test.go` (536) and `src/runner/runner_test.go` (421) also
    exceed 400 if the cap is read to include test files.

  These are soft ("~") limits; revisit when next touching those files rather than
  splitting purely for line count.

## Post-Field-Test Cleanup

After real-world use confirms the main workflows, clean up temporary
stabilization code and development scaffolding.

Cleanup checklist:

- Review and remove debug-oriented diagnostics that are no longer useful.
- Remove excessive defensive checks once behavior is proven and covered by the
  right tests.
- Remove obsolete compatibility cleanup, such as old autostart migration code,
  after the transition window is over.
- Delete stale generated files and old build artifacts from local/release flows.
- Revisit tests and remove ones that only lock in temporary implementation
  details instead of real user-facing behavior.
- Simplify README notes that were useful during early setup but are too noisy
  for normal users.
- Recheck `.gitignore`, Docker scripts, and packaging scripts for rules or
  branches that only supported early experiments.

## Tray Interaction

Improve tray icon interaction: click the tray icon to show and focus the main
window.

- Unblocked by Fyne 2.7.0, which added `desktop.App.SetSystemTrayWindow(window)`.
  On Windows, macOS, and most Linux it shows the associated window on left-click;
  any tray menu then moves to right-click. There is still no raw click /
  double-click callback, so the behavior is single left-click (the conventional
  tray gesture), not the double-click originally sketched here.
- The project is currently on Fyne 2.6.3, so this depends on a Fyne 2.6.3 -> 2.7.x
  upgrade first (minor bump; re-verify the CGO build under MSYS2 UCRT64 and check
  for 2.7 breaking changes). Track the upgrade as its own task.
- As part of that upgrade, **re-measure startup time** with the `GOSENTRY_TIMING`
  method recorded in [PERFORMANCE.md](PERFORMANCE.md). The Fyne 2.5.3 -> 2.6.3 bump
  added ~290 ms to startup, all inside `w.Show()`; check whether 2.7 recovers any
  of it or holds steady, and append the result to PERFORMANCE.md.
- After upgrading, the change in `src/ui/run.go` (configureSystemTray) is small:
  call `desk.SetSystemTrayWindow(w)` alongside `SetSystemTrayMenu(menu)`. Keep the
  existing "Show" menu item, which the Fyne docs recommend for less-compliant
  Linux systems.

## Delivery And Packaging

Keep a single portable binary as the baseline delivery format. It is simple to
test, easy to copy between machines, and matches the current storage model where
runtime YAML files live next to the executable by default.

Planned delivery variants:

- Windows portable `.zip` with `gosentry.exe`, `README.md`, and `CHANGELOG.md`.
- Linux portable `.tar.gz` archives for `linux-amd64` and `linux-arm64`.
- Debian/Ubuntu `.deb` package once the Linux runtime paths are settled.
- Windows installer later, likely Inno Setup first and MSI/WiX only if needed.
- AppImage as a possible Linux GUI-friendly format after the core workflow is stable.
- Flatpak only after the desktop integration story is clearer.
- winget manifest after stable public Windows releases exist.

Packaging design note:

- Portable builds can keep settings and jobs next to the executable.
- Installer/package builds should move runtime data to per-user locations:
  `%APPDATA%\GoSentry` on Windows, and XDG directories such as
  `~/.config/gosentry` and `~/.local/share/gosentry` on Linux.

Initial priority:

1. Windows portable `.zip`.
2. Linux portable `.tar.gz` for amd64 and arm64.
3. Debian/Ubuntu `.deb`.
4. Windows installer.
