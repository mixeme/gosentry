# Whole-project review — action plan

Working document for the findings of the 2026-08-05 whole-project review. It is
not part of the permanent doc set: delete it once every item below is either
done or moved to [ROADMAP.md](ROADMAP.md), the way `TEST_REVIEW_PLAN.md` was
retired.

The rules the findings are judged against live in [STANDARDS.md](STANDARDS.md)
and [ARCHITECTURE.md](ARCHITECTURE.md). Anything listed under "Intentional
behavior" in STANDARDS is not reported as a bug; where this review disagrees
with such an entry it says so explicitly as a **challenge**.

## Baseline the review started from

Measured on the 1.0.2 tree (`c8a4d31`), MSYS2 UCRT64 / CGO on:

- `go vet ./...` — clean.
- `go test -race ./...` — all packages pass. `src/ui` alone takes **229 s**;
  everything else finishes in under 8 s.
- Engine coverage, merged profile over `domain`, `storage`, `runner`,
  `scheduler`, `app`: **84.0%** (TESTS.md records 84.4% at the 2026-08-04
  review). The 0.4 pp dip is *not* item 4.1 — it is new 1.0.2 code that arrived
  untested: `storage.PeekKeepRunningInTray` sits at 0%. It is a startup entry
  point like `OpenStore` and `ResolvePaths`, so if it is meant to stay
  uncovered it belongs in TESTS.md's "Functions deliberately at 0%" list, which
  currently does not name it.
- 171 test functions in the tree. TESTS.md names 171 as well, but the sets do
  not match: two of the names it documents no longer exist, and two tests that
  do exist are undocumented (items 4.1, 4.2).
- 77 Go files, ~10 500 lines including tests. Two direct dependencies.

Overall finding: **the project is in good health.** The engine layering
(`domain` → `storage`/`runner`/`scheduler` → `app` → `ui`) holds, the locking
contract on `Service.mu` is stated and obeyed, and the UI layout code — usually
the first thing to rot in a desktop app — is the strongest part of the codebase:
sizes are measured from the theme, the helpers are named, and the geometry is
pinned by tests that re-run under a scaled theme. The documentation set is
unusually complete and, with the exceptions in §4, accurate.

The findings below are therefore mostly about **the paths that only show up
after the app has been running for a while** (§3.1, §6.3), **durability of the
JSON files** (§6.2), and **one confirmed Windows quoting bug** (§6.1).

Severity follows the whole-project review convention: *medium* means it gets a
regression test with the fix.

---

## 1. Architecture and project structure

### 1.1 `Service.Store()` is the hole in "the Service is the sole owner" — medium

[service.go:170](../src/app/service.go) hands callers the raw `*storage.Store`.
Its own doc calls the surface transitional ("later phases narrow this"); the
phase never came. Eight UI sites read Service-owned state straight through it:

| Site | Reads |
|---|---|
| [jobs_view.go:60,61,64,66,79](../src/ui/jobs_view.go) | `Config.Paused`, `Config.JobListView`, `Config.OverlapPolicy`, `Config.DefaultTimeoutSeconds` |
| [settings_view.go:30](../src/ui/settings_view.go) | the whole `Config`, held as a live pointer for the session |
| [run.go:73,78](../src/ui/run.go) | `Config.KeepRunningInTray`, `Config.Theme` |
| [mainwindow.go:92](../src/ui/mainwindow.go) | `Paths.LogsDir`, from inside the notification path |

This contradicts ARCHITECTURE ("the UI reads it through typed events, never
through shared mutable state") and STANDARDS. It is not a live data race
**today**, but only because of an invariant nothing writes down and nothing
enforces: every writer of `store.Config` (`UpdateSettings`, `SetGlobalPause`,
`SetJobListView`) happens to be reached from the Fyne main thread, so the
unlocked UI reads are serialised with them by accident. One background writer —
say, a future auto-reload of `jobs.json`, or moving log cleanup off the UI
thread — turns all eight into races that `-race` will not catch, because no test
drives them concurrently.

Fix: give the Service typed accessors that copy under `mu` (`Config()`,
`LogsDir()`), convert the eight call sites, and either unexport `Store()` or
reduce it to what the tests actually need.

### 1.2 `SchedulerStateChanged` is emitted and never consumed — low

[events.go:38](../src/app/events.go) documents it as "The UI uses it to update
the pause/resume control and status text." No observer handles it: the single
listener in [mainwindow.go:67](../src/ui/mainwindow.go) type-asserts only
`RunRecorded`, `ErrorOccurred`, and `JobsLoaded`. The Jobs toolbar keeps its own
`schedulerPaused` copy and relabels the button inside its own tap handler
([jobs_view.go:271](../src/ui/jobs_view.go)).

It works because the tap handler is the only thing that can pause today. That is
exactly the coupling the event bus exists to remove. Either consume the event
and delete the local mirror, or delete the event and drop the claim.

### 1.3 The "exhaustive type-switch" the doc promises does not exist — low

The same comment block says the sealed `Event` interface means "a UI listener
can exhaustively type-switch over them and the compiler will flag a new event
type that a switch forgot to handle." Go has no exhaustiveness check on type
switches, and the one listener does not even use a switch — it uses three
independent assertions. The comment claims a safety property that is not there,
which is how 1.2 went unnoticed. Reword it to say what sealing actually buys
(observers cannot be handed an event type from outside the package).

---

## 2. Complexity against the size of the project

Nothing here is over-abstracted: the `Clock` interface, the `runJob` seam, and
the `autostart.Manager` interface each have a real test seam or a real second
implementation. The findings run the other way — code that is still there after
its reason left.

### 2.1 Two full `jobs.json` rewrites per run that cannot change the file — medium

[startRunLocked](../src/app/run.go) calls `s.store.SaveJobs(s.jobs)` on every run
start and [executeRun](../src/app/run.go) calls it again on every run finish.
Neither function assigns to a single `domain.Job` field: everything they touch
lives on `JobRuntime`, which is explicitly never persisted
([runtime.go:5](../src/domain/runtime.go)). Both calls therefore re-serialise
and rewrite the identical bytes. `SetGlobalPause`
([operations.go:182](../src/app/operations.go)) does the same — its durable
change is `Config.Paused`, saved separately by `SaveConfig`.

The cost is not only I/O. `startRunLocked` carries a five-line rollback block
and a regression test (`TestStartRunLockedRollbackOnSaveFailure`) guarding a
write that can never change the file's content, and the write happens under
`Service.mu` (see 3.2). Removing the three calls removes the I/O, the rollback,
and the failure mode at once.

Care needed: the review found no durable field written on these paths, but this
should be re-verified against the `domain.Job` definition when the change is
made, and `TestStartRunLockedRollbackOnSaveFailure` retired deliberately rather
than left failing.

### 2.2 `runner.logArguments` is an alias of `runner.LogArguments` — low

[invocation.go:68](../src/runner/invocation.go) —
`func logArguments(a string) string { return LogArguments(a) }`. A leftover from
exporting the function. Four call sites; inline them and delete it.

### 2.3 `collectActivity` always returns an empty slice at startup — low

[mainwindow.go:30-37](../src/ui/mainwindow.go) builds an `initialRuntimes` map
purely to feed [collectActivity](../src/ui/history_view.go), which merges
`JobRuntime.Logs` across jobs. History is session-only by design, so at
construction time every `Logs` slice is empty and the result is always `nil`.
The function's own comment says it is kept "for future history loading from log
metadata" — a feature that is not on the ROADMAP.

Either delete the twelve lines, or record the placeholder in STANDARDS so the
next reviewer does not re-report it. Its two unit tests are fine either way —
they test the merge, not the caller.

### 2.4 The ROADMAP size table is stale — info

[ROADMAP.md](ROADMAP.md) lists the files over the ~250-line guideline as of
1.0.0. Measured today:

| File | ROADMAP | Now |
|---|---|---|
| `src/app/operations.go` | 490 | 490 |
| `src/ui/jobs_view.go` | 355 | 361 |
| `src/ui/settings_view.go` | 277 | **304** |
| `src/storage/store.go` | 265 | **299** |
| `src/app/run.go` | 287 | 287 |
| `src/ui/history_view.go` | 282 | 282 |

Refresh the numbers when the split item is picked up; the trend is the point,
not the individual figures.

---

## 3. Code quality

### 3.1 History grows without bound, and every run pays for it — medium

This is the most consequential finding in the review, because it only appears in
the mode the app is designed to run in: left in the tray for days.

`events` in [mainwindow.go:73](../src/ui/mainwindow.go) is appended to on every
`RunRecorded` and never trimmed. Each entry is a full `domain.RunRecord`,
including `Output` — the complete captured stdout and stderr of the run. Then,
on every single event, `refresh()` runs:

- `resort()` — copies the whole slice and sorts it
  ([history_view.go:181](../src/ui/history_view.go));
- `setColumnWidths()` → `historyColumnWidths(rows)` — builds three
  slices of length *n* and calls `fyne.MeasureText` once per non-empty value in
  each of the Job, Detail, and Log columns
  ([history_view.go:104](../src/ui/history_view.go)).

So the per-run cost is O(*n* log *n*) sorting plus up to 3*n* text measurements
on the UI thread, with *n* growing forever. One job on `@every 10s` produces
~8 600 records a day. `JobRuntime.Logs` is capped at 50 by `maxJobLogs`; the
History slice — the one that actually accumulates — is not capped at all.

Fix in two parts: cap the History slice (a ring buffer, or a `maxHistoryRows`
mirroring `maxJobLogs`), and stop rescanning every row for column widths on
every event — widths only ever grow, so fold the new record into the current
maxima instead of recomputing from scratch.

This is the one finding worth a measurement before and after, since STANDARDS
already treats measured geometry as the standard of proof.

### 3.2 Blocking file I/O under `Service.mu` — medium

`Service.mu` is the lock the Fyne main thread takes on every `Jobs()` and
`Runtime()` call — that is, on every UI refresh. Three things do file I/O while
holding it:

- [executeRun](../src/app/run.go) calls `runner.CleanupLogs` — a directory scan
  plus up to `MaxLogFiles` unlinks — under `mu`, after every run.
- Every `SaveJobs` / `SaveConfig` is a full JSON marshal and write under `mu`.
- [adoptJobsLocked](../src/app/service.go) calls `runner.SeedStats` under `mu`,
  reached from `UpdateSettings` on the UI thread.

None of it needs the lock: cleanup takes only the values already snapshotted
into `runEnv`, and seeding only needs the job list. Move them outside the
critical section, or snapshot and run them after `mu.Unlock()` the way the event
emission already does.

`SeedStats` also opens every log file **twice** — once in `readLogJobID` and
again in `readLogHeader` ([seed.go:59,98](../src/runner/seed.go)) — and the
first pass is not bounded by `maxFiles`, so it touches every `.log` in the
directory. One pass returning `(jobID, state, duration)` halves the syscalls.

### 3.3 `StartOnly` leaks a goroutine per run and mis-owns the process — medium

[runner.go:39](../src/runner/runner.go) builds the fire-and-forget invocation
with `jobInvocation(ctx, …)`, which uses `exec.CommandContext`. After `Start()`,
os/exec spawns a watcher goroutine that blocks until either `Wait()` returns or
the context is done. `StartOnly` never calls `Wait` — that is the whole point —
so the goroutine lives until the app exits, one per StartOnly run, and then
calls `Kill` on a process whose handle `startJobOnly` already `Release`d.

The kill is harmless in practice (a released handle makes it fail), but the
leak is real and the ownership is the wrong shape: a job the runner explicitly
stops waiting for should not be tied to the app's lifecycle context at all. Use
`exec.Command` (or `context.Background()`) for the StartOnly branch and say in
STANDARDS whether a started process is expected to outlive GoSentry.

### 3.4 `InstallDesktopIcon` swallows its error — low

[platform.go:11](../src/app/platform.go) —
`if iconPath, err := desktop.InstallDesktopIntegration(…); err == nil { … }`.
The error is discarded with no dialog, no History event, and no log line. That
is the silent `return` STANDARDS forbids. On Linux the visible symptom is a
generic dock icon with no explanation. Emit `ErrorOccurred`.

### 3.5 `RunDue` keeps only the last start error — low

[run.go:92](../src/app/run.go) — `startErr = err; continue`. If two jobs fail to
start on the same tick, the user sees one message. Join them (`errors.Join`) or
emit one event per failure.

### 3.6 Settings re-implements `validateConfig` — low

[settings_view.go:136-158](../src/ui/settings_view.go) validates max log files,
max log age, jobs file, logs dir, and default timeout with its own messages,
before `UpdateSettings` validates the same five with different messages
([operations.go:456](../src/app/operations.go)). The UI genuinely needs the
`strconv` parse; it does not need a second copy of the rules. Parse in the UI,
validate in the Service, and show what the Service returns.

---

## 4. Documentation and comments

The doc set is accurate about design and rationale. What has drifted is the
inventory.

### 4.1 Two documented tests were silently deleted — medium

[TESTS.md](TESTS.md) lists `TestJobListViewIsCompact` and
`TestDefaultConfigUsesDetailedJobList` under `src/domain/config_test.go`.
Neither exists. Commit `5b0e6fe` ("Wire KeepRunningInTray to runtime …")
**rewrote** that file to hold `TestAutostartArguments` and
`TestResolveStartHidden` instead of appending them, and the two older tests went
with it.

**This was not the test-suite review's doing, and it was not a decision.** The
2026-08-04 review deleted exactly three tests — `TestCleanupLogsKeepsFilesWithinAgeLimit`,
`TestRunDueEmptyOverlapInheritsGlobal`, and `TestSameWindowsPathHandlesSpaces` —
each after measuring byte-identical coverage against a survivor whose assertions
were a superset, and each recorded in `TEST_REVIEW_PLAN.md` and in the CHANGELOG.
Its deletion commit `2ef18e7` never opened `config_test.go`; only `29ce94c`
(which created the two tests) and `5b0e6fe` ever touched that file.

What settles it is what `5b0e6fe` did to the documentation: it **added** the two
new test rows to TESTS.md while **leaving the two old rows in place**, i.e. it
documented the file as holding all four. The doc moved in the opposite direction
from the code. A deliberate removal looks like `2ef18e7`, which took its three
rows out of TESTS.md in the same commit. Nothing in the commit message, the
1.0.2 CHANGELOG, or STANDARDS mentions the loss.

Accidental, however, does not mean both are worth having back. Only one was
pulling weight:

- **`TestJobListViewIsCompact` — restore it.** Its unique assertions are that
  `""` and a differently-cased `"Compact"` both read as detailed. Neither holds
  anywhere else now: `TestSetJobListViewNormalizesUnknownValue` (`app`) covers
  only the unrecognised-value path through `SetJobListView`. The empty case is
  live rather than theoretical — `loadOrCreateConfig` does **not** normalize
  `job_list_view` the way it normalizes `theme`, so a config written before the
  field existed reaches `IsCompact()` empty and depends on exactly this
  behaviour. STANDARDS §Config file compatibility also requires it by name:
  "Each of the three gets a test: the default in `storage`, the normalization
  in `domain`, and a round-trip through the real config file in `app`." The
  `domain` one is the one that disappeared, so a rule STANDARDS calls mandatory
  is currently unenforced.
- **`TestDefaultConfigUsesDetailedJobList` — do not restore it; take its row
  out of TESTS.md instead.** `TestLoadOrCreateConfigCreatesDefaultsOnFirstRun`
  (`storage`) already asserts `got.JobListView == domain.JobListViewDetailed`,
  through the real load path, which makes it a strict superset — and STANDARDS
  puts the default test in `storage`, not `domain`. Under TESTS.md principle 9
  this is a legitimate deletion; it simply was never made deliberately.

Neither loss moved the number: `IsCompact` and `DefaultConfig` both measure
100% today, exercised through their callers. What was lost is an assertion, not
statement coverage — which is the exact case TESTS.md principle 9 exists to
name ("Identical coverage alone is *not* grounds for deletion").

The surviving test is recoverable verbatim from
`git show 5b0e6fe^:src/domain/config_test.go`.

### 4.2 `src/ui/notify_timing_test.go` is undocumented — low

`TestNotificationTimingFormatLine` and
`TestAppendNotificationTimingLogWritesHeaderAndRow` were added in 1.0.2 with no
TESTS.md entry. Add the file's table.

### 4.3 The window-size comment describes a feature that is frozen — low

[run.go:19](../src/ui/run.go): "later launches restore the last size from
preferences." Nothing ever writes `window.width` / `window.height` — ROADMAP
records the feature as deliberately frozen. The comment is wrong and the two
`prefs.FloatWithFallback` reads are dead code that make it look implemented.
See also 8.2.

### 4.4 README says "Pause all"; the button says "Disable auto" — low

[README.md](../README.md) step 6 under *Using The App*. The control is labelled
`Disable auto` / `Enable auto` ([jobs_view.go:261](../src/ui/jobs_view.go)).

### 4.5 A comment cites a function that no longer exists — low

[jobs_view_helpers.go:11](../src/ui/jobs_view_helpers.go) refers to
`app.Service.recordRun`. The function is `executeRun`.

### 4.6 README narrows when notifications fire — low

"…whenever a scheduled or manual run exits with a non-zero exit code." The
condition is `State == "Failed"`, which also covers timeouts and processes that
failed to start.

### 4.7 The coverage command in TESTS.md does not run on the documented shell — info

TESTS.md gives the `-coverpkg` invocation in bash form. In the PowerShell
environment DEVELOPMENT.md prescribes for Windows, PowerShell splits the
comma-separated package list and the command fails with
`directory not found`. It needs `--%` (or the whole flag quoted). Worth a note
next to the command, since it is the one measurement the doc asks reviewers to
reproduce.

---

## 5. Readability and maintainability

### 5.1 `newJobsView` is one 330-line constructor over shared mutable locals — medium

[jobs_view.go:30-361](../src/ui/jobs_view.go). Twelve closures share
`jobs`, `runtimes`, `selected`, `selectedFolder`, `filteredJobs`, `listView`,
and `schedulerPaused`, and several of them patch two or three of those in
sequence before calling `refreshView`. Understanding any one handler means
reading all of them, because the invariant "`selected` indexes `jobs`, and the
list's selection index indexes `filteredJobs`" is maintained by hand in five
places.

ROADMAP already tracks the split. This review adds the reason it matters beyond
line count: the state, not the length, is what makes it hard. Extracting a small
`jobsViewState` struct with `selectByID`, `applyFilter`, and `snapshot` methods
would shrink the file and make 5.2 impossible.

### 5.2 Selection is tracked by slice index, not by job ID — medium

`selected` is an index into a snapshot of the jobs slice. Every path that can
change the slice — create, delete, filter — patches it explicitly. The path that
replaces the whole list does not: adopting a different jobs file emits
`JobsLoaded` plus a broad `JobChanged`, the observer calls `refresh()`, and
`refreshView` calls `updateDetails(selected)` with an index from the *previous*
list. The details pane then describes whichever job now happens to sit at that
index, while the list's highlight is untouched.

Track the selection by `Job.ID` and resolve it to an index at render time.

### 5.3 `operations.go` mixes three jobs in one file — low

490 lines: the public mutating operations, the `…Locked` state helpers only they
call, and the pure validators/normalizers. ROADMAP already names this as the
clearest of the six splits; nothing to add except that it is still the worst
overage.

### 5.4 The nested `fyne.Do` has no explanation — low

[mainwindow.go:71 and 85](../src/ui/mainwindow.go) — the observer's body already
runs inside `fyne.Do`, and the failure-notification block opens a second one.
The nesting is deliberate (it defers the toast by one main-thread hop so
`UIQueuedAt` can measure that hop for `notify-timing.log`), but nothing says so,
and a reader's first instinct is to "simplify" it away and lose the
instrumentation. The same block also calls `appendNotificationTimingLog` — a
file open, stat, and write — on the UI thread.

Add the sentence that explains the nesting, and move the log append off the main
thread.

---

## 6. Logical errors

### 6.1 Windows shell quoting picks the wrong program path — medium (reproduced)

[quoteLeadingWindowsProgramPath](../src/runner/invocation_windows.go) walks the
extension list `.exe`, `.cmd`, `.bat`, `.com` **in list order** and takes the
first extension that appears anywhere in the string. It should take the
extension that appears *earliest*, and only at a token boundary. When the
program is a `.bat` or `.cmd` and any argument ends in `.exe`, the `.exe` in the
argument is found first and the entire command line is treated as the program
path.

Reproduced by running the function verbatim outside the build:

| Input (job `Command`) | Produced command line |
|---|---|
| `C:\My Tools\run.bat D:\in.txt` | `cmd.exe /S /C ""C:\My Tools\run.bat" D:\in.txt"` ✔ |
| `C:\My Tools\run.bat C:\Windows\System32\notepad.exe` | `cmd.exe /S /C ""C:\My Tools\run.bat C:\Windows\System32\notepad.exe""` ✘ |
| `C:\Program Files\App\deploy.cmd D:\stage\setup.exe` | `cmd.exe /S /C ""C:\Program Files\App\deploy.cmd D:\stage\setup.exe""` ✘ |
| `C:\dir.exexample\My Tool\run.bat` | `cmd.exe /S /C "C:\dir.exexample\My Tool\run.bat"` ✘ (never quoted) |

The two ✘ rows in the middle hand `cmd.exe` a single quoted token that is not a
file, so the run fails with a shell-level error the user cannot map back to
their job. The last row is the mirror image: a `.exe` substring inside a
directory name makes the check conclude the program path has no spaces, so a
path that *does* need quoting is left bare.

Reachable through normal use: it applies whenever the command does not resolve
as a direct executable path, which is what happens when the user types a whole
command line into the **Command** field — the shape the field's own placeholder
and the existing Joplin test fixture both demonstrate.

Fix: find the earliest extension match across all four extensions, and require
the character after it to be a space or end-of-string. Regression test with the
four rows above.

### 6.2 `gosentry.json` and `jobs.json` are written non-atomically — medium

[storage.writeJSON](../src/storage/store.go) is `os.WriteFile` — truncate, then
write. A crash, a power loss, or the process exiting during the write leaves a
truncated or empty file, and for `jobs.json` that is every job definition the
user has.

The exposure is larger than it looks because of 2.1: `SaveJobs` runs twice per
run, so the window is open constantly on a busy install. And `Service.Stop()` is
never called — `ui.Run` has no shutdown path, and the tray's Quit item goes
straight to `a.Quit()` ([tray.go:72](../src/ui/tray.go)) — so quitting while a
run is completing terminates the process mid-write with nothing to flush.

Fix: write to `<name>.tmp` in the same directory, `Sync`, then `os.Rename` over
the target. Rename is atomic within a volume on both supported platforms. The
same treatment is cheap for `runner/logfile.go`, though a torn log file costs
much less than a torn jobs file.

Worth pairing with a `Service.Stop()` call on shutdown, which also makes the
`ctx` cancellation the runner already implements actually reachable.

### 6.3 `PendingRuns` survives a pause and has no ceiling — medium

[executeRun](../src/app/run.go) drains the queue with
`rerun := runtime.PendingRuns > 0 && current.Enabled && !s.paused`. Nothing ever
*clears* the counter. Two consequences:

- **Pause leaks a run.** Pause the scheduler while a `queue`-policy job has a
  backlog, and the counter stays set. `refreshNextRunLocked` parks the job at
  "Scheduler paused" and the drain is skipped — correctly, and
  `TestRunDueQueueDrainSkippedWhenPaused` pins that. But after the user resumes,
  the stale counter is still there, and the next completed run of that job fires
  a deferred run that corresponds to an occurrence from before the pause.
  Disabling a job has the same shape: `SetEnabled(false)` does not clear it.
- **No ceiling.** A job whose runs take longer than its interval increments
  `PendingRuns` on every missed occurrence forever. The job then runs
  back-to-back indefinitely, and there is no bound, no warning, and nothing in
  the UI that shows the queue depth.

Fix: clear `PendingRuns` in `SetGlobalPause(true)` and in `SetEnabled(false)`,
and cap it (a small constant, or the number of occurrences in one interval).
Document the cap in STANDARDS next to the existing overlap-policy entry, and
show the depth in the details pane if it is capped.

### 6.4 `normalizeJobs` never resolves duplicate IDs — low/medium

[store.go:180](../src/storage/store.go) assigns an ID only when one is absent
(`job.ID <= 0`). A hand-edited `jobs.json` — a workflow the project explicitly
supports and README documents — with two entries carrying `"id": 5` produces two
jobs that share one `JobRuntime` entry, one schedule-cache entry, and one
`SeedStats` bucket. `findByIDLocked` returns the first, so editing or deleting
one silently targets the other; both runs write their state onto the same
runtime.

Fix: track seen IDs during normalization and reassign the later duplicate, which
is exactly what the existing `next` counter already computes.

### 6.5 Log file names collide within the same second — low

[logfile.go:25](../src/runner/logfile.go) builds
`20060102-150405_<name>.log`. Two runs of the same job in the same second — a
fast job re-run manually, or a queue drain of a sub-second command — write the
same path and the second silently overwrites the first. `SeedStats` counts files,
so the run history also under-counts. Add a disambiguating suffix when the path
already exists.

### 6.6 Two different averages for the same history — low

[updateStats](../src/app/run.go) keeps a truncating incremental mean
(`(avg*(n-1) + d) / n` in integer arithmetic, so the truncation error
compounds), while [aggregateLogStats](../src/runner/seed.go) computes an exact
`sum / count`. The same run history therefore reports a different average
depending on whether it was seeded from logs at startup or accumulated live —
and the two are mixed, because seeds are the starting values that `updateStats`
then folds new runs into. Keep a running sum on `JobRuntime` and divide on
read.

### 6.7 Absolute paths are not cleaned; relative ones are — low

[ResolveConfiguredPath](../src/storage/store.go) returns an absolute path
verbatim and only `Clean`s the relative case. `UpdateSettings` decides whether
the user is switching jobs files by comparing the resolved path to
`Paths.JobsPath` as strings, so `C:/data/jobs.json` and `C:\data\jobs.json` read
as two different files and trigger the adoption branch against the file the app
is already using. `filepath.Clean` on both sides fixes it.

### 6.8 Missed occurrences during downtime are dropped — challenge, not a bug

`adoptJobsLocked` computes each job's first `NextDue` from `time.Now()`, so
occurrences that fell while the app was closed never run and never appear in
History. This is the right default for a desktop scheduler, but it is not
written down anywhere — a user coming from cron with `anacron` habits will
assume the opposite. Add it to STANDARDS §Intentional behavior.

---

## 7. Legacy code and migrations

The file-compatibility discipline STANDARDS describes is genuinely followed:
`Config.JobsDir` → `Config.JobsFile` and the retired `"default"` theme value are
both converted on load, cleared, and covered by a `storage` test. Nothing found
that reads a shape the app cannot write. The findings are smaller.

### 7.1 `domain.RunRecord` carries dead `yaml:` tags — low

[record.go](../src/domain/record.go) tags all nine fields `yaml:"…"`. Nothing
serialises the type — History is session-only and log files are written as
hand-rolled text — and there is no YAML dependency in `go.mod`. Leftover from an
earlier format. Delete them, or convert to `json:` if the type is ever meant to
be persisted.

### 7.2 Two compatibility shims with no retirement plan — low

`Config.JobsDir` (pre-0.15) and `Theme == "default"` (pre-1.0.1) are both
read-only shims that rewrite the file into the current shape on the next save,
so each one becomes dead the moment a user's config has been saved once by a
current build. Neither has a note saying when it can go. Add "remove after
<version>" to each, or a single ROADMAP entry that retires both.

### 7.3 `autostart` exposes two public surfaces for one job — low

Each of the three implementations exports both the `Manager` methods and the
bare `SetAutostart` / `AutostartStatus` functions the methods delegate to. Only
the interface is used outside the package (plus the tests). Unexport the
functions.

---

## 8. Stubs and claimed-but-unimplemented behavior

### 8.1 "Cleanup disabled" is documented and tested but unreachable — medium

`CleanupLogs` documents `maxFiles <= 0` and `maxAgeDays <= 0` as "policy
disabled", and `TestCleanupLogsZeroLimitsDisableBothPolicies` pins it. The app
can never produce that state: `validateConfig` rejects both as
"must be a positive number" ([operations.go:469](../src/app/operations.go)), and
`loadOrCreateConfig` backfills 0 to 100 / 30 on load
([store.go:119](../src/storage/store.go)). So a user cannot turn log cleanup off
at all, by GUI or by hand-editing.

This is also inconsistent with `DefaultTimeoutSeconds`, where the project went
to real trouble — a pointer type, a documented three-state table, a dedicated
test — precisely so that a meaningful zero would survive.

Decide one way: either accept 0 as "unlimited" in `validateConfig` and stop
backfilling it (documented in STANDARDS alongside the timeout rule), or delete
the unreachable branch in `CleanupLogs` and its test. The first is the better
outcome — "keep everything" is a real thing to want from a log retention
setting.

### 8.2 Window-size preferences are read but never written — low

`prefs.FloatWithFallback("window.width", …)` in
[run.go:64](../src/ui/run.go) always returns the fallback because no code path
writes those keys. Dead reads plus a comment that claims otherwise (4.3).
Replace with the constants and leave a one-line pointer to the frozen ROADMAP
item.

### 8.3 `notify-timing.log` shares the retention budget of run logs — low

[appendNotificationTimingLog](../src/ui/notify_timing.go) writes into
`logs_dir` with a `.log` extension, so `CleanupLogs` counts it against
`MaxLogFiles` and will delete it once it ages past `MaxLogAgeDays`. It is
diagnostic instrumentation shipped in 1.0.2 for the "Faster Windows failure
notifications" ROADMAP item, with no note on when it comes out. Give it a
different extension (or a `diagnostics/` subdirectory — `CleanupLogs` already
skips directories) and add its removal to that ROADMAP entry.

Items 1.2 (`SchedulerStateChanged`) and 2.3 (`collectActivity`) also belong to
this section; they are written up above.

---

## 9. GUI: crutches and layout

**This section is close to clean, and that is the headline.** The rule in
STANDARDS — "a size that must follow the theme is measured at build time, not
written as a pixel constant" — is actually observed: `rowOverlap`,
`captionColumnWidth`, `textColumnWidth`, `activityRowsHeight`, and
`initialSplitOffset` all derive from the theme or from measured text, and the
`ui` tests assert the resulting geometry under two themes. The two raw numbers
that remain (`commandOutputScroll`'s 460×70 minimum and the `+1` rounding
allowance in `activityRowsHeight`) both carry a comment explaining why nothing
about them tracks the theme. No layout crutches found.

The remaining items are small.

### 9.1 `AutostartStatus` runs PowerShell synchronously on the UI thread — low

`settingsView` is constructed eagerly during `newMainView`, and its constructor
calls `refreshAutostartStatus()` → `svc.AutostartStatus()`. On Windows with
autostart enabled that reaches `readShortcut`
([autostart_windows.go:124](../src/platform/autostart/autostart_windows.go)),
which spawns `powershell.exe` and blocks on `CombinedOutput()` — the same
PowerShell cold start ROADMAP measures at 700–900 ms for notifications. It runs
before the window is shown, and again on every toggle of either checkbox.

Given the project already measures and cares about startup time
([PERFORMANCE.md](PERFORMANCE.md)), this is worth moving to a goroutine that
posts its result back through `fyne.Do`, with the label showing "Checking…"
meanwhile.

### 9.2 Two package-level mutable globals in `tray.go` — low

`mainWindowHidden` is justified and documented (Fyne exposes no
`Window.Visible`). `systemTrayRegistered` is not: it is process-global state
that no test can reset, and it exists only because `applyTrayBehavior` is called
from two places. Passing it, or hanging both flags off a small struct owned by
`Run`, removes the hidden coupling.

### 9.3 The activity list is refreshed twice per redraw — low

`refreshView` calls `dp.logs.Refresh()` immediately after `updateDetails`, which
already ends in `d.logs.Refresh()` ([jobs_view.go:91](../src/ui/jobs_view.go),
[jobs_view_details.go:103](../src/ui/jobs_view_details.go)). Harmless, but it is
the shape of duplicate-refresh bug that `TestToolbarButtonRedrawsRowAndDetails`
was written to prevent.

### 9.4 The folder-filter rebuild is repeated three times — low

`folderSelect.Options = folderOptions(jobs); folderSelect.Refresh()` appears
verbatim in the create, edit, and delete handlers. One `rebuildFolders()`
closure beside `refreshView`.

---

## 10. Under-documented contentious decisions

REVIEW §8 asks whether a decision a future reader would question has its
reasoning recorded. Most do — the platform layer, the timeout pointer, the
details-pane width coupling, and the frozen window-size work are all model
entries. These four are not.

- **Single-instance falls back to "start anyway."**
  [singleinstance.go:34](../src/ui/singleinstance.go) documents *why* it does
  not abort when port 37653 is held by something that is not GoSentry. It does
  not document the consequence: two GoSentry processes then run two schedulers
  against the same `jobs.json` and the same logs directory, each overwriting the
  other's saves. Combined with 6.2 that is a plausible way to lose the file.
- **The instance channel is an unauthenticated localhost TCP port.** Any local
  process, including one running as another user on a shared machine, can send
  `show`. Low impact — the command only raises a window — but it is a design
  choice, not an accident, and it should say so.
- **The nested `fyne.Do`** (5.4).
- **No catch-up after downtime** (6.8).

---

## 11. Other improvement proposals

- **Startup parses `gosentry.json` twice**, and `PeekKeepRunningInTray`
  ([store.go:22](../src/storage/store.go)) *creates* the file as a side effect
  of a function named "Peek", before `OpenStore` runs. Harmless today; a
  surprising name for a function with a write.
- **`-race` wall time is 4 minutes**, 229 s of it `src/ui`. That is the single
  biggest tax on iteration in this repo and the reason the model
  recommendations below lean toward first-pass correctness.
- **`scripts/test.bat` prints `✓` / `✗`** as UTF-8 in a file `cmd.exe` reads in
  the OEM code page, so the summary lines render as mojibake on a default
  Russian or US console. Use ASCII, or `chcp 65001`.
- **`dist/` in the working tree holds a 1.0.1 binary and 130 sample run logs.**
  Correctly gitignored, so this is only a note: the stale binary next to a 1.0.2
  source tree is an easy thing to hand someone by accident.

---

## Suggested order

Grouped so that each commit is independently reviewable and each medium finding
lands with its regression test.

1. **4.1 — restore `TestJobListViewIsCompact`, retire the other row.** Smallest,
   and it restores an enforcement STANDARDS calls mandatory. Do it first so the
   rest of the work runs against a suite that is honest about itself. TESTS.md
   changes in the same commit: add the restored test back, drop the
   `TestDefaultConfigUsesDetailedJobList` row, add the `notify_timing_test.go`
   table (4.2), and add `PeekKeepRunningInTray` to the deliberate-0% list if
   that is the intent.
2. **6.1 — the Windows quoting bug.** Self-contained, one function, CGO-free
   package, four-row table test already written out above.
3. **6.2 — atomic writes**, plus a `Service.Stop()` on shutdown. Touches one
   helper and one call site; protects everything else.
4. **2.1 — drop the three no-op `SaveJobs` calls**, and retire
   `TestStartRunLockedRollbackOnSaveFailure` with the rollback it guards. Best
   done after 6.2, so the durability question is already settled and this is
   purely a removal.
5. **6.3 — `PendingRuns` lifecycle and cap**, with STANDARDS updated alongside.
6. **3.1 — cap History and stop rescanning column widths.** The biggest
   behavioural win; needs a before/after measurement, and it is in `ui`, so it
   is the item with the slowest feedback loop.
7. **3.2, 3.3 — I/O off `mu`, StartOnly context.** Related concurrency
   cleanups; one commit each.
8. **8.1 — decide what a zero retention limit means**, and make the code, the
   validator, and STANDARDS agree.
9. **1.1 — typed Service accessors, retire `Store()`.** Mechanical once decided,
   but it touches eight UI sites and is best done when nothing else is in
   flight. Rolls up 1.2, 1.3 and 7.3.
10. **5.1, 5.2 — the Jobs view state extraction**, folded into the ROADMAP
    file-split item rather than done separately. 5.2 is a real defect, so if the
    split slips, fix the selection-by-ID part on its own.
11. **The remaining low items** (2.2, 2.3, 3.4–3.6, 4.3–4.7, 6.4–6.7, 7.1–7.3,
    8.2, 8.3, 9.1–9.4, §10, §11) as a small number of themed cleanup commits.

CHANGELOG entries are needed for 6.1, 6.2, 6.3, 3.1, 3.3, and 8.1 — those change
shipped behavior. The rest is internal.

---

## Which model to use

For running these items in Claude Code. As in the retired test-suite plan, the
deciding factor is **not** task size — it is that the feedback loop is slow: the
`ui` package needs the MSYS2 UCRT64 toolchain with CGO on, and `src/ui` alone
took **229 s** in this review's `go test -race ./...` run — every other package
in the tree finished in under 8 s. A model that gets an edit right on the first
pass is worth more than a faster one that needs a second four-minute build to
discover it was wrong.

| Item | Model | Why |
|---|---|---|
| 1 — restore one test, sync TESTS.md | **Haiku 4.5** (`claude-haiku-4-5`) | The test is recoverable verbatim from `git show 5b0e6fe^:src/domain/config_test.go`, and both judgment calls — that the removal was accidental, and that only one of the two is worth restoring — are already settled in §4.1. What is left is a paste plus four doc-table edits, in `domain`, which runs in ~2 s. Nothing to weigh. |
| 2 — Windows quoting | **Sonnet 5** (`claude-sonnet-5`) | The defect and the four expected outputs are already pinned in this document, so the judgment is made; writing the earliest-match-at-a-boundary scan and its table test is careful execution work. `runner` needs no CGO and its Windows-gated test file runs in seconds. |
| 3 — atomic writes + `Service.Stop()` | **Sonnet 5** | Temp-file-then-rename is a known pattern; the only real decisions (same directory, `Sync` before rename, what to do with a leftover `.tmp`) are stated. The `Stop()` wiring in `run.go` is two lines. |
| 4 — remove the no-op saves | **Opus 5** (`claude-opus-5`) | This one is a judgment call disguised as a deletion. It requires re-deriving, against the current `domain.Job`, that no durable field changes on those paths — and being willing to say "actually one does" instead of deleting the safety net. It also retires an existing regression test, which is the sort of change that should not be made by a model optimising for completing the task. |
| 5 — `PendingRuns` lifecycle and cap | **Opus 5** | Interacting state across pause, disable, drain, and the tick loop, with three existing queue tests that must keep passing and a cap whose value is a design decision, not a lookup. `app` is CGO-free, but the reasoning is the cost here, not the build. |
| 6 — History cap + incremental column widths | **Opus 5** | The item with the worst feedback loop (in `ui`, 229 s per attempt) and the one where a plausible-looking fix can be wrong: widths must never shrink below what is on screen, and the cap interacts with the sort toggle and the cached `rows` snapshot that `TestHistorySortToggleKeepsRowsInSync` exists to protect. Fast mode (`/fast`) is worth enabling here specifically, since the wait is real. |
| 7 — I/O off `mu`, StartOnly context | **Opus 5** | Lock-scope changes are exactly where a confident-but-wrong edit is expensive: moving `CleanupLogs` out from under `mu` must not move the snapshot reads with it. The StartOnly half requires knowing why `exec.CommandContext` keeps a goroutine alive without `Wait` — reasoning about the standard library's internals, not about this repo. |
| 8 — zero retention limits | **Sonnet 5** | Once the direction is chosen (accept 0 as unlimited, per §8.1), the change is a validator branch, a load branch, a STANDARDS entry, and two tests, all in CGO-free packages. If the decision goes the other way — deleting the branch and its test — it is smaller still. |
| 9 — typed Service accessors | **Sonnet 5** | Eight mechanical call-site conversions plus two new accessors. The design is settled in §1.1; the work is breadth, not depth. Half the sites are in `ui`, so budget one slow verification run rather than several. |
| 10 — Jobs view state extraction | **Opus 5** | The ROADMAP already says why: a split reads as pure movement while quietly dropping a function, and this one has to break up a constructor rather than move whole functions. The selection-by-ID defect has to survive the move as a fix, not be re-introduced by it. |
| 11 — the low-severity cleanups | **Sonnet 5**, or **Haiku 4.5** for the doc-only ones | Each is small and independently verifiable. Group the CGO-free ones (`domain`, `storage`, `runner`, `app`) into one pass and the `ui` ones into another, so the 229 s build is paid once rather than per item. |

Two notes on this table:

- **Sonnet 5 is the reasonable single choice** if you would rather not switch
  models per item: items 4–7 and 10 are the only ones that really reward the
  step up, and of those only 6 and 7 are likely to go wrong quietly. Sonnet 5's
  introductory pricing runs through **2026-08-31** ($2/$10 per MTok vs $3/$15
  after), against Opus 5's $5/$25.
- **Fast mode is available on Opus 5** (toggle with `/fast`). It is the same
  model with higher output throughput, not a downgrade, but it bills at $10/$50,
  so it only pays for itself when you are actually waiting on output. On this
  plan that is item 6 — and, if you batch them, the `ui` half of item 11.
