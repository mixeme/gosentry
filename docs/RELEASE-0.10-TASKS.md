# Release 0.10.0 — Task List

Execution checklist for [RELEASE-0.10-PLAN.md](RELEASE-0.10-PLAN.md). Each task
names the recommended model and thinking depth. `Model`: haiku / sonnet / opus.
`Thinking`: low / medium / high. Section numbers (§) reference the plan.

Build/test note: the GUI needs CGO + MSYS2 UCRT64; the default Bash env has CGO
off. Use `scripts\test.bat` / `scripts\build-windows.bat` on Windows; confirm the
Linux test build with `GOOS=linux go vet ./...`.

## Phase 1 — Log-name fix + one-line activity (§3, §1)

Done first because both share a compact, single-line record formatter.

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| T1.1 | `app/format.go`: add a compact one-line formatter (`EventLine`) that drops the full log path and uses `filepath.Base(e.LogFile)`; keep verbose `EventText` for History. | sonnet | low |
| T1.2 | `ui/jobs_view.go`: set the `jobLogs` row label `Wrapping = fyne.TextTruncate` and render via the compact formatter so each entry is one line. | sonnet | low |
| T1.3 | `ui/history_view.go`: verify column width/truncation never clips `.log` to `..lo`; keep the full path available (tooltip/row). | sonnet | low |
| T1.4 | `app/format_test.go`: cover the compact formatter (log-file present/absent, base-name output). | haiku | low |

## Phase 2 — Job execution-time statistics (§2)

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| T2.1 | `domain/record.go`: add `DurationMS int64` (tag consistent with existing fields). | haiku | low |
| T2.2 | `runner/runner.go`: measure wall-clock start→finish and set duration on the record; `StartOnly` jobs record `0`. | sonnet | medium |
| T2.3 | `runner/logfile.go`: write a `duration` line into the log header. | haiku | low |
| T2.4 | `domain/runtime.go` + `app/run.go`: add the aggregate (`RunCount`, `FailCount`, `LastDurationMS`, `AvgDurationMS`, `MaxDurationMS`); update it in `executeRun`. | sonnet | medium |
| T2.5 | Seed stats from existing log files on startup: a `runner` helper that parses `duration`/`state` headers (suffix-matched, bounded by `MaxLogFiles`, tolerant of duration-less logs), folded into `JobRuntime` at build time. | opus | high |
| T2.6 | `app/format.go` `DisplayStats(rt)` one-line summary + `ui/jobs_view.go` "Statistics" detail row (refreshed in `updateDetails`). | sonnet | medium |
| T2.7 | Tests: `format_test.go` (DisplayStats), `app/run_test.go` (aggregate updates after fake runs), `runner` seed test incl. a duration-less legacy log. | sonnet | medium |

## Phase 3 — Per-job run policy (§4)

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| T3.1 | `domain/job.go`: add `OverlapPolicy` (`json:"overlap_policy,omitempty"`); empty = inherit global. | haiku | low |
| T3.2 | `app/run.go` `RunDue`: resolve the effective policy per job (job value else `Config.OverlapPolicy`); `normalizeJob`/`normalizeJobs` leave empty as inherit. | opus | high |
| T3.3 | `ui/job_dialog.go`: overlap-policy `widget.Select` with "(Use global default)" → empty; `settings_view.go` wording; `app/format.go` reflects effective policy in details. | sonnet | medium |
| T3.4 | `app/run_test.go`: per-job `queue` overrides global `skip` (and vice versa); empty inherits. | opus | high |

## Phase 4 — Persist global pause state (§6)

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| T4.1 | `domain/config.go`: add `Paused bool` (`json:"paused,omitempty"`). | haiku | low |
| T4.2 | `app/operations.go` `SetGlobalPause`: persist into `s.store.Config` + `SaveConfig`. `app/service.go`: init `s.paused` from `Config.Paused` and apply paused next-run text at startup. | sonnet | medium |
| T4.3 | `ui/jobs_view.go`: init `schedulerPaused`, the Pause-all/Resume-all button, and the scheduler-state label from the persisted state. | sonnet | low |
| T4.4 | `app/operations_test.go`: `SetGlobalPause(true)` persists; a Service rebuilt from that store starts paused and refuses `RunDue`/`RunNow`. | sonnet | medium |

## Phase 5 — Window sizing (§5)

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| T5.1 | `ui/run.go`: lower default to a 720p-safe size (~`1024×660`) + sensible `MinSize`; re-check `commandOutputScroll` min size and `minJobsSidebarWidth` in `jobs_view.go`. Manual check on 1366×768. | sonnet | medium |

## Phase 6 — Refactor + cleanup (§7)

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| T6.1 | `ui/jobs_view.go`: split a clean seam (details-panel build or toolbar/button wiring) to bring it back under the size guideline after the §1/§2/§4/§6 edits. | sonnet | medium |
| T6.2 | Post-field-test cleanup sweep: stale diagnostics, over-defensive checks, obsolete autostart-migration code, noisy README notes, ignore rules. **Keep** the startup-timing History event. | sonnet | medium |
| T6.3 | Drop the one-time YAML→JSON import: shadow structs + `importYAML*` + legacy branches in `storage/store.go`; legacy names in `paths.go`; `go.yaml.in/yaml/v4` via `go mod tidy`; YAML-import tests + `writeYAML` helper; `*.yaml` ignore rules. | sonnet | medium |
| T6.4 | `docs/ARCHITECTURE.md`: document per-job overlap policy, run-time statistics (incl. log-file seeding), the persisted pause flag, and the `jobs_view.go` split. | sonnet | medium |

## Phase 7 — Portable packaging (§8)

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| T7.1 | `scripts\package-windows.*`: build + bundle `gosentry.exe`, `README.md`, `CHANGELOG.md` into a portable `.zip`. | sonnet | medium |
| T7.2 | `scripts/package-linux.*`: build + bundle binary, `README.md`, `CHANGELOG.md` into `.tar.gz` for `linux-amd64` and `linux-arm64`. | sonnet | medium |

## Phase 8 — Release docs + version

| Task | Description | Model | Thinking |
|------|-------------|-------|----------|
| T8.1 | Bump `src/app/version.go` to `0.10.0`; update `docs/CHANGELOG.md`; tick the addressed `docs/ROADMAP.md` items; append any startup re-measure to `docs/PERFORMANCE.md`. | haiku | low |

---

## Completion checklist

### Phase 1 — Log-name + one-line activity
- [x] T1.1 — compact one-line formatter
- [x] T1.2 — `jobLogs` single-line rows
- [x] T1.3 — History `.log` truncation verified
- [x] T1.4 — formatter tests

### Phase 2 — Execution-time statistics
- [ ] T2.1 — `DurationMS` on `RunRecord`
- [ ] T2.2 — measure duration in runner
- [ ] T2.3 — `duration` log header
- [ ] T2.4 — runtime aggregate + `executeRun` update
- [ ] T2.5 — seed stats from log files
- [ ] T2.6 — `DisplayStats` + Statistics row
- [ ] T2.7 — stats tests

### Phase 3 — Per-job run policy
- [ ] T3.1 — `Job.OverlapPolicy` field
- [ ] T3.2 — effective-policy dispatch + inherit
- [ ] T3.3 — dialog select + settings/format wording
- [ ] T3.4 — per-job override tests

### Phase 4 — Persist global pause state
- [ ] T4.1 — `Config.Paused` field
- [ ] T4.2 — persist in `SetGlobalPause` + init from config
- [ ] T4.3 — UI inits from persisted state
- [ ] T4.4 — persistence + restored-paused tests

### Phase 5 — Window sizing
- [ ] T5.1 — 720p-safe default + MinSize

### Phase 6 — Refactor + cleanup
- [ ] T6.1 — `jobs_view.go` split
- [ ] T6.2 — post-field-test cleanup (keep startup timing)
- [ ] T6.3 — drop YAML→JSON migration
- [ ] T6.4 — ARCHITECTURE.md update

### Phase 7 — Portable packaging
- [ ] T7.1 — Windows `.zip`
- [ ] T7.2 — Linux `.tar.gz` (amd64 + arm64)

### Phase 8 — Release docs + version
- [ ] T8.1 — version bump + CHANGELOG + ROADMAP + PERFORMANCE

## Definition of done

- `go vet ./...` clean; `go test ./...` green on Windows (CGO) and Linux.
- Activity panel entries are one line; log file names show the base name with a
  full `.log` extension (no `..lo`).
- Details panel shows run-time statistics that update after runs and survive a
  restart (seeded from log files).
- A per-job overlap policy overrides the global default; an unset job inherits it.
- "Pause all" survives a restart: a paused install relaunches paused.
- The window opens fully visible on a 1366×768 / 720p screen.
- No YAML→JSON import code remains; `go.yaml.in/yaml/v4` is gone from `go.mod`.
- Portable `.zip` and `.tar.gz` artifacts build; ARCHITECTURE.md, CHANGELOG, and
  ROADMAP reflect the release; version is `0.10.0`.
