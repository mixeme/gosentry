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

## Intentional behavior (not bugs)

- `RunNow` is allowed during global pause and for disabled jobs.
- Sequential mode runs jobs FIFO by order in `jobs.json`.
- Scheduler tick is 1s — sub-second `@every` intervals are not supported.
- Command timeout is 30s globally.
- **History tab is session-only.** `JobRuntime.Logs` exists only in memory for the
  current process. Log files on disk feed aggregate statistics via `SeedStats`
  only. See [ARCHITECTURE.md](ARCHITECTURE.md).

## Out of scope

Larger or blocked work is tracked in [ROADMAP.md](ROADMAP.md) (per-job timeout,
window size persistence, History column filters, CI coverage gate).
