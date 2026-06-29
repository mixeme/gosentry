# GoSentry Architecture

This document shows the current component interaction model. GoSentry is a
single desktop process: the GUI, application service, scheduler, storage, and
command runner live in one application. They communicate through typed events
and well-defined interfaces rather than shared mutable state.

## Package Map

```
cmd/gosentry            entry point — starts the UI
src/
  domain/               pure value types: Job, Config, RunRecord, Schedule, JobRuntime
  app/                  Service — sole owner of job/runtime state; emits typed Events
  scheduler/            pure timing loop; calls app.Service.RunDue on every tick
  runner/               shell command execution + log file writing + cleanup
  storage/              JSON persistence (gosentry.json, jobs.json)
  platform/
    autostart/          Manager interface + Windows (shortcut) and Linux (XDG) impls
    desktop/            display-scale helper (Linux only)
    winproc/            hidden-window startup flags (Windows only)
  ui/                   Fyne windows, tabs, and dialogs; reads service via Events
```

## Component Diagram

```mermaid
flowchart LR
    user["Desktop user"]
    ui["src/ui\nFyne windows, tabs, dialogs"]
    svc["src/app Service\nsole owner of job + runtime state"]
    store["src/storage Store\nJSON config and jobs"]
    sched["src/scheduler Scheduler\npure timing loop"]
    runner["src/runner\nshell command execution"]
    autostart["src/platform/autostart Manager\nWindows shortcut / Linux XDG"]
    config["gosentry.json\napplication settings"]
    jobs["jobs.json\njob definitions"]
    logs["logs_dir\nper-run command output logs"]
    shell["Platform shell\ncmd.exe /C or sh -c"]

    user -->|"edits jobs, settings, runs commands"| ui
    ui -->|"CreateJob, UpdateJob, DeleteJob, RunNow, UpdateSettings, …"| svc
    svc -->|"SaveJobs, SaveConfig, LoadJobs, LoadConfig"| store
    store -->|"read/write"| config
    store -->|"read/write"| jobs

    svc -->|"Start(RunDue)"| sched
    sched -->|"RunDue(now)"| svc
    svc -->|"RunJob"| runner
    runner -->|"execute command"| shell
    runner -->|"write stdout/stderr log"| logs
    runner -->|"RunRecord"| svc
    svc -->|"emit JobChanged / RunRecorded / ErrorOccurred"| ui
    ui -->|"display jobs, history, status"| user

    ui -->|"SetAutostart, AutostartStatus"| autostart
    svc -->|"Set / Status via Manager"| autostart
```

## Main Flows

1. Startup:
   `cmd/gosentry` calls `ui.Run`, which creates an `app.Service`, opens the
   store, loads `gosentry.json` and `jobs.json`, subscribes the UI to service
   events, builds the main window, and calls `Service.Start` to begin the
   scheduler loop. On first launch the service seeds per-job run-time statistics
   from existing log files so the details panel reflects accumulated history
   immediately (see §Statistics below).

2. Editing settings or jobs:
   The UI calls mutating methods on `app.Service` (e.g. `CreateJob`,
   `UpdateJob`, `UpdateSettings`). The Service validates the request, updates
   its in-memory state, persists through `storage.Store`, and emits a typed
   `Event`. The UI's observer receives the event and refreshes the relevant
   widget on the main thread via `fyne.Do`.

3. Scheduled run:
   `scheduler.Scheduler` fires a tick every second. On each tick it calls
   `Service.RunDue(now)`. The Service checks which enabled, non-paused jobs are
   due, marks each as running, and launches `runner.RunJob` in a goroutine.

4. Manual run:
   `Run now` in the UI calls `Service.RunNow`. The Service checks that the job
   exists, is not already running, and (in sequential mode) that no other job is
   running, then executes `runner.RunJob` with the `Manual` trigger. Manual runs
   are allowed even while the scheduler is globally paused.

5. Command execution:
   `runner.RunJob` builds the platform-specific invocation, executes the
   command through the platform shell, captures stdout and stderr, writes one
   timestamped `.log` file, and returns a `domain.RunRecord` containing
   `DurationMS` (wall-clock milliseconds from start to finish; 0 for
   `StartOnly` fire-and-forget jobs).

6. History update:
   When a run goroutine completes, `Service` updates the job's runtime
   (including the statistics aggregate), saves JSON, triggers log cleanup, and
   emits `RunRecorded`. The UI observer appends the record to the History tab.

7. Autostart:
   `UpdateSettings` in the Service calls `autostart.Manager.Set`. The Manager
   interface has two implementations: Windows writes a `.lnk` shortcut to the
   user Startup folder; Linux writes an XDG Autostart `.desktop` file. Both
   entries pass `--start-in-tray`.

8. Error surfacing:
   Background errors (failed JSON saves, cleanup errors) are emitted as
   `ErrorOccurred` events and displayed in the UI status area, rather than
   being silently discarded.

## Key Domain Concepts

### Per-job overlap policy

`domain.Job` carries an `OverlapPolicy` field (`json:"overlap_policy,omitempty"`).
When non-empty it overrides the global `Config.OverlapPolicy` for that job alone.
Empty means inherit the global default. `app.Service.RunDue` resolves the
effective policy per job: it uses `job.OverlapPolicy` when set, otherwise falls
back to `store.Config.OverlapPolicy`. `normalizeJob` in `app/operations.go` leaves
the field empty on new jobs so the inherit semantics are preserved.

### Run-time statistics

`domain.JobRuntime` holds a rolling aggregate updated after each run:

| Field | Meaning |
|-------|---------|
| `RunCount` | total runs recorded |
| `FailCount` | runs that exited non-zero |
| `LastDurationMS` | wall-clock time of the most recent run |
| `AvgDurationMS` | mean over all runs with a recorded duration |
| `MaxDurationMS` | longest recorded run |

`runner.RunJob` measures the wall-clock start→finish and sets `DurationMS` on
the returned `RunRecord`. `runner/logfile.go` writes a `duration` line into the
log file header alongside the existing `state` line.

On startup, `runner.SeedStats` scans each job's log files (matched by the
`_<sanitized name>.log` suffix, bounded by `Config.MaxLogFiles`) and folds the
parsed `state`/`duration` headers into a `runner.StatSeed` map. `NewService`
applies those seeds to the runtime map before the first scheduler tick, so the
details panel shows accumulated run history immediately after a restart.
Older log files that pre-date the `duration` header are tolerated: the run is
counted but the timing is skipped.

### Persisted global pause

`domain.Config` carries a `Paused bool` field (`json:"paused,omitempty"`).
`app.Service.SetGlobalPause` writes the new value into `store.Config` and calls
`SaveConfig`, so the paused state survives a restart. `NewService` initialises
`s.paused` from `store.Config.Paused` and applies the paused next-run text to
all runtimes before the first tick, ensuring the UI shows the correct state from
the moment the window opens.

### `jobs_view.go` file structure

`src/ui/jobs_view.go` is split across three files to stay within the ~250-line
size guideline:

| File | Contents |
|------|----------|
| `jobs_view.go` | `newJobsView` — list, toolbar, button wiring, and layout |
| `jobs_view_details.go` | `detailsPanel` struct — widget creation, `update`, `clear`, `container` |
| `jobs_view_helpers.go` | Pure helpers — `filteredJobIndexes`, `folderOptions`, `filterValue`, `indexOfID`, `lastJobLogs` |
