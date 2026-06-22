# GoSentry Refactoring Plan

Status: proposed — not yet started.
Goal: make the codebase **solid**, **comprehensive**, and **human-readable / maintainable**
without changing observable behavior.

This document is the single source of truth for the refactor. It records the
target architecture, the rationale, and a sequence of small, independently
reviewable tasks. Each task lists the recommended agent model and effort level.

---

## 1. Why refactor

The application works and is well-commented, but its structure does not scale:

| # | Problem | Impact |
|---|---------|--------|
| 1 | `src/gui/app.go` is a 1,057-line monolith | Nothing can be found, reused, or tested in isolation |
| 2 | `src/core` is one flat package mixing 7 concerns | No boundaries; everything can call everything |
| 3 | **Shared mutable `*[]Job`** between GUI and `Scheduler` | GUI mutates the slice with no lock; scheduler locks the same slice → data race |
| 4 | `onChange` mutates Fyne widgets **from the scheduler goroutine** | Latent crash/corruption — Fyne requires UI updates on the main thread |
| 5 | `Job` mixes durable config and runtime state (`yaml:"-"` fields) | The "noise" the model fights to exclude lives in the same struct |
| 6 | Errors swallowed everywhere (`_ = store.SaveJobs(...)`) | Save failures are invisible to the user |
| 7 | No service/controller layer; GUI reaches into `store.Paths`, drives scheduler directly | Business logic is tangled into widget callbacks |
| 8 | Schedule strings re-parsed every tick; no `Schedule` value type | Validation scattered; no single source of truth |
| 9 | Tests only cover `core`; GUI and orchestration untestable | Documented gap in `docs/TESTS.md` |

> Note on layout: the project intentionally **keeps the `src/` directory**. The
> `src/` → `internal/` move was considered and rejected — it is cosmetic for a
> non-imported desktop app and not worth the import-path churn. All packages
> below live under `src/`.

---

## 2. Target architecture

The central change is to **insert an application-service layer** that owns all
state and exposes intent-based methods. This turns the UI into a thin view and
the core packages into stateless engines, dissolving problems 3, 4, 6, and 7.

```
┌──────────────┐   intents    ┌─────────────────┐   calls    ┌──────────────┐
│   ui (Fyne)  │ ───────────▶ │   app.Service   │ ─────────▶ │ core engines │
│  thin views  │ ◀─────────── │  (sole owner of │            │ scheduler /  │
│ fyne.Do only │   events     │  state + mutex) │ ◀───────── │ runner /     │
└──────────────┘              └─────────────────┘  records   │ storage      │
                                                              └──────────────┘
```

- **One writer.** `app.Service` holds the job list + runtime state behind a
  mutex. The UI never mutates state directly — it calls `CreateJob`, `RunNow`,
  `SetGlobalPause`, etc.
- **Events flow back** through an observer interface. The UI's listener is the
  *only* place that touches widgets, and it marshals onto the main thread with
  `fyne.Do`.
- **Core engines are stateless / injected** — scheduler and runner operate on
  data passed in, not a shared slice.

### 2.1 Package layout (all under `src/`)

```
cmd/gosentry/
  main.go                  # flag parse → ui.Run

src/
  domain/                  # pure types, zero external deps
    job.go                 # Job (durable config only — no yaml:"-")
    runtime.go             # JobRuntime (LastRun/NextRun/State/Output/Logs)
    record.go              # RunRecord
    config.go              # Config + StartInTrayArgument
    schedule.go            # Schedule value object: Parse / Validate / Next()

  storage/                 # persistence + path resolution + migration
    store.go               # Load/SaveConfig, Load/SaveJobs
    paths.go               # ResolvePaths
    yaml.go                # writeYAML helper
    migration.go           # pysentry → gosentry legacy handling

  scheduler/
    scheduler.go           # timing loop; drives Service via callbacks
    clock.go               # Clock interface (real + fake for tests)

  runner/
    runner.go              # RunJob orchestration
    invocation.go          # build exec.Cmd (shared)
    invocation_windows.go  # cmd.exe quoting
    invocation_other.go    # sh -c
    exitcodes.go           # parse / accept success codes
    logfile.go             # writeRunLog + sanitizeFileName
    cleanup.go             # CleanupLogs

  platform/
    winproc/               # hidden-window helper shared by runner + autostart
      winproc_windows.go   # CREATE_NO_WINDOW / HideWindow
      winproc_other.go     # no-op
    autostart/
      autostart.go         # Manager interface + Status type
      windows.go linux.go other.go
    desktop/
      desktop_linux.go other.go

  app/
    service.go             # owns state; CreateJob/UpdateJob/Delete/RunNow/...
    events.go              # Event types + Observer registration
    format.go              # display strings (moved out of GUI)

  ui/                      # renamed from src/gui; thin Fyne views
    run.go                 # Run(): lifecycle, window, tray wiring
    mainwindow.go          # tab assembly + event listener (fyne.Do)
    jobs_view.go           # list + details panel + toolbar
    job_dialog.go          # new/edit form
    history_view.go        # history table
    settings_view.go       # settings form
    tray.go                # system tray
    singleinstance.go      # localhost IPC
    layout.go              # minWidthLayout
```

Import paths follow the existing convention, e.g.
`gitea.mixdep.ru/mix/gosentry/src/domain`,
`gitea.mixdep.ru/mix/gosentry/src/app`.

### 2.2 Dependency direction (must stay acyclic)

```
domain   ← (no deps)
storage  ← domain
runner   ← domain, platform/winproc
scheduler← domain
app      ← domain, storage, scheduler, runner
ui       ← app, domain (Fyne)
platform/autostart, platform/desktop ← (own deps; winproc for windows)
cmd      ← ui
```

### 2.3 Key design decisions

1. **Split durable vs. runtime in the domain.** `domain.Job` becomes pure YAML
   config (no `yaml:"-"`). Runtime state moves to `domain.JobRuntime`, held by
   the service keyed by job ID. (Resolves #5.)
2. **`Schedule` value object.** `schedule.Parse(string) (Schedule, error)`
   validates once and exposes `Next(time.Time)`. (Resolves #8.)
3. **Autostart behind a `Manager` interface**, selected per platform — mockable,
   no package-level functions.
4. **Injectable `Clock`** in the scheduler → deterministic tests.
5. **Errors surface to the UI.** Service methods return errors; status bar shows
   them. No more `_ =` on saves. (Resolves #6.)
6. **Thread-safety contract:** core engines never import Fyne; the UI listener is
   the sole widget mutator and always wraps updates in `fyne.Do`. (Resolves #4.)

---

## 3. Task sequence

Tasks are ordered so the tree **compiles and all tests pass after every task**.
Each task is a small, reviewable unit.

**Model guidance**
- `haiku` — mechanical moves, renames, no judgment required.
- `sonnet` — localized logic changes with clear scope.
- `opus` — architecture-shaping work (new layers, concurrency, public APIs).

**Effort guidance** — reasoning depth, not size: `low` / `medium` / `high`.

### Phase 0 — Safety net

| Task | Description | Model | Effort |
|------|-------------|-------|--------|
| T0.1 | Add `scripts/test.sh` + `.bat` running `go vet ./...` and `go test -race ./...`. Document in `docs/TESTS.md`. | haiku | low |
| T0.2 | Add characterization tests that pin current behavior at seams to be moved: store load→save round-trip, scheduler `nextRunTime`, end-to-end `RunJob` log output. (Some exist; fill gaps.) | sonnet | medium |

### Phase 1 — Split the flat `core` package (no logic change)

Mechanical moves + import fixes only. Behavior identical.

| Task | Description | Model | Effort |
|------|-------------|-------|--------|
| T1.1 | Create `src/domain`; move `Job`, `RunRecord`, `Config`, `JobsFile`, `StartInTrayArgument` from `model.go`. Keep `yaml:"-"` fields for now (split happens in Phase 2). Update all references. | sonnet | medium |
| T1.2 | Create `src/platform/winproc`; move `configureHiddenWindow` + hidden-window flags out of `runner_windows.go` / `runner_other.go`. This breaks the future autostart→runner coupling early. | sonnet | medium |
| T1.3 | Create `src/runner`; move `runner.go`, `runner_windows.go`, `runner_other.go`, `runner_test.go`. Point at `winproc`. Split helpers into `invocation*.go`, `exitcodes.go`, `logfile.go`, `cleanup.go` as the file moves. | sonnet | medium |
| T1.4 | Create `src/scheduler`; move `scheduler.go`, `scheduler_test.go`. Still takes `*[]domain.Job` for now. | sonnet | medium |
| T1.5 | Create `src/storage`; move `store.go`, `paths.go`, `store_test.go`. | sonnet | medium |
| T1.6 | Create `src/platform/autostart`; move `autostart_*.go` + tests. Point at `winproc`. | sonnet | medium |
| T1.7 | Create `src/platform/desktop`; move `desktop_linux.go`, `desktop_other.go`. | haiku | low |
| T1.8 | Delete the now-empty `src/core`; run full build + tests on both platforms (or with build tags) to confirm parity. | haiku | low |

### Phase 2 — Domain cleanup

| Task | Description | Model | Effort |
|------|-------------|-------|--------|
| T2.1 | Add `src/domain/schedule.go`: `Schedule` value object with `Parse`, `Validate`, `Next(time.Time)`. Unit-test it. Keep `nextRunTime` as a thin wrapper initially. | opus | high |
| T2.2 | Migrate `scheduler` to use `Schedule` (parse on load/edit, not per tick). Remove duplicated parsing. | sonnet | medium |
| T2.3 | Split `domain.Job` (durable) from `domain.JobRuntime` (transient). Remove all `yaml:"-"` fields and `nextDue` from `Job`. Add `runtime.go`. | opus | high |
| T2.4 | Update `storage`: load/save only `Job`; move runtime initialization out of `normalizeJobs` into a `domain.NewRuntime(job)` constructor. Update round-trip tests. **(Completed as part of T2.3 — removing the runtime fields from `Job` forced all three deliverables. Runtime-map ownership is deferred to T3.1.)** | sonnet | medium |

> After Phase 2 the scheduler and GUI still share state; the `Job`/`JobRuntime`
> split is wired through temporary glue. Phase 3 removes the sharing.

### Phase 3 — Application service layer

| Task | Description | Model | Effort |
|------|-------------|-------|--------|
| T3.1 | Create `src/app/service.go`: `Service` owning `[]domain.Job` + `map[int]*domain.JobRuntime` behind a `sync.Mutex`. Constructor wires `storage`. | opus | high |
| T3.2 | Add `src/app/events.go`: `Event` types (job changed, run recorded, scheduler state) + `Observer` registration. Single-threaded dispatch contract documented. | opus | high |
| T3.3 | Move state-mutating operations into the service: `CreateJob`, `UpdateJob`, `DeleteJob`, `SetEnabled`, `RunNow`, `SetGlobalPause`, `UpdateSettings`. Each returns `error`. | opus | high |
| T3.4 | Convert `scheduler` to operate through the service (no `*[]Job`). Scheduler asks the service for due jobs and reports records back; service is the sole writer. Inject `Clock`. | opus | high |
| T3.5 | Move display/format helpers (`displayFolder`, `displayArguments`, `displayRunMode`, `statusText`, …) from GUI into `src/app/format.go`. | haiku | low |
| T3.6 | Add `src/app` unit tests (no Fyne): create/edit/delete, enable/pause, global pause, run-now path with a fake runner + fake clock. Big coverage win. | opus | high |

### Phase 4 — Carve up the GUI

Rename `src/gui` → `src/ui` and break `app.go` into focused files. The UI now
talks only to `app.Service` and reacts to events via `fyne.Do`.

| Task | Description | Model | Effort |
|------|-------------|-------|--------|
| T4.1 | Rename package `gui` → `ui`; split lifecycle into `run.go` + `mainwindow.go`. Wire the event listener and route every widget update through `fyne.Do`. (Resolves #4.) | opus | high |
| T4.2 | Extract `jobs_view.go` (list + details + toolbar), driven by service calls + events. | sonnet | medium |
| T4.3 | Extract `job_dialog.go`; validate schedule via `domain.Schedule.Validate`. | sonnet | medium |
| T4.4 | Extract `history_view.go`. | sonnet | medium |
| T4.5 | Extract `settings_view.go`; surface save/autostart/cleanup errors to the status label. (Resolves #6 in UI.) | sonnet | medium |
| T4.6 | Extract `tray.go`, `singleinstance.go`, `layout.go`. | haiku | low |
| T4.7 | Confirm `app.go` is gone and `ui` imports only `app` + `domain` + Fyne. Manual smoke test on each platform. | sonnet | medium |

### Phase 5 — Hardening & docs

| Task | Description | Model | Effort |
|------|-------------|-------|--------|
| T5.1 | Replace remaining `_ = ...Save...` with propagated/surfaced errors across service + storage. | sonnet | medium |
| T5.2 | Introduce `autostart.Manager` interface + per-platform impls; inject into the service instead of calling package funcs. | sonnet | medium |
| T5.3 | Fill documented test gaps: folder filtering, log cleanup (count + age), settings persistence/migration, concurrent run prevention. | sonnet | high |
| T5.4 | Run `go test -race ./...` clean. Confirm no data race remains. | haiku | low |
| T5.5 | Update `docs/ARCHITECTURE.md`, `docs/TESTS.md`, and the README "Project Layout" section to the new structure. | sonnet | medium |

---

## 3.1 Task completion checklist

Track progress here. Mark tasks complete as they land and pass review.

### Phase 0 — Safety net
- [x] T0.1 — Add test script + `go vet` + `go test -race`
- [x] T0.2 — Add characterization tests

### Phase 1 — Split flat `core` package
- [x] T1.1 — Create `src/domain`; move Job/RunRecord/Config/etc
- [x] T1.2 — Create `src/platform/winproc`; move `configureHiddenWindow`
- [x] T1.3 — Create `src/runner`; move runner logic
- [x] T1.4 — Create `src/scheduler`; move scheduler
- [x] T1.5 — Create `src/storage`; move store/paths
- [x] T1.6 — Create `src/platform/autostart`; move autostart logic
- [x] T1.7 — Create `src/platform/desktop`; move desktop integration
- [x] T1.8 — Delete empty `src/core`; build + test both platforms

### Phase 2 — Domain cleanup
- [x] T2.1 — Add `src/domain/schedule.go`; Schedule value object
- [x] T2.2 — Migrate `scheduler` to use Schedule
- [x] T2.3 — Split `domain.Job` (durable) from `domain.JobRuntime` (transient)
- [x] T2.4 — Update `storage`: load/save Job only; move runtime init _(landed with T2.3)_

### Phase 3 — Application service layer
- [x] T3.1 — Create `src/app/service.go`; owns state behind mutex
- [x] T3.2 — Add `src/app/events.go`; Event types + Observer
- [x] T3.3 — Add state-mutating operations to service
- [x] T3.4 — Convert `scheduler` to use service; inject Clock
- [x] T3.5 — Move display helpers to `src/app/format.go`
- [x] T3.6 — Add `src/app` unit tests (no Fyne)

### Phase 4 — Carve up the GUI
- [x] T4.1 — Rename `gui` → `ui`; split app.go into run.go + mainwindow.go _(required Fyne v2.5.3→v2.6.3 upgrade for `fyne.Do`)_
- [x] T4.2 — Extract `jobs_view.go`
- [x] T4.3 — Extract `job_dialog.go`
- [x] T4.4 — Extract `history_view.go`
- [x] T4.5 — Extract `settings_view.go`
- [x] T4.6 — Extract `tray.go`, `singleinstance.go`, `layout.go`
- [x] T4.7 — Confirm app.go is gone; smoke test both platforms

### Phase 5 — Hardening & docs
- [x] T5.1 — Surface errors from service + storage
- [x] T5.2 — Introduce `autostart.Manager` interface
- [x] T5.3 — Fill test gaps (folder filtering, cleanup, migration, concurrency)
- [x] T5.4 — Run `go test -race ./...` clean on both platforms
- [x] T5.5 — Update docs (ARCHITECTURE.md, TESTS.md, README)

---

## 4. Definition of done

- `go vet ./...` clean; `go test -race ./...` green on Windows and Linux.
- No package outside `ui` imports Fyne; no engine mutates UI state.
- `domain.Job` has no `yaml:"-"` fields.
- `app.Service` is the only writer of job/runtime state.
- `src/ui` contains no file over ~250 lines; no single file over ~400.
- `docs/ARCHITECTURE.md` matches the shipped structure.

## 5. Risks & mitigations

| Risk | Mitigation |
|------|-----------|
| Cross-platform code moves break the non-host OS build | Build with both `GOOS=windows` and `GOOS=linux` after each platform-touching task (T1.2, T1.3, T1.6, T1.7). |
| Concurrency change (Phase 3/4) introduces subtle deadlocks | Keep the service mutex non-reentrant; never call back into the UI while holding it; cover with `-race` tests in T3.6. |
| Behavior drift during moves | Characterization tests (T0.2) pin behavior before structural change. |
| Large diff hard to review | Each task is a separate commit/PR; phases land independently. |
