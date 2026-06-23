# Release 0.10.0 — Milestone Plan

This milestone bundles the open [roadmap](ROADMAP.md) follow-ups with five
feature/bug-fix requests. It is a polish-and-fill release on top of 0.9.0: no new
storage format and no architectural rework, just per-job run control, run-time
statistics, UI compaction, and screen-fit + packaging groundwork.

Build/test note: the GUI needs CGO + MSYS2 UCRT64; the default Bash env has CGO
off. Use `scripts\test.bat` / `scripts\build-windows.bat` on Windows. Confirm the
Linux test build with `GOOS=linux go vet ./...`.

## 1. Selected Job Activity panel → one line per entry

Today the "Selected job activity" list (`src/ui/jobs_view.go`, `jobLogs`) renders
each record with `app.EventText`, which is wide and can wrap, making rows tall and
the panel noisy.

- `src/ui/jobs_view.go`: in the `jobLogs` update callback, set the row label's
  `Wrapping = fyne.TextTruncate` so each record stays on exactly one line.
- `src/app/format.go`: add a compact `EventLine(e domain.RunRecord) string` (or a
  `oneLine bool` variant) that drops the full log path and uses
  `filepath.Base(e.LogFile)` — see §3. Keep the verbose `EventText` for the
  History tab.
- Tests: `src/app/format_test.go` — cover the one-line formatter, including the
  log-file-present and absent cases.

## 2. Job execution-time statistics

Capture how long each run takes and surface per-job aggregates.

- `src/domain/record.go`: add `DurationMS int64` to `RunRecord` (JSON/struct tag
  consistent with the existing fields).
- `src/runner/runner.go`: measure wall-clock from command start to finish and set
  the duration on the record. For `StartOnly` jobs (fire-and-forget) record `0`
  or omit, since there is no completion to time.
- `src/runner/logfile.go`: write a `duration` line into the log header.
- `src/domain/runtime.go`: add an aggregate (`RunCount`, `FailCount`,
  `LastDurationMS`, `AvgDurationMS`, `MaxDurationMS`) to `JobRuntime`, updated in
  `executeRun` when a record is recorded.
- **Seed stats from log files on startup** so they survive restarts. Add a
  `runner` helper that parses the `duration`/`state` headers of a job's existing
  log files (matched by the `_<sanitized name>.log` suffix, bounded by
  `MaxLogFiles`) and fold the results into the aggregate when `JobRuntime` is
  built. Reuse the §2 log-header format as the parse source. Tolerate older logs
  with no `duration` line (count the run, skip the timing).
- `src/app/format.go`: add `DisplayStats(rt)` returning a one-line summary
  (e.g. `12 runs · 1 failed · last 3.2s · avg 2.8s`).
- `src/ui/jobs_view.go`: add a "Statistics" detail row (and refresh it in
  `updateDetails`).
- Tests: `src/app/format_test.go` for the formatter; extend `src/app/run_test.go`
  to assert the aggregate updates after fake runs; add a `runner` test that seeds
  the aggregate from sample log files (including a duration-less legacy log).

## 3. Fix truncated log file name ("…lo")

The activity/History display shows the full log path and, when the cell is narrow,
truncates from the right so the visible text ends mid-extension (`…\20240101-..lo`).

- Show `filepath.Base(e.LogFile)` (just `20060102-150405_name.log`) instead of the
  full path in the compact formatter from §1; the full path can stay in a tooltip
  or the History row.
- Verify the History tab (`src/ui/history_view.go`) column/truncation so the
  `.log` extension is never clipped to `..lo`.
- Tests: assert the compact formatter emits the base filename.

## 4. Per-job run policy

Currently `ExecutionMode` and `OverlapPolicy` live only on `Config`
(`src/domain/config.go`) and are read globally in `src/app/run.go`. Make the
**overlap policy** configurable per job, falling back to the global default; keep
execution mode global (sequential is inherently a cross-job, one-at-a-time
guarantee and does not have a clean per-job meaning).

- `src/domain/job.go`: add `OverlapPolicy domain.OverlapPolicy \`json:"overlap_policy,omitempty"\``.
  Empty = inherit the global `Config.OverlapPolicy`.
- `src/app/run.go` `RunDue`: resolve the effective policy per job
  (`job.OverlapPolicy` if set, else `s.store.Config.OverlapPolicy`) instead of
  reading the global value once per tick.
- `src/app/operations.go` `normalizeJob` / `src/storage/store.go` `normalizeJobs`:
  leave empty as "inherit" (do not force a default onto the job).
- `src/ui/job_dialog.go`: add an overlap-policy `widget.Select` with an
  "(Use global default)" first option that saves empty.
- `src/ui/settings_view.go`: clarify the global control is the default for jobs
  that don't override.
- `src/app/format.go`: `DisplayRunMode` (or a new helper) reflects the effective
  overlap policy in the details panel.
- Tests: extend `src/app/run_test.go` — a job with `OverlapPolicyQueue` set
  queues even when the global default is `skip`, and vice versa; empty inherits.

## 5. Adapt initial window size for 720p screens

`src/ui/run.go` resizes to `1120×720`. On a 1366×768 / 720p display the title bar
+ taskbar push the window off-screen.

- `src/ui/run.go`: lower the default to a 720p-safe size (e.g. `1024×660`) and set
  a `w.SetFixedSize(false)` sensible `MinSize` on the content so it never demands
  more than fits.
- Optionally persist the last window size via Fyne `Preferences` and restore it on
  launch, clamped to something that fits the current screen.
- Re-check `commandOutputScroll.SetMinSize` (`520×160`) and
  `minJobsSidebarWidth` (480) in `src/ui/jobs_view.go` so the smaller default
  still lays out without forcing horizontal overflow.
- Manual verification on a 1366×768 display (or a forced-resolution VM).

## 6. Roadmap follow-ups (carried from ROADMAP.md)

- **File-size soft limits.** `src/ui/jobs_view.go` (415) and
  `src/app/operations_test.go` (536) exceed the ~250 UI / ~400 cap guideline.
  This milestone adds rows to `jobs_view.go` (§1, §2, §4) — split a clean seam out
  (e.g. the details-panel construction or the toolbar/button wiring) while it is
  already being edited.
- **Post-field-test cleanup.** Sweep for stale diagnostics, over-defensive checks,
  obsolete autostart-migration code, and noisy README setup notes now that 0.9.0
  has had field use. Recheck `.gitignore` / Docker / packaging ignore rules.
  Keep the startup-timing instrumentation (the History "Started … in Xms" event)
  so startup time can keep being measured across future changes.
- **Drop the one-time YAML→JSON migration.** The import shipped in 0.9.0, so the
  transition window has passed. Remove the legacy import path:
  - `src/storage/store.go`: delete the `yamlConfig` / `yamlJob` / `yamlJobsFile`
    shadow structs, `importYAMLConfig` / `importYAMLJobs`, and the legacy-import
    branches in `loadOrCreateConfig` / `loadOrCreateJobs`.
  - `src/storage/paths.go`: remove `legacyYAMLConfigFileName` /
    `legacyYAMLJobsFileName`.
  - `go.mod` / `go.sum`: drop `go.yaml.in/yaml/v4` (now unused) via `go mod tidy`.
  - `src/storage/store_test.go`: remove the YAML-import tests and the `writeYAML`
    helper.
  - `.gitignore` / `.dockerignore`: drop the `*.yaml` import-window ignores.
- **Architecture doc update.** Refresh `docs/ARCHITECTURE.md` for this milestone:
  the per-job overlap policy on `domain.Job` (§4), the run-time statistics added to
  `domain.JobRuntime` and seeded from log files (§2), and any `jobs_view.go` split
  (§6 file-size work).

## 7. Delivery and packaging (portable only)

This milestone targets only the portable distribution variants, matching the
ROADMAP delivery plan. Non-portable installer/package formats are out of scope and
have been dropped from the roadmap.

- Windows portable `.zip` bundling `gosentry.exe`, `README.md`, `CHANGELOG.md`
  (a `scripts\package-windows.*` helper).
- Linux portable `.tar.gz` for `linux-amd64` and `linux-arm64` bundling the
  binary, `README.md`, and `CHANGELOG.md` (a `scripts/package-linux.*` helper).
- Portable builds keep settings and jobs next to the executable — no per-user
  data-path work is needed for this release.

## Implementation order

1. §3 log-name fix + §1 one-line activity (shared compact formatter).
2. §2 execution-time stats (record → runtime aggregate → details row).
3. §4 per-job overlap policy (domain → dispatch → dialog → tests).
4. §5 window sizing.
5. §6 jobs_view split + cleanup (after the §1/§2/§4 edits land).
6. §7 portable archives (Windows `.zip`, Linux `.tar.gz`).
7. Docs: update `docs/ARCHITECTURE.md` (§6); version bump to `0.10.0`
   (`src/app/version.go`), CHANGELOG, ROADMAP tick-offs.

## Verification

- `go vet ./...` clean; `go test ./...` green on Windows (CGO) and Linux.
- Activity panel: each entry is one line; log filename shows the base name with a
  full `.log` extension (no `..lo`).
- Details panel shows live run-time statistics that update after runs.
- A per-job overlap policy overrides the global default; an unset job inherits it.
- The window opens fully visible on a 1366×768 / 720p screen.
- Bump and document the release; append any startup re-measure to PERFORMANCE.md.
