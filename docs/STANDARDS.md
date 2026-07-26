# GoSentry — Standards

Quality rules and intentional behavior for contributors. Package contracts live
in [ARCHITECTURE.md](ARCHITECTURE.md); test conventions in [TESTS.md](TESTS.md);
what a whole-project review looks at, in [REVIEW.md](REVIEW.md).

## Code quality

- Follow package contracts in [ARCHITECTURE.md](ARCHITECTURE.md).
- User-facing errors → `dialog.ShowError` or a History event, never a silent `return`.
- Pure helpers → unit test in the same package.
- Fixes with severity ≥ medium → regression test.
- Documented intentional behavior → section below, not a backlog bug.
- UI view constructors accept `*app.Service`; call `app.Open()` only from `run.go`.

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
- **History tab is session-only.** `JobRuntime.Logs` exists only in memory for the
  current process. Log files on disk feed aggregate statistics via `SeedStats`
  only. See [ARCHITECTURE.md](ARCHITECTURE.md).

## Out of scope

Larger or blocked work is tracked in [ROADMAP.md](ROADMAP.md) (window size
persistence, History column filters, CI coverage gate).
