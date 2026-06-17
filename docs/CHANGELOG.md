# Changelog

All notable GoSentry changes are recorded in this file.

## 0.3.1 - 2026-06-17

- Changed startup timing in History to measure until the main window is actually shown instead of stopping during UI construction.
- Added a separate startup History message for autostart launches that begin hidden in the tray.

## 0.3.0 - 2026-06-17

- Renamed the project from PySentry to GoSentry across the GUI, module path, build scripts, generated artifacts, desktop integration, and documentation.
- Renamed the command package to `cmd/gosentry` and Windows resource script to `packaging/windows/gosentry.rc`.
- Renamed portable application settings from `pysentry.yaml` to `gosentry.yaml`, while keeping one-time read compatibility for existing `pysentry.yaml` files.
- Renamed build artifacts from `pysentry-*` to `gosentry-*`.
- Updated autostart and Linux desktop integration to use GoSentry names while cleaning up older PySentry autostart entries.

## 0.2.5 - 2026-06-16

- Stabilized the Jobs details panel so long selected-job fields do not resize the right pane or application window.
- Switched Windows autostart from `HKCU Run` entries to a Startup folder shortcut, fixing executable paths that contain spaces.
- Added `--start-in-tray` autostart launches for Windows and Linux so sign-in startup does not open the main window.
- Added Windows shortcut tests and Linux autostart desktop-entry tests for the new startup-in-tray behavior.
- Updated autostart documentation and architecture notes for the Startup shortcut and XDG desktop-entry behavior.
- Documented the Windows VirtualBox/RDP OpenGL startup failure and the Mesa software OpenGL workaround.

## 0.2.4 - 2026-06-16

- Prevented repeated application launches by forwarding a second start attempt to the already running instance.
- A second instance now asks the first instance to show and focus the existing window, then exits.

## 0.2.3 - 2026-06-15

- Changed History to use chronological ordering with new records appended at the bottom.
- Replaced the History list with a compact table.
- Added Time column sorting in both ascending and descending directions.
- Made History table columns user-resizable through the native Fyne table header.
- Shortened the Log column display to file names instead of full paths.
- Unified UI event timestamps with command run timestamps.

## 0.2.2 - 2026-06-15

- Added Linux desktop integration that installs a user-level `.desktop` file and icon so taskbars can match the running window to the GoSentry icon.
- Added the installed icon path to Linux autostart desktop entries when available.
- Added `ARCHITECTURE.md` with a component interaction diagram and moved project documentation under `docs/`.
- Adjusted the Mermaid architecture diagram to avoid line-break syntax that breaks rendering in Gitea.
- Stabilized the Jobs tab pane layout so switching jobs does not move the divider.
- Added startup timing to the History tab.

## 0.2.1 - 2026-06-15

- Fixed Docker release scripts so container builds keep Go in `PATH`.
- Disabled Go VCS stamping for Docker release builds to avoid failures when `.git` metadata is unavailable inside the container.
- Made Docker release builds write `dist/` artifacts with the current user's UID/GID instead of root ownership.
- Added `ROADMAP.md` with planned delivery formats and packaging priorities.
- Cleaned `.gitignore` for the current Go/Fyne project and kept the local `_gsdata_/` rule.
- Added README links to official Go/Fyne sites and source repositories useful for dependency mirroring.
- Documented Windows dependency installation steps for Go and MSYS2 UCRT64 GCC.

## 0.2.0 - 2026-06-15

- Added working autostart support with status diagnostics in Settings.
- Switched Linux autostart to XDG Autostart `.desktop` files and clean up the legacy user systemd unit.
- Fixed Windows autostart status detection by parsing `HKCU Run` values and comparing executable paths reliably.
- Added background job execution so the GUI does not block while commands run.
- Suppressed Windows console windows for scheduled and manual command runs.
- Added application version display in the window title, Settings, and build artifact names.
- Moved release artifact commands from `Dockerfile` into `scripts/build-release-linux.sh` with interactive target selection.
- Added release build targets for Linux amd64, Linux arm64, and Windows amd64.
- Added README dependency installation notes and official Go/Fyne links.

## 0.1.0 - 2026-06-14

- Added the initial Fyne desktop GUI.
- Added YAML settings and single-file YAML job storage.
- Added `@every` and standard 5-field cron schedules.
- Added manual and scheduled command runs with per-run log files.
- Added job folders, history, global pause, and Windows tray support.
- Added Windows and Linux build helpers.
