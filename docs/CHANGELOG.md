# Changelog

All notable GoSentry changes are recorded in this file.

## 1.0.2 - 2026-08-05

**KeepRunningInTray is wired to runtime; Windows failure notifications can show
the app icon (experimental).**

**Application:**

- **Keep running in the system tray** now controls behaviour: with the tray on
  (default), closing the window hides it and autostart uses `--start-in-tray`;
  with the tray off, closing quits the app and autostart opens the main window.
- Saving a tray change updates close behaviour and the autostart entry
  immediately. The notification-area icon follows the saved value after a
  restart; Settings shows a hint when a restart is needed (Fyne cannot add or
  remove the icon mid-session).
- A stale autostart shortcut that still passes `--start-in-tray` no longer hides
  the window when the tray setting is off — saved config wins over the CLI flag.
- On Windows, failure toasts can show the app icon: after `NewWindow`,
  `AppMetadata.Icon` is registered so Fyne picks up artwork without calling
  `SetIcon`, which would override the PE multi-size window/taskbar icon.
- Fixed a Windows quoting bug where a job whose **Command** field held a whole
  command line (a `.bat`/`.cmd` wrapper followed by an argument that itself
  ended in `.exe`) had its entire command line mistaken for the program path,
  so the run failed with an unmappable shell error. The program path is now
  found by the earliest file-extension match at a word boundary, not the first
  extension in list order.
- `gosentry.json` and `jobs.json` (and run log files) are now written
  atomically — to a temp file, then renamed into place — so a crash or power
  loss mid-write can no longer leave a truncated or empty file. `Service.Stop()`
  is now called when the app quits, which also makes the run context
  cancellation reach in-flight runs on shutdown.
- Fixed the "queue" overlap policy's backlog (`PendingRuns`): it no longer
  survives a global pause or a job being disabled, so resuming or re-enabling a
  job can no longer replay a deferred run left over from before the pause/
  disable. It is also capped at 10 queued occurrences, so a job whose runs take
  longer than its own interval no longer accumulates an unbounded backlog that
  then runs back-to-back indefinitely. The job details pane now shows the
  queued-run count (", N queued") whenever it is non-zero.
- **Start-only jobs are no longer tied to the application's lifetime.** A job
  with *Start only* checked is launched on an uncancelable context, so quitting
  GoSentry (or a run context being cancelled) can no longer try to kill a
  process it deliberately stopped waiting for. This also removes a goroutine
  that leaked on every start-only run and lived until the app exited.
- The History tab no longer grows without bound: it keeps the newest 1000
  records and drops the oldest, the way a job's own activity list is capped.
  Column widths are also folded in one record at a time instead of being
  re-measured across every row on every event, so recording a run no longer
  gets slower the longer the app has been running. Measured on 5000 accumulated
  records, one History redraw went from **15.8 ms to 0.9 ms**; at the new cap
  the width rescan alone accounted for 1.5 ms of every redraw.
- **The Jobs tab keeps its selection on the job, not on the row.** Selecting a
  different jobs file in Settings replaces the whole job list; the details pane
  then described whichever job happened to land on the previously selected row —
  or went blank if the new list was shorter — while the highlight in the list
  stayed where it was. The selection now follows the job itself, and the
  highlight and the details pane always describe the same one.
- **Max log files and max log age days now accept 0, meaning "keep
  everything."** Log cleanup already supported disabling either policy; the
  Settings form and the Service validator rejected the value that would have
  turned it on. A config that already set either to 0 is no longer silently
  rewritten back to the 100/30 defaults on load.

**Jobs:**

- New disabled example *Failure notification test* (folder Examples). Run it
  manually to trigger a failed run and verify Settings → Notifications without
  waiting on the scheduler.

**Documentation:**

- **`docs/ARCHITECTURE.md`** — new §Platform layer: why autostart, file manager,
  shell, and winproc are OS-specific; compile-time vs runtime branching; rules
  for adding platform code.

**Internal:**

- App-side failure-notification timing is appended to `logs/notify-timing.tsv`
  for diagnosing toast delay (OS latency excluded). The `.tsv` extension keeps
  the diagnostic file out of `CleanupLogs`, which manages only `.log` files, so
  it is neither deleted by age nor counted against **Max log files**. The append
  runs off the UI thread. `scripts/measure-windows-toast.ps1` measures the
  PowerShell baseline on Windows.
- `jobs.json` is no longer rewritten twice per run. Starting and finishing a run
  touch only `JobRuntime`, which is never persisted, so both saves re-serialised
  identical bytes; `SetGlobalPause` did the same alongside its real `SaveConfig`.
  Removing them also removes the run-start rollback path and the save failure it
  reported, so `RunDue` no longer has a start error to surface at all.
- File I/O no longer happens while `Service.mu` is held — that is the lock the
  UI thread takes on every job and runtime read, so a JSON write, the
  post-run log cleanup, or the startup log scan used to make a UI refresh wait
  on the disk. Saves are now prepared under the lock and written after it is
  released, in preparation order, so `jobs.json` still ends up matching the
  in-memory list. Seeding statistics from logs also opens each log file once
  instead of twice.
- The Jobs tab was split into `jobs_view.go` (construction, refresh, layout),
  `jobs_view_state.go` (the job/runtime snapshot, folder filter, and selection),
  `jobs_view_list.go`, and `jobs_view_toolbar.go`. What used to be one 330-line
  constructor whose dozen closures shared seven mutable locals is now widgets
  reading one named state object — which is what made the selection fix above a
  change in one place instead of five.

## 1.0.1 - 2026-08-04

**The branded theme is the default, Fyne's built-in theme is System, and the
test suite is leaner.**

**Settings:**

- **The branded GoSentry theme is now the default.** Fresh installs, the
  **Defaults** button, and configs that omit `theme` all open in the teal/amber
  look; users who prefer Fyne's built-in theme can still pick **System** in
  Settings.
- The Fyne built-in theme option is labelled **System** (stored as `"system"`);
  configs that still say `"default"` are read as System and rewritten on save.
- The **About** repository link points at GitHub (`mixeme/gosentry`) instead of
  the private Gitea mirror.

**Jobs:**

- The **Disable auto** row gained a top inset matching the gap below it, so it
  no longer sits flush against the tab bar.

**Documentation:**

- The **README Schedules** section now documents `@every` in full: supported Go
  duration units (`ns` through `h`), combined values such as `1h30m`, the link to
  `time.ParseDuration`, the fact that days/months/years belong in cron rather
  than `@every`, the one-second scheduler tick floor, cron examples for monthly
  and yearly runs, and the `@hourly`/`@daily`/… descriptors.

**Tests:**

- Three tests with byte-identical coverage to an existing test and no unique
  assertion are gone: `TestCleanupLogsKeepsFilesWithinAgeLimit`,
  `TestRunDueEmptyOverlapInheritsGlobal` (its one unique setup guard moved into
  `TestRunDueQueueRerunsAfterFinish`), and `TestSameWindowsPathHandlesSpaces`
  (its spaces case folded into `TestSameWindowsPathIgnoresCaseAndQuotes`'s
  fixture). So are two that carried no assertion at all:
  `TestEmitWithNoObserversIsNoop` and `TestStoreReturnsWiredStore`.
- The four `TestFilteredJobIndexes*` tests are one table-driven
  `TestFilteredJobIndexes`, matching `TestFilterValue` above it.
- `TestMainViewBuilds` is replaced by `TestMainViewRecordStartupAddsHistoryRow`,
  which exercises the `recordStartup` closure for both wordings `run.go` selects
  between and asserts the rows reach the History table through its cell callbacks,
  including the `!windowShown` branch.
- `storage.defaultJobs` — the one accidental 0%-coverage gap the review
  found — is now exercised by
  `TestLoadOrCreateJobsSeedsSampleJobsOnFirstRun`, which also corrects
  `docs/TESTS.md`: `TestLoadOrCreateConfigCreatesDefaultsOnFirstRun` never
  touched jobs, so the "and a sample job" half of its old description was
  wrong.
- `src/runner/seed_test.go`'s hand-rolled `itoa` — 18 lines of digit-by-digit
  conversion in a file that already imports `strconv` — is replaced with
  `strconv.FormatInt`.
- **`docs/TESTS.md`** records the `-coverpkg` command and the 84.4% baseline
  (per-package figures understate the suite), design principle 9 (redundancy is
  judged by comparing coverage profiles, and identical coverage alone is not
  grounds for deletion), a table of the look-alike tests that are kept with the
  reason each survives, and the list of functions deliberately at 0%.
- **`docs/STANDARDS.md`**'s "Intentional behavior" section points at both lists,
  so the mechanism `docs/REVIEW.md` describes still reaches them. The spent test
  review plan is retired.

## 1.0.0 - 2026-07-27

**The window opens at the size it asks for, and the Jobs divider can be
dragged.**

**Window:**

- **The window opens at 1024×660 and can now be dragged narrower than it opens.**
  Fyne treats the assembled content's minimum size as a hard floor over the
  requested size, and two widgets in Settings pushed that minimum past 1024 px:
  a fixed width applied to seven controls that the layout already stretched, and
  the read-only config path, which grew the whole tab with the length of the
  path it was showing — a 75-character path alone demanded 1501 px. The path now
  clips when the window is genuinely narrow instead of widening the window, and
  the content minimum is 972 px.

**Jobs:**

- **The divider between the job list and the details pane is draggable.**
  Previously the list was pinned at its natural width and the details pane took
  whatever was left, so a long command or a deep folder path could not be given
  more room. Either pane can now be widened at the other's expense, and neither
  can be dragged below its own content, so the details pane condenses rather
  than clipping. The divider opens at the list's natural width; its position is
  not saved, so a restart reopens at that default.

**History:**

- **Columns measure their own content.** Time, Trigger and State were fixed
  pixel widths with as little as 1.6 px of headroom and truncated their own
  values on a scaled UI or at a larger text size; all five now size themselves
  from the text they have to show, under the current theme. Job and Detail stay
  bounded so one long row cannot take over the table.

**Settings:**

- The **Save / Cancel / Restore defaults** row sits 4 px from the left edge, as
  its layout always intended, rather than 8.
- The caption column is as wide as the widest caption instead of a fixed width,
  which gives each value column about 22 px more and keeps the captions readable
  at a larger text size.
- The **Application** and **About** blocks are about 2 px tighter: every stacked
  row group in the app now shares one spacing derived from the theme rather than
  three separately tuned numbers.
- The **Theme** dropdown is no longer flush against the **Notifications**
  checkbox. That shared row spacing pulls rows together by one text inset, which
  the rows above have to give but a dropdown — which paints its box out to the
  row's edge — does not, so the gap collapsed to about a pixel. The Theme row
  now keeps the same gap the checkbox rows have.

**Documentation:**

- The **README** describes the application that exists. Its `gosentry.json`
  sample was three keys short of what the app writes on first run, which made
  the one file the user is invited to hand-edit the least accurate thing in the
  document; it is now the real default, with each key explained — including why
  a zero timeout is written out and an unset one is not. The feature list has
  caught up with the run timeout, the theme, the compact job list, and the
  per-job overlap and timeout overrides the job dialog has always offered.
- **`docs/DEVELOPMENT.md`** is ordered as stack, external libraries, run from
  source, build, release, CI, behind a two-level table of contents, instead of
  opening with MSYS2 setup and burying "Run From Source" mid-document. The
  library table gains versions and licenses, the `package-*` scripts are
  documented for the first time and labelled by OS, and the Codeberg
  `RELEASE_TOKEN` note now states the failure mode rather than leaving it to be
  inferred from a red job: build and packaging succeed, the upload step fails on
  authentication and takes the job with it, leaving a published release with no
  assets. The Project Layout section is gone — it duplicated ARCHITECTURE's
  package map and had drifted out of date.
- **Cutting a GitHub release now documents the push mirror it has to survive.**
  GitHub is a pruning push mirror of Gitea, so `gh release create` creating the
  tag itself produces a tag Gitea does not know about, which the next
  synchronisation deletes — orphaning the release and taking its uploaded
  archives with it, without a single failed step to point at. The procedure is
  push the tag to Gitea, wait for the mirror, verify the tag on GitHub, then
  publish with `--verify-tag`.
- **`docs/TESTS.md`** matches the suite it indexes again. It listed 130 tests
  against 170 in the tree, omitted four test files entirely, and named two tests
  that no longer exist. Every test function now appears exactly once, under the
  file it actually lives in.
- **`docs/ARCHITECTURE.md`** no longer draws the UI calling the autostart
  manager directly — it does not, and `src/ui` holds no reference to that
  package — and `platform/desktop` is described by what it does (the XDG desktop
  entry and icon) rather than as a display-scale helper.
- The **~250-line file guideline** is stated as the target it is, with the six
  files currently over it recorded as a `docs/ROADMAP.md` item. They are to be
  split in one pass during the next whole-project review, since six separate
  passes would settle the same seam question six ways.

## 0.15.0 - 2026-07-26

**Settings points at the jobs file itself, not the folder holding it.**

**Settings:**

- The **Jobs directory** row is now a **Jobs file** row. Browse opens a file
  picker filtered to `.json` instead of a folder picker, so the job list can
  live under any file name — `team-jobs.json`, one file per machine, a file
  shared over a network drive — rather than a fixed `jobs.json` per folder. The
  field stays editable, which is how a file that does not exist yet is named.
- **Selecting an existing jobs file now loads it.** Previously the current job
  list was written over whatever was at the new path, which made it impossible
  to switch to an existing jobs file — its contents were destroyed on Save. Now
  an existing file wins: its jobs are loaded, normalized, and replace the loaded
  list, with runtimes, parsed schedules, next-run times, and log-seeded
  statistics rebuilt around them. A path with no file behind it still receives
  the current jobs (and its folder is created), which is how the jobs file is
  renamed or relocated. History records `Jobs loaded — N jobs from <path>`,
  since the switch happens without a prompt.
- Switching to a different jobs file is refused while a job is running: adoption
  discards every runtime, and a run finishing afterwards would write its result
  onto whichever job inherited its ID. Settings unrelated to the jobs file still
  save normally during a run.
- Saving a path with no file name (a trailing separator, `.`, `..`) is rejected
  with "jobs file must include a file name" instead of failing later with an
  opaque OS error.

**Configuration:**

- `Config.JobsDir` / `jobs_dir` is replaced by `Config.JobsFile` / `jobs_file`,
  which holds the full path including the file name; the default is
  `"jobs.json"`, resolved against the program folder as before. `Paths.JobsDir`
  is now derived from the configured file so job saves still create the folder.
- A `gosentry.json` written by an earlier version is migrated on load: its
  `jobs_dir` is joined with `jobs.json`, which is the exact file that version
  used, and the retired key is dropped when the config is rewritten.
- New `app.JobsLoaded{Path, Count}` event, emitted when a selected jobs file
  replaces the job list; the UI turns it into the History entry. New
  `storage.LoadJobsFile`, which reads and normalizes a jobs file and reports a
  missing one as "not found" instead of seeding it the way startup does.

## 0.14.0 - 2026-07-26

**Compact job list view, "no timeout" at both timeout levels, and an Open
button for the logs folder.**

**Compact job list view.**

- The Jobs sidebar can now render each job as a single line — name on the left,
  status on the right — instead of the three-line block. A toggle button beside
  the Folder filter switches between **Compact** and **Detailed**; it is
  labelled with the action it performs, like the "Disable auto" button. Compact
  fits many more jobs on screen without scrolling; selection, the details panel,
  the folder filter, and live status updates all work unchanged in both modes.
- The choice is persisted as a new `Config.JobListView` field
  (`"detailed"` / `"compact"`, written to `gosentry.json` as `job_list_view`),
  so it survives a restart. Empty/legacy configs and any unrecognised value
  normalize to detailed, so existing installs keep the current look.

**Jobs sidebar:**

- The **Folder** caption moved onto the filter row itself, beside the select and
  the view toggle, instead of occupying its own line above it — the job list now
  starts a full label higher.

**Settings:**

- The **Logs directory** row gained an **Open** button that shows the folder in
  the desktop file manager (Explorer on Windows, the XDG handler on Linux), so
  reading a log file no longer means copying the path by hand. It opens the
  path currently in the field — including an edit that has not been saved yet —
  resolving a relative directory against the application folder exactly as the
  store does. A folder that is missing (the logs directory is created on the
  first run) or cannot be opened is reported in a dialog.
- The Save/Cancel/Defaults row sat flush against the separator above it and the
  tab's left edge; it now uses the same padding as the other vertical gaps in
  the tab.

**Job dialog:**

- The **Arguments** placeholder now states the field's rule — one argument per
  line, no quoting — instead of showing a lone example path that left the
  line-per-argument convention to guesswork.

**Timeouts: 0 now means "no timeout" at both levels.**

- The global **Default timeout** in Settings now defaults to `0`, meaning jobs
  run to completion with no deadline instead of being killed after 30s.
- A per-job timeout of `0` now also means "no timeout" and no longer inherits
  the global default. Leaving the job's timeout **empty** is what inherits.
  `Job.TimeoutSeconds` became `*int` so the three states — unset, explicit 0,
  and a positive limit — stay distinguishable in `jobs.json`.
- Fixed: a global default of `0` did not survive a restart. `gosentry.json` was
  loaded with `0` treated as a missing value and silently reset to 30s, so the
  setting only held for the current session. `default_timeout_seconds` is now
  written unconditionally and read back as-is.

Existing jobs and configs are unaffected: a job with no `timeout_seconds` still
inherits, and a saved global default of 30 stays 30.

**Internal:**

- Job names in the list are truncated through the widget's `Truncation` field;
  `fyne.TextTruncate` is deprecated in Fyne 2.7.4. Behavior is unchanged.
- Docker release builds mount `.gocache/` from the host, so `--rm` container
  removal no longer wipes `GOCACHE` between runs.
- Added `docs/REVIEW.md` (the project-review agenda) and a "Config file
  compatibility" section in `docs/STANDARDS.md` recording the rule the `Theme`,
  `JobListView`, and `TimeoutSeconds` fields already follow. Added `CLAUDE.md`.

## 0.13.0 - 2026-07-26

**Branded GoSentry color theme; Cancel/Defaults buttons in Settings.**

**Theme:**
- Added a custom Fyne theme derived from the logo and app icon (deep teal
  primary, amber accent, branded job-status colors) with light and dark
  variants; users can switch between it and Fyne's default theme from
  Settings. The choice is persisted as a new `Config.Theme` field
  (`"default"` / `"gosentry"`), applied at startup before the first frame
  and live-previewed when picked in Settings. Empty/legacy configs
  normalize to the default theme so existing installs keep the original
  look.
- The light variant is boldly branded: a soft teal window canvas with
  white inputs, menus, dialogs, and buttons on top, plus teal-tinted
  separators, input borders, and table headers, so cards and fields lift
  off the background instead of reading as a plain accent swap on gray.
  The dark variant uses deep-teal surfaces to echo the app icon. Text
  stays dark/light per the base foreground for contrast in both variants.

**Settings tab:**
- Added Cancel and Defaults buttons. Cancel discards unsaved edits by
  reloading the saved config; Defaults loads built-in default values into
  the form for review before saving. `domain.DefaultConfig()` is now the
  single source of truth for default values, shared by storage and the
  Settings UI.

## 0.12.0 - 2026-07-25

**Per-job command timeout:**
- Each job may now set its own run timeout (seconds) in the job dialog; leaving
  it empty inherits a new **Default timeout** in Settings (default 30s), the same
  inherit pattern as the overlap policy. The details panel shows the effective
  value, marking inherited jobs as `(global default)`.
- The formerly hard-coded 30s guard in `runner.RunJob` is now the configurable
  default. `StartOnly` fire-and-forget jobs remain unaffected by the run timeout,
  continuing to measure launch latency only.

## 0.11.5 - 2026-07-01

**Quality and documentation polish:**

- Replaced the interim `docs/FUTURE_WORK.md` with `docs/STANDARDS.md` — a slim,
  permanent reference for code-quality rules and intentional behavior.
- `newMainView` now accepts an injected `*app.Service` for testability;
  `RunNow` errors are shown in a dialog instead of failing silently.
- Empty job lists no longer panic when building the Jobs tab.
- Added regression and helper tests for overlap/pause scheduling, UI history
  helpers, main-view smoke build, and Linux desktop integration.

## 0.11.4 - 2026-06-30

**Statistics:**
- `StartOnly` jobs now record launch latency (time to spawn the process) as the
  run duration instead of a hard-coded `0`, so the Statistics line shows a real
  last/avg/max for fire-and-forget jobs. Sub-millisecond launches still round to
  0 and are excluded from the average, as before.

## 0.11.3 - 2026-06-29

**Reliability fixes from an internal code review: safer runs, a real overlap
queue, and more accurate statistics.**

**Scheduler / runs:**
- Fixed a data race where background runs could read log paths while settings
  were being saved.
- A run no longer starts if persisting the "Running" state fails; the job rolls
  back to its previous status instead.
- Under the `"queue"` overlap policy, every missed occurrence while a run is
  still in flight is now remembered and executed afterward (not just the last one).
- Manual and scheduled runs now advance next-due timing from the scheduler clock
  consistently.

**Application service:**
- Create, update, delete, enable/disable, and global pause no longer announce
  UI changes when the underlying JSON save fails.
- Invalid per-job `overlap_policy` values are rejected at save time.
- Log file write failures are reported in History instead of failing silently.

**Statistics:**
- Startup stat seeding matches log files by `job_id`, avoiding collisions when
  different job names sanitize to the same filename.
- Average run duration excludes zero-duration runs (such as StartOnly launches),
  matching how stats are rebuilt from log files.

**Documentation:**
- Added `docs/CODE_REVIEW.md` with the full review summary.
- Corrected stale YAML references and clarified that global pause stops only
  scheduled runs while manual "Run now" remains available.

## 0.11.2 - 2026-06-25

**Window state persistence, History sort fix, clearer scheduler toggle, and an
appID update.**

**Application:**
- The window size (width and height) is now persisted in preferences and
  restored on next launch. When the user closes the window (via Quit menu or
  window close button), the current dimensions are saved and will be applied
  when the application starts again. Defaults to 1024×660 if no saved size exists.
- Updated appID from `ru.mixdep.gosentry.desktop` to `ru.mixeme.gosentry.desktop`
  for consistency with the new domain name.

**History tab:**
- Fixed the Time column sort toggle, which stopped working after Fyne 2.7.4 began
  rejecting header-cell selections. The plain header label is replaced with a
  custom tappable header widget that handles the click directly.
- The sort direction is now shown with ▲/▼ glyphs instead of the "asc"/"desc"
  text.

**Jobs list:**
- Renamed the global scheduler toggle from "Pause all"/"Resume all" to
  "Disable auto"/"Enable auto", and swapped the stop icon for a pause icon, to
  make clear that it only stops automatic scheduled runs.

## 0.11.1 - 2026-06-25

**Settings tab refinements: even spacing, full labels, and a smarter Save button.**

**Settings tab:**
- The Queue selects (Execution mode, Default overlap policy) now use the same
  default spacing as the Storage fields, so they no longer sit squeezed together.
- Widened the caption column so the longest label, "Default overlap policy", is
  shown in full instead of being truncated.
- The Save button now starts disabled and only enables once a field differs from
  the saved config, re-disabling after a successful save (or if a changed field
  is reverted to its original value).

## 0.11.0 - 2026-06-25

**Manual runs while paused, two-column Settings/details, and a more compact job list.**

**Scheduler:**
- "Run now" is now allowed while the scheduler is globally paused. The global
  pause stops only automatic scheduled runs; an explicit manual run is the user's
  own one-off action and is no longer blocked (the already-running and
  sequential-mode guards still apply).

**Jobs details panel:**
- Metadata captions (Folder, Command, Run mode, …) are pinned to a fixed width
  instead of an even split, so widening the window now grows the value column
  rather than the short caption.
- Fixed a bug where the "Selected job activity" panel kept showing the previous
  job's entries when a different job was selected; the list now refreshes on
  every selection change.

**Jobs list:**
- List rows (name, schedule/command, status) are condensed with a tight,
  negative-gap layout so more jobs are visible without scrolling.

**Settings tab:**
- The form is reorganized into two columns — Application and Queue on the left,
  Storage and About on the right — with the Save button spanning the full width
  below. The Autostart status moved onto its own line so the section fits a
  half-width column.
- Removed the blank row that sat between the Save button and the following
  separator.

## 0.10.2 - 2026-06-25

**Condensed details/settings panels and a window that shrinks to 720p.**

**Jobs details panel:**
- Job metadata is laid out in two columns (Folder/Schedule, Command/Arguments,
  Run mode/Overlap policy, Last run/Next run, State/Statistics), roughly halving
  the block height.
- Metadata rows are stacked with a tight, negative-gap layout so the interval
  between rows is no longer oversized.
- The command-output area's minimum height was reduced so the details pane can
  get shorter; long output still scrolls.
- The "Selected job activity" panel is now sized to exactly fit
  `maxJobActivityRows`, derived from `widget.List`'s own row metrics, so all
  three rows are visible without a scrollbar regardless of theme or DPI.

**Settings tab:**
- The form is wrapped in a vertical scroll so it no longer dictates the window's
  minimum height (tab containers size to the tallest tab); it scrolls on short
  screens instead.
- Label-only sections (Application, Queue, About) are condensed, while
  separators and the editable Storage fields keep normal spacing so dividers
  have breathing room and entry boxes stay visibly separated.

**Window sizing:**
- Together these changes drop the minimum window height from ~891px to ~570px,
  so the window can be resized noticeably shorter and fits comfortably on 720p
  screens.

## 0.10.1 - 2026-06-24

**Refactoring:**
- Unified icon asset naming from mixed scheme (big/16x16) to consistent
  size-based names (large/small) for clarity and maintainability.

**Build:**
- Windows build script now displays informative messages at each build step
  (version, output path, environment setup, icon embedding, compilation)
  to improve build transparency and aid troubleshooting.

## 0.10.0 - 2026-06-24

**Compact activity rows; per-job execution-time statistics seeded from log files.**

**Activity panel (one-line rows):**
- Each entry in the job log list is now a single truncated line using only the
  base name of the log file (e.g. `20260624-120000_Build.log`) instead of the
  full path. Long lines are clipped rather than wrapped, keeping the panel
  compact with many runs.
- History table retains the full log path for reference; base-name truncation
  applies only to the activity rows in the Jobs details panel.

**Execution-time statistics:**
- Added `DurationMS` field to `RunRecord`; the runner measures wall-clock
  start-to-finish and writes it to both the record and a `duration:` header
  line in the log file. `StartOnly` jobs record `0`.
- Added aggregate counters to `JobRuntime`: `RunCount`, `FailCount`,
  `LastDurationMS`, `AvgDurationMS`, `MaxDurationMS`. Updated after every
  completed run in `executeRun`.
- On startup the statistics are seeded from existing log files: the runner
  parses `state:` and `duration:` headers for each job's newest log files
  (bounded by `MaxLogFiles`). Legacy logs without a `duration:` line still
  count toward `RunCount`/`FailCount` but are excluded from duration
  aggregates so a missing duration cannot appear as a zero-millisecond run.
- A **Statistics** row in the Jobs details panel shows a one-line summary
  (`N runs, M failed, last X ms, avg Y ms, max X ms`) that refreshes after
  each run and is pre-populated from log files after a restart.

**Per-job overlap policy:**
- Added an `OverlapPolicy` field to `domain.Job`; a job can now override the
  global skip/queue default. `RunDue` resolves the effective policy per job
  (the job's value if set, otherwise `Config.OverlapPolicy`).
- The job dialog gains an overlap-policy selector with a
  "(Use global default)" option that saves empty so the job inherits the
  global setting. The details panel reflects the effective policy.

**Persisted global pause:**
- Added `Paused` to the config so the global "Pause all" state survives a
  restart. `SetGlobalPause` persists the new value, and the service initializes
  its paused state from config at startup — a paused install now relaunches
  paused instead of silently resuming the scheduler.
- The Pause-all/Resume-all button and scheduler-state label initialize from the
  persisted state.

**Window sizing (720p-safe):**
- Lowered the default window size to `1024×660` with a sensible `MinSize` so the
  window opens fully visible on a 1366×768 / 720p screen. Layout minimums in the
  Jobs view were tightened to match.

**Packaging:**
- Added portable-distribution helpers: `scripts\package-windows.bat` builds and
  bundles `gosentry.exe`, `README.md`, and `CHANGELOG.md` into a versioned
  `.zip`; `scripts/package-linux.sh` does the same for `linux-amd64` and
  `linux-arm64` into `.tar.gz` archives.

**Internal cleanup:**
- Split `ui/jobs_view.go` into focused files (`jobs_view_details.go`,
  `jobs_view_helpers.go`) to bring it back under the file-size guideline.
- Removed the one-time YAML→JSON import path (shadow structs, `importYAML*`,
  legacy path names) now that the 0.9.0 transition window has passed;
  `go.yaml.in/yaml/v4` is dropped from `go.mod`.
- Post-field-test sweep of stale diagnostics, obsolete autostart-migration code,
  and noisy README/ignore rules. The startup-timing History event is retained.
- Removed the completed release-milestone docs and trimmed `ROADMAP.md` to open
  items only.

## 0.9.0 - 2026-06-24

**Storage migrated to JSON; queue execution modes; failure notifications; tray left-click; Fyne 2.7.4.**

**Storage and data model:**
- Settings and jobs now stored as `gosentry.json` and `jobs.json` (2-space indented JSON).
  On first run after upgrading, existing `gosentry.yaml` / `jobs.yaml` files are imported
  automatically and rewritten as JSON; the YAML files are not deleted.
- Removed `SuccessExitCodes` field. Exit-code handling is now fixed: exit code 0 = success,
  any nonzero exit code = failure. Jobs relying on nonzero success codes must switch to
  `StartOnly` mode if the exit code is irrelevant.

**Execution modes and overlap policies:**
- Added `ExecutionMode` (parallel/sequential) and `OverlapPolicy` (skip/queue) settings in
  Settings under a new Queue group.
  - **Parallel mode** (default): all due jobs start simultaneously.
  - **Sequential mode**: due jobs run one at a time, in order.
  - **Skip policy** (default): if a job comes due while its previous run is still active, the new
    run is discarded.
  - **Queue policy**: if a job comes due while running, the run is held and automatically started
    when the current run completes.
- Both settings are persisted to `gosentry.json` and validated on load; defaults ensure
  backward compatibility.

**Notifications and command input:**
- Failed job runs now raise a desktop notification (when enabled in Settings) with the job name
  and failure detail. Notifications fire for scheduled and manual runs; internal activity events
  are not notified.
- Added a Browse button next to the Command field in the job dialog for file picker selection.

**UI and platform integration:**
- Removed all PySentry legacy code: registry autostart entries (Windows), systemd and desktop
  file cleanup (Linux).
- Updated `.gitignore` and `.dockerignore` to track `gosentry.json` / `jobs.json` instead of
  legacy YAML filenames; added `*.yaml` wildcard to ignore legacy files during import.
- Moved developer documentation (Requirements, Build, Run From Source, Project Layout, Dependencies)
  out of README into `docs/DEVELOPMENT.md`. README now focuses on end-user content.

**Icons and tray:**
- Regenerated all icon assets with feathered color-to-alpha so the rounded-tile boundary is
  transparent — the opaque white halo visible on dark taskbars and trays is gone.
- Rebuilt `gosentry.ico` as a multi-size file (16 hand-tuned + 32/48/256 from the large PNG)
  and added a dedicated 16×16 icon for the Windows tray.
- Per-platform icon wiring: Windows window/taskbar uses the ICO resource so GLFW selects the
  right frame per slot; Windows tray uses the 16×16 ICO; Linux titlebar uses `IconSmall()` for
  a crisp ~16 px `_NET_WM_ICON`.
- Left-clicking the tray icon now shows and focuses the main window without opening the menu;
  the explicit "Show" menu item is preserved for right-click access.

**Performance:**
- Upgraded Fyne 2.6.3 → 2.7.4 (systray 1.11.0 → 1.12.1): startup time drops from ~644 ms to
  ~414 ms (−36%).
- Moved Windows-only runner tests into `runner_windows_test.go` (guarded by `//go:build windows`)
  to fix Linux test build.

## 0.8.0 - 2026-06-23

**Desktop notifications for failed jobs; Browse button for command paths.**

- Failed job runs now raise a desktop notification (title "GoSentry: Job Failed", body shows the job name and failure detail) when the "Show desktop notifications for failed jobs" setting is enabled. Notifications fire for scheduled and manual runs only; internal activity events are not notified.
- Added a Browse button next to the Command field in the job dialog so users can pick an executable from a file picker instead of typing the full path.

## 0.7.0 - 2026-06-23

**Execution modes and overlap policies for parallel and sequential job dispatch.**

- Added `ExecutionMode` setting (parallel/sequential) and `OverlapPolicy` setting (skip/queue) in Settings under a new Queue group.
  - **Parallel mode** (default): all due jobs start simultaneously.
  - **Sequential mode**: due jobs run one at a time, in order; a new job waits for the previous one to finish.
  - **Skip policy** (default): if a job comes due again while its previous run is still active, the new run is discarded.
  - **Queue policy**: if a job comes due again while running, the run is held and automatically started when the current run completes.
- Both settings are persisted to `gosentry.json` and validated on load; defaults ensure backward compatibility with existing installations.
- Added comprehensive unit tests verifying parallel start, sequential serialization, skip drops, and queue re-runs.
- Manual runs (`RunNow`) respect sequential mode: refused while any other job is running.
- No observable behavior changes with default (parallel/skip) settings; installations upgrading from earlier versions continue unchanged.

## 0.6.0 - 2026-06-22

**PySentry legacy code removed.**

- Removed all PySentry registry autostart entries (Windows), systemd and desktop file cleanup (Linux), and associated legacy code paths.
- Updated `.gitignore` and `.dockerignore` to ignore `gosentry.json` / `jobs.json` instead of the old YAML filenames;
  added `*.yaml` wildcard to ignore legacy files during the import window.
- No observable behavior changes; codebase cleanup after migration from PySentry naming.

## 0.5.0 - 2026-06-22

**Storage migrated from YAML to JSON; exit-code flexibility removed.**

- Settings and jobs are now stored as `gosentry.json` and `jobs.json` (2-space indented JSON).
  On first run after upgrading, existing `gosentry.yaml` / `jobs.yaml` files are imported
  automatically and the JSON files are written; the YAML files are not deleted and can be
  removed manually.
- Removed `SuccessExitCodes` field from jobs. Exit-code handling is now fixed: exit code 0 is
  success, any nonzero exit code is failure. Jobs that relied on nonzero success codes will now
  show "Failed"; update those jobs to use `StartOnly` mode if the exit code is irrelevant.
- Deleted `runner/exitcodes.go`; simplified `runStateDetail` accordingly.
- Tests updated: JSON round-trip tests, YAML-import tests for both config and jobs,
  exit-code flexibility tests removed.

## 0.4.0 - 2026-06-22

**Architectural milestone: completed refactoring and reached target architecture.**

- Completed Phase 5 refactoring: hardening, testing, and documentation.
  - Surface all save/cleanup errors from service and storage; no more silently swallowed `_ = ...` on persistence.
  - Introduced `platform/autostart.Manager` interface with per-platform implementations (Windows, Linux, other); inject into service instead of calling package functions.
  - Filled test gaps: folder filtering, log cleanup (count and age), settings persistence and migration, concurrent run prevention.
  - Verified `go test -race ./...` passes on Windows; no data races in the refactored codebase.
  - Updated `docs/ARCHITECTURE.md`, `docs/TESTS.md`, and README with final package structure and build/test instructions.
- **Refactoring target reached:** Service layer owns all state and is the sole writer; UI is a thin view marshaling updates via `fyne.Do`; core engines are stateless and injectable; domain layer is pure with no test noise.
- Known follow-ups recorded in `ROADMAP.md`:
  - Linux test build is currently broken (Windows-only test symbols need `//go:build windows`); will fix separately.
  - File-size soft limits exceeded in a few places; revisit when next editing those files.
- No observable behavior changes.

## 0.3.6 - 2026-06-22

- Completed Phase 4 refactoring: carved up the GUI into focused, testable components.
  - Renamed `src/gui` → `src/ui` and split monolithic `app.go` into `run.go` (lifecycle) and `mainwindow.go` (view construction).
  - Extracted view components into separate files: `jobs_view.go`, `job_dialog.go`, `history_view.go`, `settings_view.go`.
  - Extracted platform wiring into separate files: `tray.go`, `singleinstance.go`, `layout.go`.
  - Removed forbidden platform imports (autostart, desktop, paths) from `src/ui`; all platform concerns now flow through `app.Service`.
  - Upgraded Fyne from v2.5.3 to v2.6.3 to enable `fyne.Do` for cross-thread widget marshaling (resolves concurrency issue #4).
- Added `docs/PERFORMANCE.md` with measured startup-time analysis: the ~290ms increase from Phase 4 is entirely the Fyne 2.6.3 upgrade's `w.Show()` cost, not the restructuring.
- Added `docs/PERFORMANCE.md` and wired post-Fyne-2.7.x re-check into `ROADMAP.md`.
- No observable behavior changes; continued internal refactoring toward separated concerns and testability.

## 0.3.5 - 2026-06-19

- Completed Phase 3 refactoring: application service and state management.
  - Added `app.Service` as the single owner of application state (job registry, settings, run history).
  - Implemented event-driven observer dispatch: Services can emit events (JobAdded, JobChanged, etc.) to decouple state changes from UI updates.
  - Added `app.Clock` interface for testable time-dependent behavior in scheduler and run tracking.
  - Converted scheduler to drive app.Service instead of directly managing domain state.
  - Created `app.Format` helpers for display rendering (job names, schedule summaries, run times).
  - Added comprehensive unit tests for app.Service and supporting types.
- No observable behavior changes; continued internal refactoring toward separated concerns and testability.

## 0.3.4 - 2026-06-19

- Completed Phase 2 refactoring: domain cleanup and value object extraction.
  - Split durable job configuration (`domain.Job`) from transient execution state (`domain.JobRuntime`), keyed by job ID.
  - Added `domain.Schedule` value object with `Parse`, `Validate`, and `Next(time.Time)` methods for cron/interval parsing.
  - Migrated scheduler to parse schedules once at load/edit instead of per tick, removing duplicated parsing.
  - Made `RunJob` pure: runner no longer mutates jobs, returning only `RunRecord` for the caller to fold into runtime state.
  - Simplified `storage.normalizeJobs` to touch only durable configuration; runtime initialization moved to `domain.NewRuntime`.
- No observable behavior changes; continued internal refactoring toward separated concerns.

## 0.3.3 - 2026-06-18

- Completed Phase 1 refactoring: split the flat `src/core` package into specialized, focused packages:
  - `src/domain` for pure types (Job, RunRecord, Config)
  - `src/storage` for persistence (Load/Save, Paths, YAML helpers)
  - `src/runner` for job execution (RunJob orchestration, logging, exit codes)
  - `src/scheduler` for timing loop
  - `src/platform/winproc` for cross-platform hidden window configuration
  - `src/platform/autostart` for system autostart integration
  - `src/platform/desktop` for desktop environment integration
  - `src/app` for application-level code (Version, future Service layer)
- No observable behavior changes; internal structure improvements only.

## 0.3.1 - 2026-06-17

- Changed startup timing in History to measure until the main window is actually shown instead of stopping during UI construction.
- Added a separate startup History message for autostart launches that begin hidden in the tray.

## 0.3.0 - 2026-06-17

- Renamed the project from PySentry to GoSentry across the GUI, module path, build scripts, generated artifacts, desktop integration, and documentation.
- Renamed the command package to `cmd/gosentry` and Windows resource script to `packaging/windows/gosentry.rc`.
- Renamed portable application settings from `pysentry.yaml` to `gosentry.yaml`, while keeping one-time read compatibility for existing `pysentry.yaml` files.
- Renamed build artifacts from `pysentry-*` to `gosentry-*`.
- Updated autostart and Linux desktop integration to use GoSentry names while cleaning up older PySentry autostart entries.

## 0.2.5 - 2026-06-16

- Stabilized the Jobs details panel so long selected-job fields do not resize the right pane or application window.
- Switched Windows autostart from `HKCU Run` entries to a Startup folder shortcut, fixing executable paths that contain spaces.
- Added `--start-in-tray` autostart launches for Windows and Linux so sign-in startup does not open the main window.
- Added Windows shortcut tests and Linux autostart desktop-entry tests for the new startup-in-tray behavior.
- Updated autostart documentation and architecture notes for the Startup shortcut and XDG desktop-entry behavior.
- Documented the Windows VirtualBox/RDP OpenGL startup failure and the Mesa software OpenGL workaround.

## 0.2.4 - 2026-06-16

- Prevented repeated application launches by forwarding a second start attempt to the already running instance.
- A second instance now asks the first instance to show and focus the existing window, then exits.

## 0.2.3 - 2026-06-15

- Changed History to use chronological ordering with new records appended at the bottom.
- Replaced the History list with a compact table.
- Added Time column sorting in both ascending and descending directions.
- Made History table columns user-resizable through the native Fyne table header.
- Shortened the Log column display to file names instead of full paths.
- Unified UI event timestamps with command run timestamps.

## 0.2.2 - 2026-06-15

- Added Linux desktop integration that installs a user-level `.desktop` file and icon so taskbars can match the running window to the GoSentry icon.
- Added the installed icon path to Linux autostart desktop entries when available.
- Added `ARCHITECTURE.md` with a component interaction diagram and moved project documentation under `docs/`.
- Adjusted the Mermaid architecture diagram to avoid line-break syntax that breaks rendering in Gitea.
- Stabilized the Jobs tab pane layout so switching jobs does not move the divider.
- Added startup timing to the History tab.

## 0.2.1 - 2026-06-15

- Fixed Docker release scripts so container builds keep Go in `PATH`.
- Disabled Go VCS stamping for Docker release builds to avoid failures when `.git` metadata is unavailable inside the container.
- Made Docker release builds write `dist/` artifacts with the current user's UID/GID instead of root ownership.
- Added `ROADMAP.md` with planned delivery formats and packaging priorities.
- Cleaned `.gitignore` for the current Go/Fyne project and kept the local `_gsdata_/` rule.
- Added README links to official Go/Fyne sites and source repositories useful for dependency mirroring.
- Documented Windows dependency installation steps for Go and MSYS2 UCRT64 GCC.

## 0.2.0 - 2026-06-15

- Added working autostart support with status diagnostics in Settings.
- Switched Linux autostart to XDG Autostart `.desktop` files and clean up the legacy user systemd unit.
- Fixed Windows autostart status detection by parsing `HKCU Run` values and comparing executable paths reliably.
- Added background job execution so the GUI does not block while commands run.
- Suppressed Windows console windows for scheduled and manual command runs.
- Added application version display in the window title, Settings, and build artifact names.
- Moved release artifact commands from `Dockerfile` into `scripts/build-release-linux.sh` with interactive target selection.
- Added release build targets for Linux amd64, Linux arm64, and Windows amd64.
- Added README dependency installation notes and official Go/Fyne links.

## 0.1.0 - 2026-06-14

- Added the initial Fyne desktop GUI.
- Added YAML settings and single-file YAML job storage.
- Added `@every` and standard 5-field cron schedules.
- Added manual and scheduled command runs with per-run log files.
- Added job folders, history, global pause, and Windows tray support.
- Added Windows and Linux build helpers.
