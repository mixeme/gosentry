# Roadmap

This file tracks planned GoSentry work that is larger than a single bug fix.

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

Improve tray icon interaction after choosing a tray backend path.

- Add double-click on the tray icon to show and focus the main window.
- Current Fyne 2.5.3 desktop tray API exposes menu and icon setup, but does not
  expose click or double-click callbacks for the tray icon itself.
- Revisit when Fyne exposes this callback, or evaluate a small platform-specific
  tray integration if the behavior becomes important enough.

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
