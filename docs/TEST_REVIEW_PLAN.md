# Test-suite review — action plan

Working document for the findings of the 2026-08-04 review of the test suite.
It is not part of the permanent doc set: delete it once every item below is
either done or moved to [ROADMAP.md](ROADMAP.md).

The rules the findings were judged against live in [STANDARDS.md](STANDARDS.md);
the suite itself is described in [TESTS.md](TESTS.md).

## Baseline the review started from

- 172 tests, 4693 lines of test code.
- 84.4% statement coverage across `domain`, `storage`, `runner`, `scheduler`,
  and `app` measured together with `-coverpkg` (per-package figures understate
  it, because e.g. `domain.NewRuntime` is exercised from the `app` tests).
- [TESTS.md](TESTS.md) documents 171 of the 172 tests.

Overall finding: the suite is **not** padded. Every test but one carries a real
assertion, and most record the property they pin. The items below are the
exceptions.

Method note: redundancy was not judged by reading. Each suspected pair was run
in isolation with `-coverprofile` and the profiles compared. "Identical
coverage" below means the two profiles were byte-identical after sorting.
Identical coverage alone is *not* grounds for deletion — several kept tests hit
the same statements while asserting genuinely different properties. Deletion
requires identical coverage **and** assertions that are a subset.

## 1. Delete the measured duplicates

Each of these has a byte-identical coverage profile with an existing test whose
assertions are a superset. Roughly 30 lines total.

- [x] `TestCleanupLogsKeepsFilesWithinAgeLimit`
      ([cleanup_test.go:53](../src/runner/cleanup_test.go)) — delete.
      `TestCleanupLogsRemovesFilesPastMaxAge` already asserts that the file
      inside the age limit survives.
- [x] `TestRunDueEmptyOverlapInheritsGlobal`
      ([run_test.go:457](../src/app/run_test.go)) — delete. It builds the same
      service as `TestRunDueQueueRerunsAfterFinish` (parallel mode, global
      `queue`, a job with an empty `OverlapPolicy`) and asserts strictly less.
      Before deleting, move its one unique line — the setup guard
      `svc.jobs[0].OverlapPolicy != ""` — into `TestRunDueQueueRerunsAfterFinish`,
      so that test still states out loud that it is exercising the inherited
      policy rather than an explicit one.
- [x] `TestSameWindowsPathHandlesSpaces`
      ([autostart_windows_test.go:20](../src/platform/autostart/autostart_windows_test.go))
      — delete. It is the same case as `TestSameWindowsPathIgnoresCaseAndQuotes`
      (quoted path, mixed case); `sameWindowsPath` does not split on spaces, so
      the space in the fixture reaches no new code. If the spaces case is worth
      naming, fold the path into the surviving test's fixture instead.

After the deletions, re-run the affected packages and confirm coverage is
unchanged:

```bash
go test -coverpkg=./src/domain,./src/storage,./src/runner,./src/scheduler,./src/app ./src/domain ./src/storage ./src/runner ./src/scheduler ./src/app
```

## 2. Fix the documentation drift

- [x] [TESTS.md](TESTS.md) claims `TestLoadOrCreateConfigCreatesDefaultsOnFirstRun`
      verifies that a missing config file is created "with sane defaults **and a
      sample job**". The test never touches jobs, and `storage.defaultJobs` sits
      at 0% coverage. Decide which half is wrong: either drop the claim from the
      table, or add the assertion that the seeded `jobs.json` contains the
      sample jobs. Adding the assertion is the better outcome — `defaultJobs` is
      the only accidental coverage gap the review found.
- [x] [TESTS.md](TESTS.md) does not list
      `TestCancelRowOverlapAddsBackOneInnerPadding`
      ([layout_test.go:35](../src/ui/layout_test.go)). Add it to the
      `src/ui/layout_test.go` table.

## 3. Replace the hand-rolled helper in test code

- [x] [seed_test.go:34](../src/runner/seed_test.go) defines `itoa`: 18 lines of
      digit-by-digit conversion with a fresh allocation per digit, in a file
      that already imports `strconv`. Replace the calls with
      `strconv.FormatInt` and delete the helper. Untested logic inside a test
      file is exactly what produces a test result nobody can trust.

## 4. Thin tests — decide, then act

None of these is wrong; each is close enough to worthless that it should be
either justified or removed. Grouped because they want one decision, not four.

- [ ] `TestEmitWithNoObserversIsNoop`
      ([events_test.go:36](../src/app/events_test.go)) — the only test in the
      suite with no assertion at all. Ranging over a nil slice cannot panic in
      Go, so it pins nothing. Delete.
- [ ] `TestStoreReturnsWiredStore`
      ([service_test.go:51](../src/app/service_test.go)) — asserts that a
      one-line getter returns its own field. Delete.
- [ ] `TestMainViewBuilds`
      ([mainwindow_test.go:72](../src/ui/mainwindow_test.go)) — a smoke test;
      `TestMainViewFitsTheDefaultWindowSize` builds the same view. Its only
      unique coverage is `w.SetContent(content)` and `recordStartup(0, true)`.
      Either fold those two calls into the sizing test and delete this one, or
      keep it and say in its comment that `recordStartup` is what it is for.
- [ ] `TestFilteredJobIndexesAll` / `ByNamedFolder` / `NoFolder` / `EmptySlice`
      ([jobs_view_test.go:59-101](../src/ui/jobs_view_test.go)) — four tests
      over one small pure function. Collapse into one table-driven test in the
      style of `TestFilterValue` directly above them; the `EmptySlice` case
      becomes one row rather than a function.

## 5. Runtime cost of the runner tests (optional)

`TestRunJobLogFileAllHeaders`, `TestRunJobRecordFields`, and
`TestRunJobWritesLogFile` ([runner_test.go](../src/runner/runner_test.go)) have
identical coverage profiles but assert three genuinely different things — log
headers, `RunRecord` field values, and the log file's name and directory. They
are **not** duplicates and should not be deleted on that basis.

The cost is that each spawns a real subprocess; the `runner` package takes 5.3 s.
If suite wall time becomes a concern, merge them into one `RunJob` call with
three assertion blocks. Until then, leave them alone.

## Explicitly not changing

Recorded here so a later pass does not re-report them:

- `TestSetGlobalPausePersistsToConfigFile` has the same coverage as
  `TestSetGlobalPauseUpdatesRuntimesAndEmits` but asserts a different property —
  that the flag reaches `gosentry.json`, which is what makes the pause survive a
  restart.
- `TestRunDueQueueDrainsMultipleOverlaps` has the same coverage as
  `TestRunDueQueueRerunsAfterFinish`, but drains three queued occurrences rather
  than one. It is the test that would catch a drain loop that fires once.
- `TestCreateStartupShortcutHandlesCyrillicPath` and `...HandlesSpaces` cover the
  same statements, but non-ASCII paths and paths with spaces are different
  real-world failure modes for the WScript.Shell COM call.
- The 0% functions in the non-UI packages are deliberate: the real `Clock` (a
  fake is injected everywhere), `OpenStore` / `ResolvePaths` / `Service.Start` /
  `Service.Open` and the autostart and desktop-icon wrappers (process entry
  points and OS integration), and `ShouldNotifyOnFailure` (a getter under the
  mutex). `storage.defaultJobs` is the exception — see item 2.

## Suggested order

1. Item 2 (docs) — smallest, and the `defaultJobs` assertion is the only one
   that adds coverage.
2. Item 3 (`itoa`) — independent of everything else.
3. Item 1 (deletions) — one commit, with the coverage re-run as evidence.
4. Item 4 (thin tests) — needs a judgment call per test.
5. Item 5 — only if suite wall time becomes a problem.

Items 1 and 4 change the test inventory, so [TESTS.md](TESTS.md) has to be
updated in the same commit. No [CHANGELOG.md](CHANGELOG.md) entry is needed:
none of this changes shipped behavior.

## Which model to use

For running these items in Claude Code. The deciding factor here is not task
size — it is that **the feedback loop is slow**: the `ui` package needs the
MSYS2 UCRT64 toolchain with CGO on, and a cold `go test ./src/ui/...` took
**258 s** during the review. A model that gets an edit right on the first pass
is worth more than a faster one that needs a second build to find out.

| Item | Model | Why |
|---|---|---|
| 3 — `itoa` → `strconv.FormatInt` | **Haiku 4.5** (`claude-haiku-4-5`) | A mechanical substitution in one file, in the `runner` package, which needs no CGO and runs in ~5 s. Nothing to weigh. |
| 2 — docs, and the `defaultJobs` assertion | **Sonnet 5** (`claude-sonnet-5`) | Two doc edits plus one new assertion in `storage`. Reading `loadOrCreateJobs` to write the assertion is real work, but the answer is not in doubt. No CGO. |
| 1 — the three deletions | **Sonnet 5** | Deleting is easy; the judgment is narrow and already made in this document (which line to carry over from `TestRunDueEmptyOverlapInheritsGlobal`, and that identical coverage must be re-verified afterwards). One of the three is in `platform/autostart`, which is Windows-gated but CGO-free. |
| 4 — the four thin tests | **Opus 5** (`claude-opus-5`) | This is the only item that is genuinely a judgment call rather than an execution task: whether each test should exist at all, and — for `TestMainViewBuilds` — whether to fold two calls into the sizing test or keep it with a better comment. Two of the four are in `ui`, so a wrong call costs a 4-minute rebuild to discover. |
| 5 — merging the runner tests | **Opus 5**, if attempted | It requires holding three distinct sets of assertions and confirming none is silently dropped in the merge. It is also the item most likely to be *not worth doing* — a model that will say so is the point. |

Two notes on this table:

- **Sonnet 5 is the reasonable single choice** if you would rather not switch
  models per item. It is near-Opus on coding and agentic work, and only item 4
  really rewards the step up. The introductory pricing through **2026-08-31**
  ($2/$10 per MTok vs $3/$15) makes it cheaper than usual relative to Opus 5's
  $5/$25.
- **Fast mode is available on Opus 5** (toggle with `/fast`). It is the same
  model with higher output throughput, not a downgrade — but it bills at
  $10/$50, so it only pays for itself when you are waiting on the output. Given
  that the actual wait here is the Fyne build rather than token generation, it
  is unlikely to help on this plan.
