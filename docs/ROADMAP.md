# Roadmap

This file tracks planned GoSentry work that is larger than a single bug fix.
Completed work is recorded in [CHANGELOG.md](CHANGELOG.md), not here.

## Open Items

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

### GUI review — custom layouts and composition

**The review has been carried out — its findings are in
[GUI-LAYOUT-REVIEW.md](GUI-LAYOUT-REVIEW.md).** This item stays open until they
are applied; F9, F13 and F15 there are larger than a single fix and come back
here once the rest has landed. The agenda below is what the pass was asked to
answer.

The `ui` package has accumulated hand-written layouts and tuned constants that
work but have never been reviewed as a whole:
[`layout.go`](../src/ui/layout.go) holds four custom `fyne.Layout`
implementations (`minWidthLayout`, `compactVBoxLayout`, `fixedHeightLayout`,
`captionValueLayout`), and the views drive them with negative spacings
(`detailRowSpacing = -8`, `jobRowSpacing = -8`, `settingsRowSpacing = -6`) that
cancel out the built-in padding of Fyne widgets.

Do a focused pass over composition only — not a general code review — and
answer, per layout and per constant: is it still needed, is it the smallest
thing that works, and does it hold up at different window sizes and theme
scales.

What to look for:

- *Negative spacing as a workaround.* Pulling rows together to overlap label
  padding is a workaround for widget metrics, not a layout decision. Check
  whether a `widget.Form`, a grid, or a custom text row would express the same
  result without depending on the padding a future Fyne release may change.
- *Hard-coded pixel constants.* Widths and heights expressed in raw pixels
  (`logColumnMinWidth`, `minJobsSidebarWidth`, `settingsControlWidth`) do not
  follow `theme.Padding()` / text size, so they behave differently under a
  scaled UI. `activityRowsHeight` already derives its height from the theme —
  decide which of the rest should do the same.
- *Layouts with one call site.* `fixedHeightLayout` is used once. If a stock
  container expresses the same intent, deleting the type is a net win — the
  complexity rule in [REVIEW.md](REVIEW.md) §2 applies to layouts too.
- *File size.* `settings_view.go` is well past the ~250-line guideline in
  [ARCHITECTURE.md](ARCHITECTURE.md) and should be split the way `jobs_view.go`
  was.
- *Behaviour at small sizes.* The details pane was condensed to fit 720p; verify
  the current composition still degrades sensibly when the window is narrow or
  short, instead of clipping.

Findings that are single fixes go straight in; anything larger comes back here.

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
