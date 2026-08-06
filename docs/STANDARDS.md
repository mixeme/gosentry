# GoSentry — Standards

Quality rules and intentional behavior for contributors. Package contracts live
in [ARCHITECTURE.md](ARCHITECTURE.md); test conventions in [TESTS.md](TESTS.md).

## Code quality

- Follow package contracts in [ARCHITECTURE.md](ARCHITECTURE.md).
- User-facing errors → `dialog.ShowError` or a History event, never a silent `return`.
- Pure helpers → unit test in the same package.
- Fixes with severity ≥ medium → regression test.
- Documented intentional behavior → section below, not a backlog bug.
- UI view constructors accept `*app.Service`; call `app.Open()` only from `run.go`.
- **No blocking file I/O under `Service.mu`.** It is the lock the Fyne main
  thread takes on every `Jobs()` and `Runtime()` call, so a JSON write, a
  log-directory scan, or a pass over every log header inside it makes a UI
  refresh wait on the disk. Mutate state under the lock, snapshot what the I/O
  needs, and run the I/O after `mu.Unlock()` — the way `emit()` already is.
  Store writes go through `Service.deferSaveLocked` and `Store.PrepareSaveJobs` /
  `Store.PrepareSaveConfig`, which take `saveMu` while `mu` is still held so
  writes still reach the file in the order their snapshots were taken; log
  cleanup and `runner.SeedStats` run from plain snapshots.
- A size that must follow the theme is **measured at build time, not written as
  a pixel constant.** `theme.Padding()` and text metrics depend on the running
  app's theme, text size, and DPI, so a hand-tuned number is only correct for
  the one theme it was tuned against and clips under any other. Measure the real
  widget, or derive the value from the theme, in a named helper: `rowOverlap`
  (theme padding), `captionColumnWidth` and `textColumnWidth` (the widest of the
  actual strings), `activityRowsHeight` (the list's own row template). The same
  applies to a ratio computed from an absolute width — see `initialSplitOffset`.
  A raw pixel literal is left only where nothing about it tracks the theme, and
  says so in a comment.

## Config file compatibility

There is no migration step: `gosentry.json` and `jobs.json` are read as-is, are
meant to be hand-editable, and may have been written by an older version. A
change to their shape has to stay compatible on its own.

- A new `Config` field is tagged `omitempty`, and its zero value must mean the
  behavior that existed before the field was added — a file written without it
  keeps working unchanged. `DefaultConfig()` still sets the value explicitly.
- A zero that carries meaning is not a missing field and must not be backfilled
  on load. See `DefaultTimeoutSeconds` in `storage.loadOrCreateConfig` and
  `Job.TimeoutSeconds *int`, where unset and `0` are different answers.
- An unrecognised enum value reads as the default rather than an error, through
  one helper that every consumer shares (`JobListView.IsCompact`, `ui.themeFor`),
  and is normalized before being written back, so the file never gains a value
  no reader understands.
- A renamed key keeps the old field on `Config` (tagged `omitempty`) purely so
  it can still be read. `storage.loadOrCreateConfig` converts it to the new
  field and clears it, so the retired key disappears on the next save. See
  `Config.JobsDir` → `Config.JobsFile`. Where the new field has a non-empty
  default, clear that default before unmarshalling, or "the file omits it" and
  "the file sets it" become indistinguishable and the conversion never runs.
- Each of the three gets a test: the default in `storage`, the normalization in
  `domain`, and a round-trip through the real config file in `app`.

## Intentional behavior (not bugs)

- `RunNow` is allowed during global pause and for disabled jobs.
- Selecting a jobs file that already exists **loads** it: its jobs replace the
  in-memory list, which is the only way the user can switch between job lists. A
  path with no file behind it receives the current jobs (rename/relocate). The
  switch is refused while a job is running, because adoption drops every runtime
  and a finishing run would then write its result onto whichever job inherited
  its ID.
- Sequential mode runs jobs FIFO by order in `jobs.json`.
- Scheduler tick is 1s — sub-second `@every` intervals are not supported.
- Command timeout defaults to no timeout globally (`Config.DefaultTimeoutSeconds`
  = 0) and is overridable per job (`Job.TimeoutSeconds *int`: unset = inherit the
  global default, 0 = no timeout, positive = seconds). Neither zero may be
  normalized away on load — 0 is a value, not a missing field.
- **`Config.MaxLogFiles` and `Config.MaxLogAgeDays` of 0 mean "keep everything",
  not "unset".** `runner.CleanupLogs` already treated `<= 0` as "policy
  disabled"; `app.validateConfig` and the Settings form now accept 0 (only a
  negative count is rejected), and `storage.loadOrCreateConfig` no longer
  backfills 0 to 100 / 30 — a config written before either field existed still
  picks up the default because `json.Unmarshal` leaves an absent key holding
  whatever `DefaultConfig()` set, the same mechanism `DefaultTimeoutSeconds`
  relies on.
- **A `StartOnly` process is expected to outlive GoSentry.** The option exists to
  launch something and let go of it, so the runner builds that invocation on
  `context.Background()`, not on the application's lifecycle context: quitting
  GoSentry (or cancelling a run) does not stop a process it started this way, and
  `Service.Stop()` reaches only jobs the runner is still waiting on. The
  uncancelable context is also what keeps `os/exec` from leaving a watcher
  goroutine per run — it only starts one when the context can be done, and
  `StartOnly` never calls `Wait` to end it.
- **History tab is session-only.** `JobRuntime.Logs` exists only in memory for the
  current process. Log files on disk feed aggregate statistics via `SeedStats`
  only. See [ARCHITECTURE.md](ARCHITECTURE.md).
- **History is capped and its columns only widen.** The tab keeps the newest
  `maxHistoryRows` records and drops the oldest, the way `maxJobLogs` caps a
  job's own activity list — an app left in the tray records thousands of runs a
  day, each carrying the run's full captured output. Column widths are folded in
  one record at a time instead of rescanned from every row, so a column never
  narrows when a record ages out: the rows on screen were laid out against the
  wider value. A theme change is the one case that rescans, because every stored
  width was measured at the old text size.
- Several tests share a coverage profile with another test on purpose, and a few
  functions sit at 0% on purpose. Both lists live in
  [TESTS.md](TESTS.md) — check them before reporting a test as redundant or a
  coverage gap as an oversight.
- **`KeepRunningInTray` controls tray and close behavior.** When enabled (the
  default), the app registers a system tray icon at launch, closing the window
  hides it, and autostart passes `--start-in-tray`. When disabled, no tray icon
  is registered at launch, closing the window quits the app, and autostart opens
  the main window. Toggling the setting in Settings updates close behavior and
  rewrites the autostart entry immediately; the tray icon itself follows the
  saved value only after a restart because Fyne has no API to add or remove it
  mid-session (see [ROADMAP.md](ROADMAP.md)).
- **`--start-in-tray` defers to config.** A stale autostart shortcut that still
  passes the flag does not hide the window when `KeepRunningInTray` is off.
- **`JobRuntime.PendingRuns` (the "queue" overlap policy's backlog) is capped at
  `maxPendingRuns` (10) and cleared on pause or disable.** A job whose runs take
  longer than its interval stops accumulating backlog once the cap is hit —
  further overlaps are dropped like the "skip" policy until the backlog drains
  below the cap. `SetGlobalPause(true)` and `SetEnabled(id, false)` both zero
  the counter, so resuming or re-enabling a job never replays a deferred run for
  an occurrence that fired before the pause/disable. The details pane appends
  ", N queued" to the statistics line via `DisplayStats` whenever the count is
  non-zero.

## Out of scope

Larger or blocked work is tracked in [ROADMAP.md](ROADMAP.md) (update check from
GitHub releases, cron-table import/export, window size persistence, History
column filters).
