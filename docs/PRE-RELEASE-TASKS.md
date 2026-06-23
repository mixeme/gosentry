# Pre-Release Milestone — Task List

Execution checklist for [PRE-RELEASE-PLAN.md](PRE-RELEASE-PLAN.md). Each task names
the recommended model and thinking depth. `Model`: haiku / sonnet / opus.
`Thinking`: low / medium / high. Section numbers (§) reference the plan.

Build/test note: the GUI needs CGO + MSYS2 UCRT64; the default Bash env has CGO
off. Use `scripts\test.bat` / `scripts\build-windows.bat` on Windows.

## Phase 1 — Storage JSON + exit-code removal (§1, §10)

These land together because both edit `domain/job.go` and `storage/store.go`.

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| P1.1 | `domain/job.go`, `domain/config.go`, `domain.JobsFile`: swap `yaml:"…"` tags for `json:"…"`, keep `omitempty`. | sonnet | low |
| P1.2 | `storage/store.go`: replace `writeYAML` with `writeJSON` (`json.MarshalIndent` 2-space); switch the two `Unmarshal` calls to `json`. | opus | high |
| P1.3 | `storage/paths.go`: rename files to `gosentry.json` / `jobs.json`; remove `LegacyConfigFileName`; add `legacyYAMLConfigFileName` / `legacyYAMLJobsFileName`. | sonnet | medium |
| P1.4 | One-time YAML import in `store.go`: read `gosentry.yaml` / `jobs.yaml` via private yaml-tagged shadow structs when JSON is absent; rely on existing `SaveConfig`/`SaveJobs` to rewrite as JSON. | opus | high |
| P1.5 | Drop `SuccessExitCodes` field; delete `runner/exitcodes.go`; simplify `runStateDetail` (0 = OK, non-zero = Failed); strip the field from `runner/logfile.go`, `app/format.go`, `app/operations.go` (`runningOutput`, `normalizeJob`), `storage/store.go` (`normalizeJobs`). | sonnet | medium |
| P1.6 | Update tests/docs: `storage/store_test.go` (JSON write helper + YAML-import test, drop pysentry-migration test), remove exit-code tests in `runner/runner_test.go` + `app/format_test.go`, update `docs/TESTS.md`. | sonnet | medium |

## Phase 2 — PySentry legacy removal (§7)

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| P2.1 | `autostart_windows.go`: remove `legacyAutostartName`, `cleanupLegacyRegistryAutostart`, `legacyRegistryAutostartExists`, `parseRegistryRunValue`, and their use in `Set`/`Status`; drop `readShortcutTarget` if unused. | sonnet | medium |
| P2.2 | `autostart_linux.go`: remove legacy systemd + desktop cleanup functions and their `Set`/`Status` calls. | sonnet | medium |
| P2.3 | Delete the legacy tests in `autostart_windows_test.go` / `autostart_linux_test.go`. | haiku | low |
| P2.4 | `.gitignore` / `.dockerignore`: drop `pysentry.yaml`; add `gosentry.json` / `jobs.json`; keep `*.yaml` ignores for the import window. | haiku | low |

## Phase 3 — Task-queue model + settings (§2, §9 split)

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| P3.1 | `domain/config.go`: add `ExecutionMode` / `OverlapPolicy` (+ JSON tags) and exported constants; defaults (`parallel` / `skip`) in `loadOrCreateConfig` and `validateConfig`. `domain/runtime.go`: add `Pending bool`. | sonnet | medium |
| P3.2 | Create `src/app/run.go`; move `RunDue`, `RunNow`, `startRunLocked`, `executeRun` and helpers out of `operations.go`. | sonnet | medium |
| P3.3 | Rework dispatch: `startRunLocked` advances `NextDue` instead of zeroing it; `RunDue` scans all due jobs and applies execution mode (parallel/sequential) + overlap policy (skip/queue); `executeRun` re-runs a `Pending` job; add `anyRunningLocked`; sequential guard in `RunNow`. | opus | high |
| P3.4 | `ui/settings_view.go`: add a Queue group with `widget.Select` for execution mode and overlap policy, wired into the saved config. | sonnet | medium |
| P3.5 | Extend `app/operations_test.go` (or new `run_test.go`): parallel multi-start, sequential serialization, skip drops overlap, queue re-runs after finish. Reuse fake-`runJob` + `StartWith(fakeClock)`. | opus | high |

## Phase 4 — Notifications + Command browse (§4, §3)

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| P4.1 | Add `Service.ShouldNotifyOnFailure()` (reads config under `mu`); in `ui/mainwindow.go` listener, `SendNotification` on a failed real-run when enabled. Update Settings wording. | sonnet | medium |
| P4.2 | `ui/job_dialog.go`: wrap Command entry with a Browse button; add `chooseFile` helper (`dialog.NewFileOpen`) in `settings_view.go`. | sonnet | low |

## Phase 5 — Icons (§5)

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| P5.1 | `assets/assets.go`: embed `gosentry-icon-16x16.png`; add `IconSmall()`; keep `Icon()`/`IconBytes()` (large). | haiku | low |
| P5.2 | `ui/tray.go`: `desk.SetSystemTrayIcon(assets.IconSmall())`; confirm window/app icon stays large in `run.go`. Verify `gosentry.ico` carries both sizes. | sonnet | low |

## Phase 6 — Fyne 2.7 upgrade + tray click (§6)

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| P6.1 | `go.mod`: bump `fyne.io/fyne/v2` to 2.7.x (`go get` + `go mod tidy`); rebuild under MSYS2 UCRT64; review 2.7 changelog for breaking changes. | opus | high |
| P6.2 | `ui/tray.go`: add `desk.SetSystemTrayWindow(w)` for left-click-to-show; keep the "Show" menu item. | sonnet | low |
| P6.3 | Re-measure startup via the History "Started … in Xms" event; append the result to `docs/PERFORMANCE.md`. | haiku | low |

## Phase 7 — Roadmap follow-ups (§9)

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| P7.1 | Move Windows-only runner tests (`TestShellCommandHidesWindow`, `TestShellCommandUsesWindowsSafeQuoting`, peers touching `SysProcAttr`/`windowsShellCommandLine`) into `runner/runner_windows_test.go` guarded by `//go:build windows`; confirm Linux test build compiles. | sonnet | medium |

## Phase 8 — Docs + version (§8, §7)

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| P8.1 | New `docs/DEVELOPMENT.md`: move Requirements, Build, Run From Source, Project Layout, Dependencies/mirroring out of README. | sonnet | medium |
| P8.2 | Rewrite `README.md` for end users: Features, Storage (JSON), Schedules, Using the App, Queue settings, notifications, Autostart, Troubleshooting; add a Documentation link list; fix `0.3.0` → current version. | sonnet | medium |
| P8.3 | Update `docs/CHANGELOG.md`; mark the addressed `docs/ROADMAP.md` items done. | haiku | low |

---

## Completion checklist

### Phase 1 — Storage JSON + exit-code removal
- [x] P1.1 — JSON struct tags
- [x] P1.2 — `writeJSON` + JSON unmarshal
- [x] P1.3 — `gosentry.json` / `jobs.json` paths; drop pysentry name
- [x] P1.4 — One-time YAML import
- [x] P1.5 — Remove `SuccessExitCodes` across code
- [x] P1.6 — Update storage/runner/format tests + TESTS.md

### Phase 2 — PySentry legacy removal
- [x] P2.1 — Windows autostart legacy code
- [x] P2.2 — Linux autostart legacy code
- [x] P2.3 — Delete legacy autostart tests
- [x] P2.4 — `.gitignore` / `.dockerignore`

### Phase 3 — Task-queue model + settings
- [x] P3.1 — Config/runtime fields + defaults
- [x] P3.2 — Split dispatch into `app/run.go`
- [ ] P3.3 — Rework `RunDue`/`executeRun` for mode + overlap policy
- [ ] P3.4 — Settings Queue selects
- [ ] P3.5 — Queue tests

### Phase 4 — Notifications + Command browse
- [ ] P4.1 — Failure notifications
- [ ] P4.2 — Command Browse button

### Phase 5 — Icons
- [ ] P5.1 — Embed small icon + `IconSmall()`
- [ ] P5.2 — Tray uses small icon; verify `.ico`

### Phase 6 — Fyne 2.7 upgrade + tray click
- [ ] P6.1 — Bump Fyne to 2.7.x; rebuild
- [ ] P6.2 — `SetSystemTrayWindow` left-click-to-show
- [ ] P6.3 — Re-measure startup → PERFORMANCE.md

### Phase 7 — Roadmap follow-ups
- [ ] P7.1 — Linux test build fix (build-tagged Windows tests)

### Phase 8 — Docs + version
- [ ] P8.1 — `docs/DEVELOPMENT.md`
- [ ] P8.2 — End-user README rewrite
- [ ] P8.3 — CHANGELOG + ROADMAP updates

## Definition of done

- `go vet ./...` clean; `go test ./...` green on Windows and Linux (CGO on).
- Storage reads/writes JSON; an existing `*.yaml` install imports once into JSON.
- Queue execution mode + overlap policy are configurable and exercised by tests.
- A failed real run raises a desktop notification when enabled.
- Tray left-click shows the window; tray uses the small icon, window/taskbar the
  large one.
- No PySentry legacy code remains; README is end-user-focused with dev docs split
  into `docs/DEVELOPMENT.md`.
