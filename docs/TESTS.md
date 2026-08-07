# GoSentry Test Suite

All tests are located alongside source code in their respective packages under `src/`. Tests follow Go conventions with `*_test.go` filename patterns.

## Running Tests

### Using the test scripts

The repository provides convenience scripts to run all tests with static analysis:

**Unix/Linux/macOS:**
```bash
./scripts/test.sh
```

**Windows:**
```bash
scripts\test.bat
```

Both scripts run:
1. `go vet ./...` — static analysis for common errors and suspicious code patterns
2. `go test -race ./...` — tests with race condition detection enabled

The GUI tests build the Fyne desktop backend, so CGO must be enabled; on Windows
that means the MSYS2 UCRT64 toolchain described in
[DEVELOPMENT.md](DEVELOPMENT.md).

`src/ui` dominates `go test -race ./...`'s wall time — around 229s in the
2026-08-05 whole-project review, against under 8s for every other package
combined. Budget iteration accordingly: a change confined to `domain`,
`storage`, `runner`, `scheduler`, or `app` gets a fast feedback loop; a `ui`
change does not.

### Manual test commands

Run all tests:
```bash
go test ./...
```

Run all tests with race detection:
```bash
go test -race ./...
```

Run tests with verbose output:
```bash
go test -v ./...
```

Run a specific test by name:
```bash
go test -run TestRunJobWritesLogFile ./src/runner
```

Run tests with code coverage:
```bash
go test -cover ./src/runner
go test -coverprofile=coverage.out ./src/runner
go tool cover -html=coverage.out
```

Per-package coverage understates the suite, because several packages are
exercised from another one's tests — `domain.NewRuntime`, for instance, is
covered by the `app` tests. Measure the engine packages together instead:

```bash
go test -coverpkg=./src/domain,./src/storage,./src/runner,./src/scheduler,./src/app ./src/domain ./src/storage ./src/runner ./src/scheduler ./src/app
```

In the PowerShell environment DEVELOPMENT.md prescribes on Windows, PowerShell
splits the comma-separated `-coverpkg` list on its own and the command fails
with `directory not found`. Use the stop-parsing token, or quote the whole
flag:

```powershell
go test --% -coverpkg=./src/domain,./src/storage,./src/runner,./src/scheduler,./src/app ./src/domain ./src/storage ./src/runner ./src/scheduler ./src/app
```

That figure was 84.4% at the 2026-08-04 review, which is the number to compare
against before concluding that coverage has slipped.

---

## Test Files Overview

### src/domain/schedule_test.go

**Package:** `domain`

Tests schedule parsing and validation.

| Test | Purpose |
|------|---------|
| `TestParseRejectsInvalidSchedules` | Verifies that invalid schedule strings return an error. |
| `TestParseEveryInterval` | Verifies `@every` duration syntax (e.g., `@every 10s`) is parsed and computes the correct next run time. |
| `TestParseEveryTrimsSurroundingWhitespace` | Verifies leading/trailing whitespace around the `@every` spec is ignored. |
| `TestParseCronExpression` | Verifies 5-field cron expressions (e.g., `*/5 * * * *`) are parsed and compute the correct next run time. |
| `TestParseCronDescriptor` | Verifies predefined cron descriptors such as `@hourly` are accepted. |
| `TestValidateAcceptsValidSchedules` | Verifies that `Schedule.Validate` returns nil for valid schedule strings. |
| `TestZeroScheduleNextIsZero` | Verifies that a zero-value Schedule returns a zero time from `Next`. |
| `TestStringReturnsTrimmedSpec` | Verifies that `Schedule.String` returns the trimmed schedule spec. |

---

### src/domain/config_test.go

**Package:** `domain`

Tests autostart argument helpers and the jobs-list density normalization rule.

| Test | Purpose |
|------|---------|
| `TestAutostartArguments` | Verifies `AutostartArguments` returns `--start-in-tray` when the tray is enabled and an empty string when it is off. |
| `TestResolveStartHidden` | Verifies hidden autostart requires both the CLI flag and `KeepRunningInTray`. |
| `TestJobListViewIsCompact` | Verifies only the exact `"compact"` value selects one-line rows: empty, differently-cased, and unrecognised values all read as detailed. |

---

### src/app/service_test.go

**Package:** `app`

Tests `Service` construction and the state-accessor contract.

| Test | Purpose |
|------|---------|
| `TestNewServiceBuildsRuntimePerJob` | Verifies that `NewService` creates a `JobRuntime` entry for every loaded job. |
| `TestJobsReturnsCopy` | Verifies that `Service.Jobs` returns a defensive copy so callers cannot mutate internal state. |

---

### src/app/operations_test.go

**Package:** `app`

Tests all mutating operations on the Service, scheduler integration, and settings persistence.

#### Job CRUD

| Test | Purpose |
|------|---------|
| `TestCreateJobAssignsIDAndEmits` | Verifies that `CreateJob` assigns a unique ID, persists to JSON, and emits `JobChanged`. |
| `TestCreateJobValidates` | Verifies that `CreateJob` rejects jobs with an invalid schedule. |
| `TestUpdateJobKeepsRuntimeAndReflectsDisable` | Verifies that `UpdateJob` preserves existing runtime state and disables a job correctly. |
| `TestUpdateJobReenablesPausedJob` | Verifies that re-enabling a previously-disabled job clears the paused runtime state. |
| `TestRuntimeLazilyRecreated` | Verifies that `UpdateJob` recreates a missing runtime entry rather than panicking. |
| `TestUpdateJobNotFound` | Verifies that `UpdateJob` returns an error for an unknown job ID. |
| `TestDeleteJobRemovesEverything` | Verifies that `DeleteJob` removes the job from the slice, the runtime map, and the schedule cache. |
| `TestDeleteJobNotFound` | Verifies that `DeleteJob` returns an error for an unknown job ID. |
| `TestSetEnabledNotFound` | Verifies that `SetEnabled` returns an error for an unknown job ID. |
| `TestSetEnabledToggles` | Verifies that `SetEnabled` flips the enabled flag and persists the change. |
| `TestSetEnabledClearsPendingRuns` | Verifies that disabling a job zeroes a `PendingRuns` backlog it was carrying, so re-enabling it later does not replay a stale deferred run. |

#### Global pause / run-now / run-due

| Test | Purpose |
|------|---------|
| `TestSetGlobalPauseUpdatesRuntimesAndEmits` | Verifies that `SetGlobalPause` updates all job runtimes, emits `SchedulerStateChanged`, and persists state. |
| `TestSetGlobalPausePersistsToConfigFile` | Verifies the paused flag reaches `gosentry.json`, which is what makes the pause survive a restart. |
| `TestServiceRebuiltFromPausedStoreStartsPaused` | Verifies a Service built from a paused config starts paused, with the paused next-run text applied before the first tick. |
| `TestRunNowUsesRunnerAndRecords` | Verifies that `RunNow` invokes the runner, records a `RunRecord`, and emits `RunRecorded`. |
| `TestRunNowNotFound` | Verifies that `RunNow` returns an error for an unknown job ID. |
| `TestRunNowRefusedWhileAlreadyRunning` | Verifies that a second concurrent `RunNow` on the same job is rejected while the first is in progress. |
| `TestRunNowAllowedWhilePaused` | Verifies that `RunNow` is allowed when the global pause flag is set (pause stops scheduled runs only). |
| `TestRunDueStartsDueJob` | Verifies that `RunDue` launches a job whose next-run time has passed. |
| `TestRunDueSkipsJobNotYetDue` | Verifies that `RunDue` does not launch a job that is not yet due. |
| `TestRunDueSkipsJobInRunningState` | Verifies that `RunDue` does not start a second concurrent run for an already-running job, even with a stale `NextDue` in the past. |
| `TestRunDueDoesNothingWhilePaused` | Verifies that `RunDue` launches nothing when the global pause flag is set. |
| `TestStartDrivesRunDueOnTick` | Verifies that `Service.Start` wires `RunDue` to the scheduler tick and that each tick advances state. |

#### Settings

| Test | Purpose |
|------|---------|
| `TestUpdateSettingsPersistsAndValidates` | Verifies that `UpdateSettings` persists a valid config and rewrites autostart if needed. |
| `TestUpdateSettingsRejectsInvalidConfigs` | Verifies that `UpdateSettings` returns validation errors without persisting. |
| `TestHasFileName` | Verifies the jobs-file path check: a file name passes; a trailing separator, `.`, and `..` do not. |
| `TestUpdateSettingsWritesJobsToTheNewFile` | Verifies that changing `JobsFile` re-resolves `Paths.JobsPath` and writes the loaded jobs to the new file, creating its folder. |
| `TestUpdateSettingsAdoptsExistingJobsFile` | Verifies that selecting a jobs file that already exists replaces the job list with its contents, rebuilds runtimes, and emits `JobsLoaded`. |
| `TestUpdateSettingsKeepsJobsWhenTheNewFileIsMissing` | Verifies that a path with no file behind it receives the current jobs instead (the rename/relocate case). |
| `TestUpdateSettingsRefusesJobsFileSwitchWhileRunning` | Verifies that switching the jobs file is refused (and not persisted) while a job runs, while unrelated settings still save. |
| `TestUpdateSettingsSeedsAdoptedJobsFromLogs` | Verifies that statistics reconstructed from the new logs directory still reach the runtime map, now that the log scan happens before `UpdateSettings` takes `mu`. |
| `TestConcurrentJobOperationsLeaveTheFileMatchingMemory` | Verifies that saves prepared under `mu` and run after it is released still land in mutation order, so `jobs.json` matches the in-memory list after concurrent create/disable operations. |
| `TestSetJobListViewPersistsToConfigFile` | Verifies the Jobs-list density preference reaches `gosentry.json`, so the chosen view reopens after a restart. |
| `TestSetJobListViewNormalizesUnknownValue` | Verifies anything but `"compact"` is stored as `"detailed"`, so the config never gains a value no reader understands. |
| `TestPrependLogCapsActivityList` | Verifies that the activity log never grows beyond its maximum cap. |

---

### src/app/run_test.go

**Package:** `app`

Tests overlap policy, sequential execution, run statistics, timeout resolution,
and scheduler edge cases using injected `runJob` and `primeDue`.

| Test | Purpose |
|------|---------|
| `TestUpdateStats` | Verifies aggregate duration math on `JobRuntime`. |
| `TestUpdateStatsSkipsZeroDuration` | Verifies zero-duration runs are excluded from averages. |
| `TestRunDueParallelStartsAllDueJobs` | Parallel mode: both due jobs enter the runner before either completes. |
| `TestRunDueSequentialSerializes` | Sequential mode: job 2 waits until job 1 finishes. |
| `TestRunDueSkipDropsOverlap` | Global skip: no second concurrent run, `PendingRuns` stays 0. |
| `TestRunDueQueueRerunsAfterFinish` | Queue: one deferred run after an in-flight finish; also covers an empty per-job policy inheriting the global default. |
| `TestRunDueQueueDrainsMultipleOverlaps` | Queue: multiple missed ticks drain as separate runs. |
| `TestRunDueQueueCapsPendingRuns` | Regression: `PendingRuns` stops growing at `maxPendingRuns` instead of accumulating without bound for a job that never keeps up with its schedule. |
| `TestRunDuePerJobQueueOverridesGlobalSkip` | Per-job `queue` beats global `skip`. |
| `TestRunDuePerJobSkipOverridesGlobalQueue` | Per-job `skip` beats global `queue`. |
| `TestRunNowSequentialGuard` | Manual run refused while another job runs in sequential mode. |
| `TestRunDueQueueDrainSkippedWhenPaused` | Queued overlaps are not drained while the scheduler is paused, and pausing clears the backlog rather than leaving it to fire a stale deferred run on resume. |
| `TestEffectiveTimeout` | Verifies the three-state resolution: `nil` inherits the global default, a positive value overrides it, and an explicit `0` means no timeout without inheriting. |

---

### src/app/events_test.go

**Package:** `app`

Tests the event-emission and observer-subscription machinery.

| Test | Purpose |
|------|---------|
| `TestEmitDeliversToAllObserversInOrder` | Verifies that all registered observers receive emitted events in registration order. |
| `TestObserverCanReadServiceState` | Verifies that an observer called by `emit` can safely read Service state (jobs, runtimes). |

---

### src/app/format_test.go

**Package:** `app`

Tests display-formatting helpers used by the UI.

| Test | Purpose |
|------|---------|
| `TestStatusText` | Verifies that job status codes map to the correct display strings. |
| `TestEventText` | Verifies trigger-type labels for scheduled, manual, and UI triggers. |
| `TestEventLine` | Verifies the one-line activity rendering of a `RunRecord`, including the log basename and the `Unknown` fallback for a blank trigger. |
| `TestDisplayFolder` | Verifies that an empty folder string shows "No folder". |
| `TestDisplayArguments` | Verifies that an empty arguments string shows "None". |
| `TestDisplayRunMode` | Verifies run-mode labels for normal and start-only modes. |
| `TestDisplayInvocation` | Verifies that the full invocation display string combines command and arguments with spacing. |
| `TestDisplayIndex` | Verifies the list position of a job index in a filtered index slice. |
| `TestDisplayStats` | Verifies statistics line formatting for the details panel. |
| `TestDisplayOverlapPolicy` | Verifies per-job vs inherited global overlap policy labels. |
| `TestDisplayTimeout` | Verifies the three timeout states read differently in the details panel: `45 s`, `no timeout`, and `… (global default)`. |

---

### src/storage/store_test.go

**Package:** `storage`

Tests JSON round-tripping, default generation, and backward compatibility.

| Test | Purpose |
|------|---------|
| `TestJobsRoundTrip` | Verifies that jobs saved to JSON are reloaded with identical field values. |
| `TestConfigRoundTrip` | Verifies that settings saved to JSON are reloaded with identical field values. |
| `TestNormalizeJobsFillsDefaults` | Verifies that `normalizeJobs` assigns sequential IDs and sets default name, schedule, and command for jobs missing those fields. |
| `TestNormalizeJobsReassignsDuplicateIDs` | Verifies that a hand-edited `jobs.json` with two entries sharing one ID gets the later duplicates reassigned instead of colliding on one runtime. |
| `TestResolveConfiguredPathCleansAbsolutePaths` | Verifies (Windows only) that forward-slash and backslash spellings of the same absolute path resolve to the same string. |
| `TestLoadOrCreateConfigCreatesDefaultsOnFirstRun` | Verifies that a missing config file is created with sane defaults. |
| `TestLoadOrCreateJobsSeedsSampleJobsOnFirstRun` | Verifies that a missing jobs file is created with the sample jobs from `defaultJobs`. |
| `TestLoadOrCreateConfigKeepsZeroTimeoutOnReload` | Verifies that `default_timeout_seconds: 0` survives a reload rather than being normalized away — 0 is a value, not a missing field. |
| `TestLoadOrCreateConfigMigratesJobsDir` | Verifies that a pre-0.15 `jobs_dir` becomes `jobs_file` pointing at the same `jobs.json`, and that the retired key is not written back. |
| `TestLoadOrCreateConfigMigratesLegacyThemeDefault` | Verifies that a config storing the retired `"default"` theme value is normalized to `system` on load. |
| `TestLoadJobsFileReportsMissingWithoutCreating` | Verifies that `LoadJobsFile` reports a missing file as not-found without creating or seeding it, and normalizes the jobs it does load. |
| `TestApplyConfigPathsDerivesJobsDir` | Verifies that the configured jobs file resolves against the program folder and that `Paths.JobsDir` is derived from it. |
| `TestJobTimeoutRoundTripsThreeStates` | Verifies the on-disk encoding that keeps "inherit" and "no timeout" distinguishable: `nil` is omitted entirely, an explicit `0` is written and read back as set. |
| `TestJobsJSONDoesNotPersistRuntimeNoise` | Verifies that `jobs.json` does not persist runtime state (LastRun, NextRun, etc.). Only durable job fields are stored. |

---

### src/scheduler/scheduler_test.go

**Package:** `scheduler`

Tests the timing-loop contract using a fake clock.

| Test | Purpose |
|------|---------|
| `TestSchedulerCallsTickWithClockNow` | Verifies that the scheduler calls the injected tick function with the wall-clock time returned by the fake Clock. |
| `TestSchedulerStopReleasesClock` | Verifies that `Stop` terminates the tick loop and releases the Clock without hanging. |

---

### src/runner/runner_test.go

**Package:** `runner`

Tests command execution, exit code handling, output capture, and the run timeout.

#### Log file tests

| Test | Purpose |
|------|---------|
| `TestRunJobLogFileAllHeaders` | Verifies that the log file contains all expected metadata headers: trigger type, job name, command, arguments, and start time. |
| `TestRunJobRecordFields` | Verifies that the returned `RunRecord` carries the correct status, trigger, and log-file path. |
| `TestRunJobWritesLogFile` | Verifies that each job execution creates a `.log` file in the configured logs directory with a sanitized job name in the filename. |

#### Output formatting

| Test | Purpose |
|------|---------|
| `TestFormatOutput` | Verifies that stdout and stderr are combined with section labels in the formatted output. |
| `TestFormatOutputEmptyStreams` | Verifies that empty stdout/stderr streams are omitted from the formatted output. |
| `TestLogArguments` | Verifies that arguments are included in the log header and absent when the arguments field is empty. |
| `TestSanitizeFileName` | Verifies that spaces and special characters in a job name are replaced to produce a safe filename segment. |

#### Command execution

| Test | Platform | Purpose |
|------|----------|---------|
| `TestRunJobRunsQuotedWindowsExecutable` | Windows | Verifies that executable paths with quotes are executed correctly via `cmd.exe`. |
| `TestRunJobRunsUnquotedWindowsProgramPathWithSpaces` | Windows | Verifies that unquoted executable paths with spaces are quoted and executed correctly. |
| `TestRunJobRunsWindowsCommandWithSeparateArguments` | Windows | Verifies that command and arguments from the Job struct are combined and executed correctly. |

#### Exit code handling

| Test | Purpose |
|------|---------|
| `TestRunJobFailsOnNonZeroExitCode` | Verifies that a nonzero process exit code results in "Failed" status with an "exit code N" detail. |

#### Timeout

| Test | Purpose |
|------|---------|
| `TestRunJobTimesOut` | Verifies that a positive timeout kills a long-running command and reports `Timed out after <timeout>`. |
| `TestRunJobZeroTimeoutMeansNoTimeout` | Verifies that a non-positive duration runs without a deadline, bounded only by the caller's context. |
| `TestRunJobStartOnlyIgnoresTimeout` | Verifies that fire-and-forget jobs run on the untimed context, so the timeout never kills a process the runner is not waiting for. |

#### Start-only mode

| Test | Purpose |
|------|---------|
| `TestRunJobStartOnlyDoesNotWaitForExitCode` | Verifies that `StartOnly: true` jobs launch and return "OK" immediately without waiting for the process to exit. |
| `TestRunJobStartOnlyReportsStartFailure` | Verifies that `StartOnly: true` jobs still report "Failed" if the process cannot be started. |
| `TestRunJobStartOnlyLeavesNoContextWatcher` | Verifies that a start-only run leaves no `os/exec` context-watcher goroutine behind, since it never calls `Wait` and the started process is meant to outlive the app. |

---

### src/runner/runner_windows_test.go

**Location:** `src/runner/runner_windows_test.go`
**Build Tags:** `//go:build windows`

Tests the Windows shell invocation and hidden-window flags.

| Test | Purpose |
|------|---------|
| `TestDirectCommandDoesNotHideWindow` | Verifies that direct executable commands do not request hidden-window startup. |
| `TestShellCommandHidesWindow` | Verifies that shell commands request hidden-window startup to prevent console flash. |
| `TestShellCommandUsesWindowsSafeQuoting` | Verifies `cmd.exe /S /C` quoting for paths with spaces and special characters. |
| `TestWindowsShellCommandLineQuotesUnquotedProgramPath` | Verifies that unquoted program paths in shell commands are quoted while preserving already-quoted arguments. |

---

### src/runner/seed_test.go

**Package:** `runner`

Tests `SeedStats`, which rebuilds aggregate run statistics from the `.log` files
on disk at startup.

| Test | Purpose |
|------|---------|
| `TestSeedStatsBasic` | Verifies run/fail counts and the last, average, and maximum durations parsed from a job's log headers. |
| `TestSeedStatsDurationLessLegacyLog` | Verifies a log written before the `duration` header still counts as a run but is excluded from the duration aggregates, so a missing duration cannot masquerade as a 0 ms run. |
| `TestSeedStatsMaxFilesHonoured` | Verifies that only the newest `MaxLogFiles` logs are parsed when the limit is positive. |
| `TestSeedStatsMissingDir` | Verifies a missing logs directory yields an empty map rather than an error or a panic. |
| `TestSeedStatsUnknownJobProducesNoEntry` | Verifies log files that match no known job are ignored. |
| `TestSeedStatsMatchesByJobID` | Verifies logs are matched by the `job_id` header even when two job names sanitize to the same filename. |

---

### src/runner/cleanup_test.go

**Package:** `runner`

Tests log-file cleanup by age and by count.

| Test | Purpose |
|------|---------|
| `TestCleanupLogsMissingDirReturnsNil` | Verifies that cleanup returns nil (not an error) when the logs directory does not exist. |
| `TestCleanupLogsRemovesFilesPastMaxAge` | Verifies that `.log` files older than `MaxLogAgeDays` are deleted and files within the limit are retained. |
| `TestCleanupLogsByCountDeletesOldest` | Verifies that when file count exceeds `MaxLogFiles`, the oldest files are removed first. |
| `TestCleanupLogsNonLogFilesNotDeleted` | Verifies that non-`.log` files in the logs directory are never deleted by cleanup. |
| `TestCleanupLogsSubdirsNotDeleted` | Verifies that subdirectories inside the logs directory are not deleted by cleanup. |
| `TestCleanupLogsZeroLimitsDisableBothPolicies` | Verifies that setting both limits to zero disables both the age and count cleanup policies. |

---

### src/runner/logfile_test.go

**Package:** `runner`

Tests the disambiguating suffix `writeRunLog` applies when two runs land on
the same second.

| Test | Purpose |
|------|---------|
| `TestUniqueLogPathAvoidsCollision` | Verifies repeated calls for the same file name return distinct paths instead of silently overwriting an existing log. |

---

### src/platform/autostart/autostart_windows_test.go

**Location:** `src/platform/autostart/autostart_windows_test.go`
**Build Tags:** `//go:build windows`

Tests Windows autostart via shortcuts in the Startup folder.

| Test | Purpose |
|------|---------|
| `TestSameWindowsPathIgnoresCaseAndQuotes` | Verifies that Windows path comparison is case-insensitive, handles quote marks, and matches paths containing spaces. |
| `TestSameWindowsPathStripsExtendedLengthPrefix` | Verifies that `\\?\`-prefixed paths are compared correctly after stripping the prefix. |
| `TestSameWindowsPathMatchesShortNameViaFilesystem` | Verifies that 8.3 short names are resolved to long names for comparison. |
| `TestStartupShortcutPathUsesUserStartupFolder` | Verifies that the shortcut path resolves into `%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup`. |
| `TestCreateStartupShortcutHandlesCyrillicPath` | Verifies that `.lnk` files are created correctly when the executable path contains Cyrillic characters. |
| `TestCreateStartupShortcutHandlesSpaces` | Verifies that `.lnk` files are created with correct `TargetPath` and `--start-in-tray` arguments when the path contains spaces. |
| `TestCreateStartupShortcutWithoutTrayFlag` | Verifies that autostart shortcuts omit `--start-in-tray` when the tray setting is off. |
| `TestAutostartStatusRequiresMatchingTrayFlag` | Verifies `AutostartStatus` reports a problem when the shortcut arguments do not match `KeepRunningInTray`. |

---

### src/platform/autostart/autostart_linux_test.go

**Location:** `src/platform/autostart/autostart_linux_test.go`
**Build Tags:** `//go:build linux`

Tests Linux autostart via XDG Desktop Entry files.

| Test | Purpose |
|------|---------|
| `TestLinuxAutostartStartsInTray` | Verifies that the XDG Desktop Entry is created with `--start-in-tray` in the `Exec=` field. |
| `TestLinuxAutostartWithoutTrayFlag` | Verifies that the desktop entry omits `--start-in-tray` when the tray setting is off. |

---

### src/ui/tray_test.go

**Package:** `ui`

Tests startup helpers for tray and autostart interaction.

| Test | Purpose |
|------|---------|
| `TestResolveStartHiddenUsesDomainHelper` | Verifies the UI startup helper stays aligned with `domain.ResolveStartHidden`. |

---

### src/platform/desktop/desktop_linux_test.go

**Location:** `src/platform/desktop/desktop_linux_test.go`
**Build Tags:** `//go:build linux`

Tests Linux desktop integration (`.desktop` file and icon under XDG data home).

| Test | Purpose |
|------|---------|
| `TestInstallDesktopIntegrationWritesDesktopAndIcon` | Verifies `.desktop` and PNG icon files are written under `$XDG_DATA_HOME`. |
| `TestQuoteDesktopExecQuotesPath` | Verifies `Exec=` paths with spaces are shell-quoted. |

---

### src/platform/filemanager/filemanager_test.go

**Package:** `filemanager`

Tests the guards around opening a folder in the desktop file manager. The
success path is not tested: it would open a real file manager window.

| Test | Purpose |
|------|---------|
| `TestOpenRejectsMissingFolder` | Verifies that `Open` reports a missing directory (naming the path) instead of launching a handler. |
| `TestOpenRejectsFile` | Verifies that `Open` refuses a path that is a file rather than a directory. |
| `TestOpenCommandNamesPlatformHandler` | Verifies the per-platform handler (`explorer` / `xdg-open`, none elsewhere) and that the path is passed as one argument. |

---

### src/ui/jobs_view_test.go

**Package:** `ui`

Tests the Jobs tab: pure filter helpers, and — through Fyne's headless
`test.NewApp()` — the geometry and redraw behaviour that only shows up once the
widgets are assembled.

| Test | Purpose |
|------|---------|
| `TestFilterValue` | Verifies that `filterValue` returns the correct display string for the current folder filter. |
| `TestFolderOptionsAlwaysIncludesSentinels` | Verifies that the folder filter list always starts with "All" and "No folder" sentinel entries. |
| `TestFolderOptionsAppendsUniqueFolders` | Verifies that folder names from the job list are appended once each, in order, without duplicates. |
| `TestFilteredJobIndexes` | Table: verifies the "All" filter returns every index, a named folder returns only its own jobs, "No folder" matches empty and blank folder fields, and an empty job list yields no indexes. |
| `TestNextJobListViewFlipsBothWays` | Verifies the density toggle alternates between detailed and compact from either starting value. |
| `TestViewToggleTextNamesTheAction` | Verifies the toggle button is labelled with the action it performs, not the state it is in. |
| `TestJobListViewToggleShrinksRowsAndPersists` | End-to-end: one tap shrinks the row height, relabels the button, and reaches the config; tapping back undoes all three. |
| `TestJobListViewCompactConfigOpensCompact` | Verifies the persisted density is honoured at build time, not only after a tap. |
| `TestJobsSidebarWidthIsItsContent` | Regression guard: nothing but the sidebar's own toolbar row imposes a width floor on it. |
| `TestJobsSplitOpensAtTheSidebarWidth` | Verifies the derived split offset opens the divider at the sidebar's own width at the default window size — enough that the toolbar is never born clipped, and no more. |
| `TestToolbarButtonRedrawsRowAndDetails` | Regression guard: with the duplicate refreshes removed from the handlers, `jobsView.refresh` alone must re-snapshot the jobs and repopulate the details pane. |
| `TestJobsViewSelectionSurvivesAJobsFileSwitch` | Regression guard: adopting a different jobs file replaces the whole list from the Service, and the refresh that follows must leave the details pane and the list highlight describing the same job — not redraw the pane from a row index that belonged to the previous list. |
| `TestDetailCaptionWidthCoversEveryCaption` | Verifies every caption `metadataRows` returns fits the measured caption column, which is what makes the single row list self-enforcing. |

---

### src/ui/jobs_view_state_test.go

**Package:** `ui`

Tests `jobsViewState`, the Jobs tab's model: the job/runtime snapshot, the
folder filter, and the ID-based selection. No Fyne app is built — the state
touches no widgets, so these run in milliseconds.

| Test | Purpose |
|------|---------|
| `TestJobsViewStateSelectsTheFirstJob` | Verifies the opening state selects the first row, so the details pane is never blank when there is something to show. |
| `TestJobsViewStateEmptyListSelectsNothing` | Verifies an empty job list leaves nothing selected and no row to highlight (`displayRow` = -1). |
| `TestJobsViewStateSelectionFollowsTheJobNotTheRow` | Regression guard: a job removed above the selected one (through the Service, the way an external change reaches the view) must not slide the selection onto its neighbour — the selection is a job ID, and only its row moves. |
| `TestJobsViewStateDropsSelectionWhenItsJobIsGone` | Verifies a selection whose job no longer exists falls back to the first visible row instead of describing whichever job inherited its position. |
| `TestJobsViewStateApplyFilter` | Verifies the folder filter keeps a selection it still shows, moves it to the folder's first row when it does not, and that "No folder" matches the job without one. |
| `TestJobsViewStateEmptyFilterSelectsNothing` | Verifies a filter matching no job is a filter choice, not an error state: nothing selected, nothing highlighted, and the selection returns when the filter is cleared. |
| `TestJobsViewStateHiddenSelectionIsNotHighlighted` | Verifies a selected job the filter hides reports no display row rather than falling back to row 0, which would highlight an unrelated job. |
| `TestJobsViewStateRuntimeIsNeverNil` | Verifies `runtime` returns an empty `JobRuntime` for a job the Service has none for, so callers need no nil check. |
| `TestJobsViewStateJobAtRejectsRowsOutsideTheFilter` | Verifies row lookups are bounded by the filtered rows, which is what the list widget draws from. |

---

### src/ui/history_view_test.go

**Package:** `ui`

Tests the History tab: the pure activity helpers and the sorted-snapshot and
column-width behaviour of the assembled table.

| Test | Purpose |
|------|---------|
| `TestHistoryCellText` | Verifies table cell text for all columns; empty trigger → `Unknown`. |
| `TestLogFileName` | Verifies log path basename extraction on Windows and Unix paths. |
| `TestNewEventUsesConsistentTimestampShape` | Verifies UI events use the same timestamp layout as run records. |
| `TestLastJobLogsCapsAndCopies` | Verifies activity panel cap and defensive copy semantics. |
| `TestLastJobLogsEmpty` | Verifies nil/empty log input returns an empty slice. |
| `TestIndexOfID` | Verifies job lookup by ID returns `-1` when not found. |
| `TestHistorySortToggleKeepsRowsInSync` | Regression guard for the cached sorted snapshot: the length callback and the cells must be refilled together, or the row count and the cell contents disagree. |
| `TestHistoryCellTemplateIsPlainText` | Verifies the cell template already carries the zero `TextStyle`, since the per-cell assignment that used to reset it is gone. |
| `TestTextColumnWidthClamps` | Covers the three shapes of `textColumnWidth`: below the minimum, in range, and capped at the maximum. |
| `TestHistoryColumnsFitTheirContent` | Verifies every column is at least as wide as its widest known or present value, at the default text size and at a scaled theme. |
| `TestHistoryLogCapsRecords` | Regression guard for the unbounded History list: the log keeps the newest `maxHistoryRows` records, drops the oldest from the front, and trims a list handed in already over the cap. |
| `TestHistoryLogWidthsMatchAFullScan` | Verifies the incremental column widths equal a full rescan while every measured record is still present — the cheaper path must not clip what the old one showed. |
| `TestHistoryLogWidthsDoNotShrinkWhenRecordsAgeOut` | Verifies a column keeps its width after the record that set it is dropped by the cap, since the rows on screen were laid out against it. |
| `TestHistoryLogRescansOnThemeChange` | Verifies a theme change falls back to a full rescan, the one case the incremental fold cannot handle because every stored width was measured at the old text size. |

---

### src/ui/settings_view_test.go

**Package:** `ui`

Tests the Settings tab helpers and the row layout.

| Test | Purpose |
|------|---------|
| `TestSettingsFolderPath` | Verifies the folder the Logs directory "Open" button targets: blank text yields no path, a relative path resolves against the application directory, an absolute path is used as typed. |
| `TestSettingsRowStretchesItsControl` | Verifies the row's centre slot already stretches the control to the column width — the property that made a fixed-width wrapper around it redundant. |
| `TestChooseFileAppliesFilter` | Verifies the deduplicated picker opens a dialog both with a nil filter (the command browser) and with a concrete one (`chooseJSONFile`). |
| `TestSettingsCaptionsCoverEveryRow` | Verifies every caption used in a row is present in `settingsCaptions` and fits the measured caption column. |

---

### src/ui/layout_test.go

**Package:** `ui`

Tests the theme-derived sizing helpers in `layout.go`.

| Test | Purpose |
|------|---------|
| `TestRowOverlapMatchesInnerPadding` | Pins `rowOverlap` to `-theme.InnerPadding()` under two themes, the property that lets it follow a theme instead of drifting from a hand-tuned literal. |
| `TestCancelRowOverlapAddsBackOneInnerPadding` | Verifies that `cancelRowOverlap` adds back exactly one inner padding on the top edge only, leaving width and the row below unaffected. |
| `TestCaptionColumnWidth` | Covers no captions, one, and several of varying length, at two text sizes. |

---

### src/ui/theme_test.go

**Package:** `ui`

Tests the branded theme and the stored theme choice.

| Test | Purpose |
|------|---------|
| `TestGoSentryThemeBrandColors` | Verifies the brand colors land on the semantically correct `ColorName`s in both the light and dark variants. |
| `TestGoSentryThemeDelegatesUnbrandedColors` | Verifies unbranded color names fall through to the base theme rather than rendering transparent. |
| `TestThemeForChoice` | Verifies the GoSentry choice and the empty legacy value yield the branded primary; only the explicit system choice yields Fyne's built-in theme. |
| `TestThemeLabelRoundTrip` | Verifies the dropdown labels round-trip and that the empty value maps to the GoSentry label rather than a blank option. |

---

### src/ui/mainwindow_test.go

**Package:** `ui`

Tests main view construction with an injected `*app.Service`.

| Test | Purpose |
|------|---------|
| `TestMainViewFitsTheDefaultWindowSize` | Verifies the assembled content's minimum fits the window size the app asks for, so Fyne never silently widens the window past it. The store's config path is deliberately long, since it was the path label that used to grow the Settings tab. |
| `TestMainViewRecordStartupAddsHistoryRow` | Verifies the `recordStartup` closure `newMainView` returns appends the startup receipt to History and redraws the table, with the windowed and tray wordings `run.go` selects between. |

---

### src/ui/notify_timing_test.go

**Package:** `ui`

Tests the failure-notification timing diagnostics added in 1.0.2.

| Test | Purpose |
|------|---------|
| `TestNotificationTimingFormatLine` | Verifies `notificationTiming.formatLine` renders the job name and the three millisecond deltas (`ms_after_run`, `ms_fyne_do`, `ms_send`) plus their sum (`ms_app_total`). |
| `TestAppendNotificationTimingLogWritesHeaderAndRow` | Verifies `appendNotificationTimingLog` creates `notify-timing.tsv` with its header on first write and appends a row containing the job name. |

---

## Test Design Principles

1. **Isolation** — Tests use `t.TempDir()` for file operations and `t.Setenv()` for environment variables to avoid affecting system state.

2. **Cross-platform** — Platform-specific tests use `//go:build` tags and `runtime.GOOS` checks to skip when not applicable.

3. **Fake clocks and runners** — The scheduler is exercised with an injected fake `Clock`; the service operations tests inject a fake `runJob` function to avoid spawning real processes.

4. **Event-driven correctness** — `app` tests subscribe to the event bus and assert that the expected events are emitted, rather than inspecting internal fields directly.

5. **Path Handling** — Extensive tests cover Windows path quoting, spaces in paths, and case-insensitive matching to avoid subtle shell escaping bugs.

6. **Start-Only Mode** — Special handling for long-running processes that should be launched but not waited on, tested separately from normal execution flow.

7. **Regression on serious fixes** — Any fix from an internal review with severity ≥ medium gets a targeted regression test (see `run_test.go` for examples).

8. **Geometry is measured, not eyeballed** — The `ui` tests that build widgets under `test.NewApp()` assert sizes and offsets, and several re-run under a scaled theme. That is what keeps [STANDARDS.md](STANDARDS.md)'s "measure at build time, never a pixel constant" rule enforceable rather than aspirational.

9. **Redundancy is measured, not read** — Before deleting a test as a duplicate, run both in isolation with `-coverprofile` and compare the profiles. Identical coverage alone is *not* grounds for deletion: several kept tests hit the same statements while asserting genuinely different properties (see the table below). Deletion requires identical coverage **and** assertions that are a subset of the survivor's.

---

## Look-alike tests that are kept

Every pair here has an identical coverage profile, so a redundancy pass will
flag them again. They were measured under principle 9 and kept because the
assertions differ — not because nobody looked.

| Tests | Why both stay |
|-------|---------------|
| `TestSetGlobalPausePersistsToConfigFile` / `TestSetGlobalPauseUpdatesRuntimesAndEmits` | The first asserts the flag reaches `gosentry.json`, which is what makes the pause survive a restart; the second asserts the in-memory runtimes and the emitted event. |
| `TestRunDueQueueDrainsMultipleOverlaps` / `TestRunDueQueueRerunsAfterFinish` | The first drains three queued occurrences rather than one, so it is the test that would catch a drain loop that fires only once. |
| `TestCreateStartupShortcutHandlesCyrillicPath` / `TestCreateStartupShortcutHandlesSpaces` | Non-ASCII paths and paths with spaces are different real-world failure modes for the WScript.Shell COM call. |
| `TestRunJobLogFileAllHeaders` / `TestRunJobRecordFields` / `TestRunJobWritesLogFile` | Three different subjects: the log file's headers, the returned `RunRecord`'s fields, and the log file's name and directory. The fixtures differ too — only `TestRunJobWritesLogFile` runs the `Manual` trigger. Merging them into one `RunJob` call was measured and declined: it saves ~90 ms (the three cost 0.14 s combined; the `runner` package's seconds are `TestRunJobTimesOut` and `TestRunJobZeroTimeoutMeansNoTimeout`, which wait on purpose) and would drop the `Manual` path from the header assertions. |

---

## Remaining Test Coverage Gaps

- Full GUI E2E — tab navigation, dialog flows, and native file pickers are not exercised end-to-end; the `ui` tests assemble views and measure them, but nothing drives a real window.
- History is session-only by design — `.log` files seed aggregate stats only, not the History table (see [STANDARDS.md](STANDARDS.md))
- Fyne's headless driver cannot report a maximized window, which is why window-size persistence stays frozen in [ROADMAP.md](ROADMAP.md)

### Functions deliberately at 0%

A coverage run over the non-UI packages reports these as uncovered. All are
intentional; none is an oversight to be "fixed" with a test.

- The real `Clock` — a fake is injected everywhere it is used.
- `storage.OpenStore`, `storage.ResolvePaths`, `storage.PeekKeepRunningInTray`, `app.Service.Start`, `app.Service.Open` — process entry points, exercised by running the app.
- The autostart and desktop-icon wrappers — OS integration, driven only on a real desktop.
- `app.Service.ShouldNotifyOnFailure` — a getter under the mutex.
