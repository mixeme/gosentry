# Pre-Release Milestone Plan

This document tracks the work that takes GoSentry from the v0.4.0 architectural
milestone to a pre-release-ready build. It closes the remaining
[roadmap](ROADMAP.md) items (except packaging), wires up features that were
stubbed during the refactor, and cleans the tree of legacy/rename scaffolding.

The goal is a coherent, end-user-ready build: JSON storage, a real task-queue
policy, working failure notifications, no legacy code, proper icons, and an
end-user-focused README.

Confirmed decisions:

- **Storage:** switch to JSON. One-time YAML import for `gosentry.yaml` /
  `jobs.yaml` (read once, rewrite as JSON). Drop all PySentry legacy entirely.
- **Task queue defaults:** execution mode = Parallel, overlap policy = Skip (both
  selectable in Settings).
- **Fyne 2.7 upgrade:** included (enables tray-click-to-show).
- **Per-job success exit codes:** dropped (exit 0 = OK, non-zero = Failed).

## 1. Switch storage YAML → JSON

- `src/domain/config.go`, `src/domain/job.go`: replace every `yaml:"..."` struct
  tag with `json:"..."` (Config, Job, JobsFile). Keep `omitempty` where used.
- `src/storage/store.go`: replace `writeYAML` with `writeJSON` using
  `encoding/json` (`MarshalIndent(value, "", "  ")` for human-editable files).
  Replace the two `yaml.Unmarshal` calls with `json.Unmarshal`.
- `src/storage/paths.go`: `ConfigFileName = "gosentry.json"`,
  `JobsFileName = "jobs.json"`. Remove `LegacyConfigFileName` (pysentry). Add
  `legacyYAMLConfigFileName = "gosentry.yaml"` and
  `legacyYAMLJobsFileName = "jobs.yaml"` for the import path.
- One-time YAML import in `store.go`:
  - `loadOrCreateConfig`: if `gosentry.json` is absent but `gosentry.yaml`
    exists, `yaml.Unmarshal` it through a private yaml-tagged shadow struct.
    `OpenStore` already calls `SaveConfig` afterward, which writes JSON.
  - `loadOrCreateJobs`: same pattern for `jobs.yaml` via a yaml-tagged shadow;
    `OpenStore`'s `SaveJobs` rewrites as JSON.
  - The old `.yaml` files are left on disk untouched (non-destructive).
- Keep `go.yaml.in/yaml/v4` in `go.mod` (now used only by the import path).
- Tests: `src/storage/store_test.go` — switch the write helper to JSON; replace
  the pysentry migration test with one covering the YAML→JSON one-time import.

## 2. Task-queue execution settings

Reworks the run-dispatch model so the schedule keeps advancing even while a run
is in flight, which is what makes an overlap policy meaningful.

- `src/domain/config.go`: add `ExecutionMode` and `OverlapPolicy` (JSON tags) plus
  exported constants (`ExecutionParallel`/`ExecutionSequential`,
  `OverlapSkip`/`OverlapQueue`). Defaults in `loadOrCreateConfig` and
  `validateConfig`: empty → `parallel` / `skip`.
- `src/domain/runtime.go`: add `Pending bool` to `JobRuntime` (a queued overlap).
- Dispatch logic (moved into `src/app/run.go`, see §9):
  - `startRunLocked`: instead of zeroing `NextDue`, advance it to the next
    occurrence while the display `NextRun` shows `"Running"`, so a due tick can
    arrive during a run.
  - `RunDue`: scan all due jobs. For each due, enabled job:
    - already running → overlap policy: `skip` drops this occurrence; `queue`
      sets `runtime.Pending = true`.
    - not running → execution mode: `parallel` starts it immediately;
      `sequential` starts it only if no job is currently running and none started
      earlier in this tick.
  - `executeRun`: on finish, if `runtime.Pending` and not paused and the mode
    permits, clear `Pending` and start the job again immediately.
  - Add `anyRunningLocked()` helper.
  - `RunNow`: keep the already-running guard; in sequential mode also refuse if
    another job is running.
- `src/ui/settings_view.go`: add a Queue group with `widget.Select` controls for
  execution mode and overlap policy, wired into the saved config.
- Tests: extend `src/app/operations_test.go` for parallel/sequential/skip/queue,
  reusing the fake-`runJob` seam and `StartWith(fakeClock)`.

## 3. Browse button for the Command field

- `src/ui/job_dialog.go`: wrap `commandEntry` in
  `container.NewBorder(nil,nil,nil, browseBtn, commandEntry)` (same pattern as the
  directory rows in `settings_view.go`) and pass the container as the Command
  form item. The button opens a file picker.
- Add a `chooseFile(w, target)` helper in `src/ui/settings_view.go` using
  `dialog.NewFileOpen`; on selection set the entry text to `uri.Path()`.

## 4. System notifications on failure

`Config.NotifyOnFailure` is stored but never acted on. Wire it to Fyne.

- Add `func (s *Service) ShouldNotifyOnFailure() bool` (reads config under `mu`).
- `src/ui/mainwindow.go`: in the existing `svc.Subscribe(...)` handler, when the
  event is `RunRecorded` with `State == "Failed"`, a real-run trigger
  (`Manual`/`Schedule`), and notifications enabled, call
  `fyne.CurrentApp().SendNotification(...)`. The handler is already inside
  `fyne.Do`.
- Update the Settings checkbox wording to drop the "reserved" note.

## 5. Application icons — small vs large

Assets present: `gosentry-icon-16x16.png` (small), `gosentry-icon-big.png`
(large), `gosentry.ico`.

- `assets/assets.go`: also embed the 16×16; add `IconSmall()`. Keep `Icon()`
  (large) for the window/app and `IconBytes()` (large) for Linux desktop
  integration.
- `src/ui/run.go`: window/app icon stays large.
- `src/ui/tray.go`: set the tray icon to the small variant via
  `desk.SetSystemTrayIcon(assets.IconSmall())` (Fyne 2.7).
- Windows Explorer icon stays via `packaging/windows/gosentry.rc`; confirm
  `gosentry.ico` has both a 16×16 and a large frame.

## 6. Fyne 2.6.3 → 2.7.x upgrade + tray click

- `go.mod`: bump `fyne.io/fyne/v2` to latest 2.7.x; `go get` + `go mod tidy`.
- Rebuild under MSYS2 UCRT64 (CGO); skim the 2.7 changelog for breaking changes.
- `src/ui/tray.go`: add `desk.SetSystemTrayWindow(w)` so left-click shows/focuses
  the window; keep the "Show" menu item.
- Re-measure startup using the existing History "Started … in Xms" event and
  append to [PERFORMANCE.md](PERFORMANCE.md).

## 7. Drop legacy + debug; prepare for pre-release

- PySentry removal:
  - Legacy pysentry config path (done in §1).
  - `src/platform/autostart/autostart_windows.go`: remove `legacyAutostartName`,
    `cleanupLegacyRegistryAutostart`, `legacyRegistryAutostartExists`,
    `parseRegistryRunValue`, and their use in `SetAutostart`/`AutostartStatus`.
  - `src/platform/autostart/autostart_linux.go`: remove the legacy systemd +
    desktop cleanup functions and their `Set`/`Status` calls.
  - Delete the corresponding legacy tests; drop `readShortcutTarget` if unused.
  - `.gitignore` / `.dockerignore`: drop the `pysentry.yaml` lines; add
    `gosentry.json` / `jobs.json`; keep the `*.yaml` ignores for the import
    window.
- Debug/diagnostics: confirm no `GOSENTRY_TIMING` code remains (docs only); keep
  the lightweight startup History event (needed for §6).
- Stale artifacts: ensure `dist/` and `cmd/gosentry/*.syso` stay gitignored.
- Bump hardcoded `0.3.0` references to the current `0.4.0`/next pre-release
  version in README/docs.

## 8. README split — end-user vs developer

- New `docs/DEVELOPMENT.md`: move Requirements (toolchain), Build (all variants),
  Run From Source, Project Layout, and Dependencies/mirroring out of README.
- `README.md` keeps end-user content only: intro, Features, Storage, Schedules,
  Using the App, Autostart, Troubleshooting (the VirtualBox/RDP OpenGL workaround
  stays). Update YAML → JSON file names + examples, the new Queue settings, real
  notifications wording, and version strings. Add a Documentation link list to
  `docs/`.

## 9. Roadmap refactoring follow-ups

- Linux test build fix: move the Windows-only tests
  (`TestShellCommandHidesWindow`, `TestShellCommandUsesWindowsSafeQuoting`, and
  peers touching `SysProcAttr`/`windowsShellCommandLine`) from
  `src/runner/runner_test.go` into a new `src/runner/runner_windows_test.go`
  guarded by `//go:build windows`.
- File-size (soft) limits: split the run/dispatch code (`RunDue`, `RunNow`,
  `startRunLocked`, `executeRun`, queue helpers) out of
  `src/app/operations.go` into a new `src/app/run.go`. Optionally split
  `src/ui/jobs_view.go` if a clean seam exists.

## 10. Drop per-job success-exit-codes feature

After removal the run outcome is: exit code 0 → `OK`, any non-zero → `Failed`.

- `src/domain/job.go`: remove the `SuccessExitCodes` field.
- `src/runner/exitcodes.go`: delete the file.
- `src/runner/runner.go` `runStateDetail`: drop the `acceptedExitCode` branch; a
  non-zero `exec.ExitError` is always `Failed` with `Exit code %d`.
- `src/runner/logfile.go`: remove the `success_exit_codes` field.
- `src/app/format.go`: remove `DisplaySuccessExitCodes` (and its test).
- `src/app/operations.go`: drop the `success_exit_codes` lines from
  `runningOutput` and the default in `normalizeJob`.
- `src/storage/store.go` `normalizeJobs`: drop the default.
- `src/ui/job_dialog.go`: remove the entry, form item, and save.
- `src/ui/jobs_view.go`: remove the label and detail row.
- Tests/docs: remove `TestParseExitCodes`,
  `TestRunJobAcceptsConfiguredExitCode`, `TestRunJobRejectsUnconfiguredExitCode`,
  the `success_exit_codes` log-content assertions, the store_test exit-code
  fields/defaults, and the matching `docs/TESTS.md` rows.
- §1 import: legacy YAML jobs may carry `success_exit_codes`; the shadow struct
  ignores it.

## Implementation order

1. Storage JSON + one-time import (§1) and exit-code removal (§10) — both touch
   `domain/job.go` and `storage/store.go`.
2. PySentry removal (§7 autostart + ignores).
3. Queue model + settings (§2) and the `operations.go` → `run.go` split (§9).
4. Notifications (§4), Command Browse (§3).
5. Icons (§5).
6. Fyne 2.7 upgrade + tray click + startup re-measure (§6).
7. Linux test-build fix (§9).
8. README split + docs/version updates (§8, §7).

## Verification

- `go test ./...` must build and pass. Build with CGO under MSYS2 UCRT64 on
  Windows (`scripts\test.bat`); the default Bash env has CGO off. Confirm the
  Linux test build compiles (`GOOS=linux go vet ./...` where available).
- Manual smoke (Windows GUI): build via `scripts\build-windows.bat` and run.
  - First run with an existing `gosentry.yaml`/`jobs.yaml` imports into JSON; a
    fresh install creates JSON defaults.
  - Job dialog: Browse picks a command path.
  - Settings: Queue mode + overlap policy persist; failure notifications toggle;
    a failing job (`exit 1`) raises a desktop notification when enabled.
  - Queue behavior: a fast-schedule long-running job demonstrates skip vs queue;
    parallel runs two due jobs at once; sequential serializes.
  - Tray: left-click shows/focuses the window; tray uses the small icon, window
    and taskbar use the large icon.
- Record the post-upgrade startup time and append to PERFORMANCE.md.
