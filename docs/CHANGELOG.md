# Changelog

All notable PySentry changes are recorded in this file.

## 0.2.2 - 2026-06-15

- Added Linux desktop integration that installs a user-level `.desktop` file and icon so taskbars can match the running window to the PySentry icon.
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
