# Roadmap

This file tracks planned GoSentry work that is larger than a single bug fix.

## Refactoring Follow-Ups

Loose ends found while verifying the refactoring against its Definition of done.
The architecture target is reached and verified on Windows and Linux.

- **File-size guidelines exceeded (soft limits).** The DoD suggests no `src/ui` file
  over ~250 lines and no single file over ~400:
  - `src/ui/jobs_view.go` — 415 lines (over both the ~250 UI target and the ~400 cap).
  - `src/app/run.go` — dispatch logic split out of `operations.go` in this release.
  - `src/app/operations_test.go` (536) also exceeds 400.

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

## Tray Interaction (Resolved in 0.9.0)

Improve tray icon interaction: click the tray icon to show and focus the main
window.

- Upgraded to Fyne 2.7.4, which provides `desktop.App.SetSystemTrayWindow(window)`.
  Left-clicking the tray icon now shows and focuses the main window without opening
  the menu; the explicit "Show" menu item remains for right-click access and
  less-compliant Linux systems.
- Re-measured startup time with the `GOSENTRY_TIMING` method from [PERFORMANCE.md](PERFORMANCE.md).
  The Fyne 2.6.3 -> 2.7.4 upgrade recovers ~230 ms (startup drops from ~644 ms to ~414 ms, a
  −36% improvement). The 2.7.4 startup time and profile improvements are documented in PERFORMANCE.md.

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
