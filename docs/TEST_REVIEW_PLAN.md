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

- [ ] `TestCleanupLogsKeepsFilesWithinAgeLimit`
      ([cleanup_test.go:53](../src/runner/cleanup_test.go)) — delete.
      `TestCleanupLogsRemovesFilesPastMaxAge` already asserts that the file
      inside the age limit survives.
- [ ] `TestRunDueEmptyOverlapInheritsGlobal`
      ([run_test.go:457](../src/app/run_test.go)) — delete. It builds the same
      service as `TestRunDueQueueRerunsAfterFinish` (parallel mode, global
      `queue`, a job with an empty `OverlapPolicy`) and asserts strictly less.
      Before deleting, move its one unique line — the setup guard
      `svc.jobs[0].OverlapPolicy != ""` — into `TestRunDueQueueRerunsAfterFinish`,
      so that test still states out loud that it is exercising the inherited
      policy rather than an explicit one.
- [ ] `TestSameWindowsPathHandlesSpaces`
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

- [ ] [TESTS.md](TESTS.md) claims `TestLoadOrCreateConfigCreatesDefaultsOnFirstRun`
      verifies that a missing config file is created "with sane defaults **and a
      sample job**". The test never touches jobs, and `storage.defaultJobs` sits
      at 0% coverage. Decide which half is wrong: either drop the claim from the
      table, or add the assertion that the seeded `jobs.json` contains the
      sample jobs. Adding the assertion is the better outcome — `defaultJobs` is
      the only accidental coverage gap the review found.
- [ ] [TESTS.md](TESTS.md) does not list
      `TestCancelRowOverlapAddsBackOneInnerPadding`
      ([layout_test.go:35](../src/ui/layout_test.go)). Add it to the
      `src/ui/layout_test.go` table.

## 3. Replace the hand-rolled helper in test code

- [ ] [seed_test.go:34](../src/runner/seed_test.go) defines `itoa`: 18 lines of
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
