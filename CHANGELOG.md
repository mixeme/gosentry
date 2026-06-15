# Changelog

All notable PySentry changes are recorded in this file.

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
