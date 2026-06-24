# Changelog

All notable GoSentry changes are recorded in this file.

## 0.10.0 - 2026-06-24

**Compact activity rows; per-job execution-time statistics seeded from log files.**

**Activity panel (one-line rows):**
- Each entry in the job log list is now a single truncated line using only the
  base name of the log file (e.g. `20260624-120000_Build.log`) instead of the
  full path. Long lines are clipped rather than wrapped, keeping the panel
  compact with many runs.
- History table retains the full log path for reference; base-name truncation
  applies only to the activity rows in the Jobs details panel.

**Execution-time statistics:**
- Added `DurationMS` field to `RunRecord`; the runner measures wall-clock
  start-to-finish and writes it to both the record and a `duration:` header
  line in the log file. `StartOnly` jobs record `0`.
- Added aggregate counters to `JobRuntime`: `RunCount`, `FailCount`,
  `LastDurationMS`, `AvgDurationMS`, `MaxDurationMS`. Updated after every
  completed run in `executeRun`.
- On startup the statistics are seeded from existing log files: the runner
  parses `state:` and `duration:` headers for each job's newest log files
  (bounded by `MaxLogFiles`). Legacy logs without a `duration:` line still
  count toward `RunCount`/`FailCount` but are excluded from duration
  aggregates so a missing duration cannot appear as a zero-millisecond run.
- A **Statistics** row in the Jobs details panel shows a one-line summary
  (`N runs, M failed, last X ms, avg Y ms, max X ms`) that refreshes after
  each run and is pre-populated from log files after a restart.

## 0.9.0 - 2026-06-24

**Storage migrated to JSON; queue execution modes; failure notifications; tray left-click; Fyne 2.7.4.**

**Storage and data model:**
- Settings and jobs now stored as `gosentry.json` and `jobs.json` (2-space indented JSON).
  On first run after upgrading, existing `gosentry.yaml` / `jobs.yaml` files are imported
  automatically and rewritten as JSON; the YAML files are not deleted.
- Removed `SuccessExitCodes` field. Exit-code handling is now fixed: exit code 0 = success,
  any nonzero exit code = failure. Jobs relying on nonzero success codes must switch to
  `StartOnly` mode if the exit code is irrelevant.

**Execution modes and overlap policies:**
- Added `ExecutionMode` (parallel/sequential) and `OverlapPolicy` (skip/queue) settings in
  Settings under a new Queue group.
  - **Parallel mode** (default): all due jobs start simultaneously.
  - **Sequential mode**: due jobs run one at a time, in order.
  - **Skip policy** (default): if a job comes due while its previous run is still active, the new
    run is discarded.
  - **Queue policy**: if a job comes due while running, the run is held and automatically started
    when the current run completes.
- Both settings are persisted to `gosentry.json` and validated on load; defaults ensure
  backward compatibility.

**Notifications and command input:**
- Failed job runs now raise a desktop notification (when enabled in Settings) with the job name
  and failure detail. Notifications fire for scheduled and manual runs; internal activity events
  are not notified.
- Added a Browse button next to the Command field in the job dialog for file picker selection.

**UI and platform integration:**
- Removed all PySentry legacy code: registry autostart entries (Windows), systemd and desktop
  file cleanup (Linux).
- Updated `.gitignore` and `.dockerignore` to track `gosentry.json` / `jobs.json` instead of
  legacy YAML filenames; added `*.yaml` wildcard to ignore legacy files during import.
- Moved developer documentation (Requirements, Build, Run From Source, Project Layout, Dependencies)
  out of README into `docs/DEVELOPMENT.md`. README now focuses on end-user content.

**Icons and tray:**
- Regenerated all icon assets with feathered color-to-alpha so the rounded-tile boundary is
  transparent — the opaque white halo visible on dark taskbars and trays is gone.
- Rebuilt `gosentry.ico` as a multi-size file (16 hand-tuned + 32/48/256 from the large PNG)
  and added a dedicated 16×16 icon for the Windows tray.
- Per-platform icon wiring: Windows window/taskbar uses the ICO resource so GLFW selects the
  right frame per slot; Windows tray uses the 16×16 ICO; Linux titlebar uses `IconSmall()` for
  a crisp ~16 px `_NET_WM_ICON`.
- Left-clicking the tray icon now shows and focuses the main window without opening the menu;
  the explicit "Show" menu item is preserved for right-click access.

**Performance:**
- Upgraded Fyne 2.6.3 → 2.7.4 (systray 1.11.0 → 1.12.1): startup time drops from ~644 ms to
  ~414 ms (−36%).
- Moved Windows-only runner tests into `runner_windows_test.go` (guarded by `//go:build windows`)
  to fix Linux test build.

## 0.8.0 - 2026-06-23

**Desktop notifications for failed jobs; Browse button for command paths.**

- Failed job runs now raise a desktop notification (title "GoSentry: Job Failed", body shows the job name and failure detail) when the "Show desktop notifications for failed jobs" setting is enabled. Notifications fire for scheduled and manual runs only; internal activity events are not notified.
- Added a Browse button next to the Command field in the job dialog so users can pick an executable from a file picker instead of typing the full path.

## 0.7.0 - 2026-06-23

**Execution modes and overlap policies for parallel and sequential job dispatch.**

- Added `ExecutionMode` setting (parallel/sequential) and `OverlapPolicy` setting (skip/queue) in Settings under a new Queue group.
  - **Parallel mode** (default): all due jobs start simultaneously.
  - **Sequential mode**: due jobs run one at a time, in order; a new job waits for the previous one to finish.
  - **Skip policy** (default): if a job comes due again while its previous run is still active, the new run is discarded.
  - **Queue policy**: if a job comes due again while running, the run is held and automatically started when the current run completes.
- Both settings are persisted to `gosentry.json` and validated on load; defaults ensure backward compatibility with existing installations.
- Added comprehensive unit tests verifying parallel start, sequential serialization, skip drops, and queue re-runs.
- Manual runs (`RunNow`) respect sequential mode: refused while any other job is running.
- No observable behavior changes with default (parallel/skip) settings; installations upgrading from earlier versions continue unchanged.

## 0.6.0 - 2026-06-22

**PySentry legacy code removed.**

- Removed all PySentry registry autostart entries (Windows), systemd and desktop file cleanup (Linux), and associated legacy code paths.
- Updated `.gitignore` and `.dockerignore` to ignore `gosentry.json` / `jobs.json` instead of the old YAML filenames;
  added `*.yaml` wildcard to ignore legacy files during the import window.
- No observable behavior changes; codebase cleanup after migration from PySentry naming.

## 0.5.0 - 2026-06-22

**Storage migrated from YAML to JSON; exit-code flexibility removed.**

- Settings and jobs are now stored as `gosentry.json` and `jobs.json` (2-space indented JSON).
  On first run after upgrading, existing `gosentry.yaml` / `jobs.yaml` files are imported
  automatically and the JSON files are written; the YAML files are not deleted and can be
  removed manually.
- Removed `SuccessExitCodes` field from jobs. Exit-code handling is now fixed: exit code 0 is
  success, any nonzero exit code is failure. Jobs that relied on nonzero success codes will now
  show "Failed"; update those jobs to use `StartOnly` mode if the exit code is irrelevant.
- Deleted `runner/exitcodes.go`; simplified `runStateDetail` accordingly.
- Tests updated: JSON round-trip tests, YAML-import tests for both config and jobs,
  exit-code flexibility tests removed.

## 0.4.0 - 2026-06-22

**Architectural milestone: completed refactoring and reached target architecture.**

- Completed Phase 5 refactoring: hardening, testing, and documentation.
  - Surface all save/cleanup errors from service and storage; no more silently swallowed `_ = ...` on persistence.
  - Introduced `platform/autostart.Manager` interface with per-platform implementations (Windows, Linux, other); inject into service instead of calling package functions.
  - Filled test gaps: folder filtering, log cleanup (count and age), settings persistence and migration, concurrent run prevention.
  - Verified `go test -race ./...` passes on Windows; no data races in the refactored codebase.
  - Updated `docs/ARCHITECTURE.md`, `docs/TESTS.md`, and README with final package structure and build/test instructions.
- **Refactoring target reached:** Service layer owns all state and is the sole writer; UI is a thin view marshaling updates via `fyne.Do`; core engines are stateless and injectable; domain layer is pure with no test noise.
- Known follow-ups recorded in `ROADMAP.md`:
  - Linux test build is currently broken (Windows-only test symbols need `//go:build windows`); will fix separately.
  - File-size soft limits exceeded in a few places; revisit when next editing those files.
- No observable behavior changes.

## 0.3.6 - 2026-06-22

- Completed Phase 4 refactoring: carved up the GUI into focused, testable components.
  - Renamed `src/gui` → `src/ui` and split monolithic `app.go` into `run.go` (lifecycle) and `mainwindow.go` (view construction).
  - Extracted view components into separate files: `jobs_view.go`, `job_dialog.go`, `history_view.go`, `settings_view.go`.
  - Extracted platform wiring into separate files: `tray.go`, `singleinstance.go`, `layout.go`.
  - Removed forbidden platform imports (autostart, desktop, paths) from `src/ui`; all platform concerns now flow through `app.Service`.
  - Upgraded Fyne from v2.5.3 to v2.6.3 to enable `fyne.Do` for cross-thread widget marshaling (resolves concurrency issue #4).
- Added `docs/PERFORMANCE.md` with measured startup-time analysis: the ~290ms increase from Phase 4 is entirely the Fyne 2.6.3 upgrade's `w.Show()` cost, not the restructuring.
- Added `docs/PERFORMANCE.md` and wired post-Fyne-2.7.x re-check into `ROADMAP.md`.
- No observable behavior changes; continued internal refactoring toward separated concerns and testability.

## 0.3.5 - 2026-06-19

- Completed Phase 3 refactoring: application service and state management.
  - Added `app.Service` as the single owner of application state (job registry, settings, run history).
  - Implemented event-driven observer dispatch: Services can emit events (JobAdded, JobChanged, etc.) to decouple state changes from UI updates.
  - Added `app.Clock` interface for testable time-dependent behavior in scheduler and run tracking.
  - Converted scheduler to drive app.Service instead of directly managing domain state.
  - Created `app.Format` helpers for display rendering (job names, schedule summaries, run times).
  - Added comprehensive unit tests for app.Service and supporting types.
- No observable behavior changes; continued internal refactoring toward separated concerns and testability.

## 0.3.4 - 2026-06-19

- Completed Phase 2 refactoring: domain cleanup and value object extraction.
  - Split durable job configuration (`domain.Job`) from transient execution state (`domain.JobRuntime`), keyed by job ID.
  - Added `domain.Schedule` value object with `Parse`, `Validate`, and `Next(time.Time)` methods for cron/interval parsing.
  - Migrated scheduler to parse schedules once at load/edit instead of per tick, removing duplicated parsing.
  - Made `RunJob` pure: runner no longer mutates jobs, returning only `RunRecord` for the caller to fold into runtime state.
  - Simplified `storage.normalizeJobs` to touch only durable configuration; runtime initialization moved to `domain.NewRuntime`.
- No observable behavior changes; continued internal refactoring toward separated concerns.

## 0.3.3 - 2026-06-18

- Completed Phase 1 refactoring: split the flat `src/core` package into specialized, focused packages:
  - `src/domain` for pure types (Job, RunRecord, Config)
  - `src/storage` for persistence (Load/Save, Paths, YAML helpers)
  - `src/runner` for job execution (RunJob orchestration, logging, exit codes)
  - `src/scheduler` for timing loop
  - `src/platform/winproc` for cross-platform hidden window configuration
  - `src/platform/autostart` for system autostart integration
  - `src/platform/desktop` for desktop environment integration
  - `src/app` for application-level code (Version, future Service layer)
- No observable behavior changes; internal structure improvements only.

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
