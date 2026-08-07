# Roadmap

This file tracks planned GoSentry work that is larger than a single bug fix.
Completed work is recorded in [CHANGELOG.md](CHANGELOG.md), not here.

## Open Items

### Faster Windows failure notifications

Fyne `SendNotification` on Windows does not call WinRT directly. Each toast
writes a short script to `%TEMP%` and runs it through a **new PowerShell
process** (`app/app_windows.go`), which typically adds **1–3 seconds** of cold
start before the toast appears. GoSentry's own path from run completion through
`SendNotification` is much smaller and is logged separately.

**Baseline (2026-08-05, `scripts/measure-windows-toast.ps1`, 3 runs on dev
machine):** average **773 ms** per toast (695–874 ms), dominated by PowerShell
cold start. Re-run the script when comparing after a native toast implementation.

**App-side timing:** each failure notification appends one line to
`logs/notify-timing.tsv` (`ms_after_run`, `ms_fyne_do`, `ms_send`,
`ms_app_total`). These columns end when Fyne returns from `SendNotification`; OS
toast latency is not included. The `.tsv` extension keeps it out of
`runner.CleanupLogs`, which only manages `.log` files — this file is
diagnostic instrumentation for this item, not job output, and should be
removed (or unified with the run-log retention policy under its own knob) once
the native-toast direction below lands and the timing data is no longer
needed.

**Direction:** add `src/platform/notify/` with a native Windows toast (WinRT or
a maintained Go wrapper), used for failure notifications on Windows. Keep Fyne
`SendNotification` on Linux (DBus / xdg-desktop-portal) unless profiling shows it
needs the same treatment.

### Retire the config compatibility shims

Two read-only shims in `storage.loadOrCreateConfig` rewrite an old file into
the current shape on the next save, so each becomes dead the moment a user's
config has been saved once by a build that has it:

- `Config.JobsDir` (pre-0.15, superseded by `Config.JobsFile`).
- `Theme == "default"` (pre-1.0.1, superseded by `ThemeSystem`).

Neither has an expiry. Remove both — the field, the migration branch, and
`TestLoadOrCreateConfigMigratesJobsDir` /
`TestLoadOrCreateConfigMigratesLegacyThemeDefault` — once a release has shipped
long enough that a config file still carrying either old shape is not a
realistic upgrade path GoSentry needs to support.

### Dynamic tray icon toggle

Fyne exposes `SetSystemTrayIcon` and related APIs only at application startup.
There is no supported way to register or remove the notification-area icon
after the process is running.

GoSentry now honours `KeepRunningInTray` from config: close behaviour and the
autostart entry update immediately when the user saves Settings; the tray icon
follows the saved value on the next launch. Settings shows a restart hint when
the tray checkbox changes.

Revisit when Fyne adds a documented API for mid-session tray registration, or
when a stable cross-platform approach exists without reaching into driver
internals. Until then, removing the restart hint and applying the icon on save
is blocked.

### Update check from GitHub releases

Releases are published as GitHub Releases (tags like `v0.12.0`, built by
`.github/workflows/release.yml`), but the app never tells the user a newer
version exists — they have to check the releases page by hand.

Add an update check that queries the GitHub Releases API
(`GET /repos/mixeme/gosentry/releases/latest`) for the latest published tag,
strips the leading `v`, and compares it against `app.Version`. When a newer
version is available, surface it non-intrusively — an "Update available"
line in Settings (next to the existing version/build info) with a hyperlink
to the release page, not a modal on launch.

Design notes / open questions:

- *Opt-in and offline-safe.* The check makes a network request, so it must be
  off by default (or clearly consented) and never block startup. Failures
  (offline, rate-limited, API change) should be silent — no error dialogs for a
  best-effort convenience feature.
- *Version comparison.* Compare semantic versions, not strings, so `0.12.0`
  reads as newer than `0.9.0`. A tiny semver comparator in `app` (or a small
  dependency) avoids lexical bugs.
- *Where the check lives.* Keep it in the `app` layer behind the Service so the
  UI only renders the result, and cache the last check so opening Settings
  repeatedly does not spam the API (unauthenticated GitHub allows 60 req/h).
- *Repo coordinates.* The primary remote is Gitea; the GitHub repo used for
  releases is [`mixeme/gosentry`](https://github.com/mixeme/gosentry) and must
  be wired in explicitly (constant or build-time value) rather than derived from
  `origin`.
- *No auto-download.* Scope is detection and notification only; installing the
  update stays a manual click-through to the release page.

### Import/export jobs as a cron table

Jobs can only be moved between machines by copying `jobs.json` by hand. Add
"Import" / "Export" actions (Settings tab, file dialogs) that read and write a
crontab-style text file, so a job list can be shared, version-controlled, or
seeded from an existing Unix crontab.

Export writes one line per job — schedule fields, then command and arguments —
and import parses the same format back into `domain.Job` values.

Design notes / open questions:

- *The job model is wider than a crontab line.* `Name`, `Folder`, `StartOnly`,
  `OverlapPolicy`, `TimeoutSeconds`, and `Enabled` have no cron equivalent.
  Either accept a lossy export (schedule + command only) or carry the extra
  fields in a structured comment above each line (`# gosentry: name=… folder=…
  timeout=…`), which keeps the file readable by real cron while making the
  round-trip lossless. The comment form is preferred; decide the exact key set
  before implementing.
- *Disabled jobs.* `Enabled: false` maps naturally to a commented-out line, but
  then a disabled job is indistinguishable from a user's own comment unless the
  `# gosentry:` marker is present. Pick one representation and document it.
- *`@every` is not crontab.* GoSentry accepts `@every 10s` (see
  [`domain.Parse`](../src/domain/schedule.go)), which no cron implementation
  understands. Exporting it produces a file that is not a valid crontab;
  exporting it as an approximation would silently change the schedule. Keep the
  raw string and flag the file as GoSentry-flavoured, rather than converting.
- *Command vs arguments.* Crontab has a single command string; GoSentry splits
  `Command` and `Arguments`. Import must split the line the same way the runner
  would (see `runner/invocation*.go`, which differs per OS), and export must
  join them back without changing quoting.
- *What to skip on import.* Environment assignments (`SHELL=`, `PATH=`,
  `MAILTO=`), six-field (seconds) crontabs, and `@reboot` are outside what
  `domain.Parse` accepts. Skip them, and report which lines were skipped and
  why — a partial import that silently drops rows is worse than a failed one.
- *Merge semantics.* Import must decide between replacing the job list and
  appending to it, and must assign fresh IDs rather than trusting the file.
  Appending with a confirmation dialog is the safer default; replacing needs an
  explicit "this deletes N jobs" confirmation.
- *Where it lives.* Encoding/decoding is pure text handling and belongs in
  `domain` (or a small `storage` codec) with unit tests over round-trips; the
  Service exposes import/export operations; the UI only picks the file and
  shows the outcome.

### Split the files that are over the size guideline

[ARCHITECTURE.md](ARCHITECTURE.md) sets a ~250-line guideline per source file
and records the `jobs_view.go` and `settings_view.go` splits as the worked
examples. `jobs_view.go` was split again in 1.0.2 — into view, state, list, and
toolbar — because the selection defect it carried was a symptom of the size
(one 330-line constructor over seven shared locals). Six non-test files are
over the guideline:

| File | Lines |
|------|-------|
| `src/app/operations.go` | 529 |
| `src/storage/store.go` | 382 |
| `src/ui/history_view.go` | 355 |
| `src/ui/settings_view.go` | 326 |
| `src/app/run.go` | 275 |
| `src/app/service.go` | 252 |

The remaining six are deliberately deferred rather than done piecemeal: a
split touches every reader of the file, and doing them in one pass keeps the
seams consistent instead of settling each one its own way. Splitting is
also the kind of change that reads as pure movement while quietly dropping a
function, so it wants one careful pass, not a hurried one per file.

Seams visible today, as a starting point rather than a decision:

- **`operations.go`** — the worst overage and the clearest split: the public
  mutating operations (`CreateJob` … `UpdateSettings`), the `…Locked` state
  helpers that only they call, and the pure validators and normalizers
  (`normalizeJob`, `validateJob`, `hasFileName`, `validateConfig`) are three
  distinct jobs already sitting in three consecutive blocks.
- **`history_view.go`** — the column-measuring helpers (`textWidth` through
  `historyColumnWidths`) are pure, already unit-tested, and independent of the
  table they size.
- **`store.go`** — path resolution, the config load/normalize path, and the jobs
  load/normalize path are three separate concerns in one file.
- **`run.go`**, **`settings_view.go`**, **`service.go`** — barely over. Worth
  re-measuring at the time; if a pass elsewhere has shrunk them, leave them
  alone rather than splitting for the sake of the number. The counts above move
  a few lines either way with any edit, so re-measure before acting on them
  rather than treating the table as current.

The `jobs_view.go` pass is the worked example for the rest: the constructor was
broken up along the state it shared, not along line count, and the split landed
with the selection fix rather than promising it separately.

Scope note: the guideline is about source files. Test files are much larger and
that is fine — a table-driven test file grows with the cases it covers.

### Window size persistence *(frozen)*

Window size is currently **not** saved on quit or close. Saving was disabled
because `w.Canvas().Size()` returns the maximized dimensions when the window is
maximized, which would corrupt the stored size on the next launch.

Re-enabling requires a cross-platform way to detect the maximized state before
saving. Fyne v2.x has no API for this; it needs per-OS native calls:
`IsZoomed` (Windows), `_NET_WM_STATE` (X11/Linux), `NSWindow.isZoomed`
(macOS). Unfreeze once that detection is in place.

**Disadvantages of a platform-specific approach:**

- *Three separate implementations.* Windows, macOS, and Linux each need their
  own file guarded by a build tag. Each adds CGO bindings or raw syscall
  wrappers that must be kept in sync as OS APIs evolve.
- *Linux is not one target.* X11 and Wayland have completely different window
  state models. `_NET_WM_STATE` is X11-only; under Wayland the compositor
  controls window decorations and there is no stable client-side API to query
  the maximized state. A single `linux` build tag cannot cover both correctly.
- *Native window handle is not exposed.* Fyne does not surface the underlying
  `HWND` / `NSWindow` / `XID` through its public API. Obtaining it requires
  either enumerating OS-level windows by PID (fragile, finds wrong windows when
  dialogs are open) or reaching into Fyne/GLFW internals (breaks on Fyne
  upgrades).
- *Thread-safety constraints.* Win32 and GLFW both require their calls to be
  made from the OS main thread. Tray-menu callbacks run on a separate goroutine,
  so any native call must be marshalled back to the main thread, adding
  synchronisation complexity.
- *Test coverage gap.* Maximized-state detection cannot be exercised by Fyne's
  headless test driver; it requires a real display and manual or screen-capture
  automation per platform.

### History tab — column filters (Trigger / Job / State)

Add dropdown filters above the History table so the user can narrow rows by
trigger source, job name, or run state. Blocked on Fyne native support: the
current `widget.Table` has no built-in filter API, and a filter bar built from
`widget.Select` widgets above the table feels visually out-of-place. Revisit
when Fyne adds first-class column filtering or a composable data-grid widget.
