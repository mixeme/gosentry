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

### src/app/service_test.go

**Package:** `app`

Tests `Service` construction and the state-accessor contract.

| Test | Purpose |
|------|---------|
| `TestNewServiceBuildsRuntimePerJob` | Verifies that `NewService` creates a `JobRuntime` entry for every loaded job. |
| `TestJobsReturnsCopy` | Verifies that `Service.Jobs` returns a defensive copy so callers cannot mutate internal state. |
| `TestStoreReturnsWiredStore` | Verifies that `Service.Store` returns the injected `storage.Store`. |

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

#### Global pause / run-now / run-due

| Test | Purpose |
|------|---------|
| `TestSetGlobalPauseUpdatesRuntimesAndEmits` | Verifies that `SetGlobalPause` updates all job runtimes, emits `SchedulerStateChanged`, and persists state. |
| `TestRunNowUsesRunnerAndRecords` | Verifies that `RunNow` invokes the runner, records a `RunRecord`, and emits `RunRecorded`. |
| `TestRunNowNotFound` | Verifies that `RunNow` returns an error for an unknown job ID. |
| `TestRunNowRefusedWhileAlreadyRunning` | Verifies that a second concurrent `RunNow` on the same job is rejected while the first is in progress. |
| `TestRunNowRefusedWhilePaused` | Verifies that `RunNow` is rejected when the global pause flag is set. |
| `TestRunDueStartsDueJob` | Verifies that `RunDue` launches a job whose next-run time has passed. |
| `TestRunDueSkipsJobNotYetDue` | Verifies that `RunDue` does not launch a job that is not yet due. |
| `TestRunDueSkipsJobInRunningState` | Verifies that `RunDue` does not start a second concurrent run for an already-running job. |
| `TestRunDueDoesNothingWhilePaused` | Verifies that `RunDue` launches nothing when the global pause flag is set. |
| `TestStartDrivesRunDueOnTick` | Verifies that `Service.Start` wires `RunDue` to the scheduler tick and that each tick advances state. |

#### Settings

| Test | Purpose |
|------|---------|
| `TestUpdateSettingsPersistsAndValidates` | Verifies that `UpdateSettings` persists a valid config and rewrites autostart if needed. |
| `TestUpdateSettingsRejectsInvalidConfigs` | Verifies that `UpdateSettings` returns validation errors without persisting. |
| `TestPrependLogCapsActivityList` | Verifies that the activity log never grows beyond its maximum cap. |

---

### src/app/events_test.go

**Package:** `app`

Tests the event-emission and observer-subscription machinery.

| Test | Purpose |
|------|---------|
| `TestEmitDeliversToAllObserversInOrder` | Verifies that all registered observers receive emitted events in registration order. |
| `TestEmitWithNoObserversIsNoop` | Verifies that emitting an event with no observers does not panic. |
| `TestObserverCanReadServiceState` | Verifies that an observer called by `emit` can safely read Service state (jobs, runtimes). |

---

### src/app/format_test.go

**Package:** `app`

Tests display-formatting helpers used by the UI.

| Test | Purpose |
|------|---------|
| `TestStatusText` | Verifies that job status codes map to the correct display strings. |
| `TestEventText` | Verifies trigger-type labels for scheduled, manual, and UI triggers. |
| `TestDisplayFolder` | Verifies that an empty folder string shows "No folder". |
| `TestDisplayArguments` | Verifies that an empty arguments string shows "None". |
| `TestDisplayRunMode` | Verifies run-mode labels for normal and start-only modes. |
| `TestDisplayInvocation` | Verifies that the full invocation display string combines command and arguments with spacing. |
| `TestDisplayIndex` | Verifies that the display index is one-based (job slice index + 1). |

---

### src/storage/store_test.go

**Package:** `storage`

Tests JSON round-tripping, YAML migration import, and default generation.

| Test | Purpose |
|------|---------|
| `TestJobsRoundTrip` | Verifies that jobs saved to JSON are reloaded with identical field values. |
| `TestConfigRoundTrip` | Verifies that settings saved to JSON are reloaded with identical field values. |
| `TestNormalizeJobsFillsDefaults` | Verifies that `normalizeJobs` assigns sequential IDs and sets default name, schedule, and command for jobs missing those fields. |
| `TestLoadOrCreateConfigMigratesFromLegacy` | Verifies that when `gosentry.json` is absent but `gosentry.yaml` exists the config is imported from the legacy YAML file on first load. |
| `TestLoadOrCreateJobsMigratesFromLegacy` | Verifies that when `jobs.json` is absent but `jobs.yaml` exists the jobs are imported from the legacy YAML file on first load. |
| `TestLoadOrCreateConfigCreatesDefaultsOnFirstRun` | Verifies that a missing config file is created with sane defaults and a sample job. |
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

Tests command execution, exit code handling, output capture, and Windows-specific process behavior.

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

#### Start-only mode

| Test | Purpose |
|------|---------|
| `TestRunJobStartOnlyDoesNotWaitForExitCode` | Verifies that `StartOnly: true` jobs launch and return "OK" immediately without waiting for the process to exit. |
| `TestRunJobStartOnlyReportsStartFailure` | Verifies that `StartOnly: true` jobs still report "Failed" if the process cannot be started. |

#### Utility / Windows invocation

| Test | Platform | Purpose |
|------|----------|---------|
| `TestDirectCommandDoesNotHideWindow` | Windows | Verifies that direct executable commands do not request hidden-window startup. |
| `TestShellCommandHidesWindow` | Windows | Verifies that shell commands request hidden-window startup to prevent console flash. |
| `TestShellCommandUsesWindowsSafeQuoting` | Windows | Verifies `cmd.exe /S /C` quoting for paths with spaces and special characters. |
| `TestWindowsShellCommandLineQuotesUnquotedProgramPath` | Windows | Verifies that unquoted program paths in shell commands are quoted while preserving already-quoted arguments. |

---

### src/runner/cleanup_test.go

**Package:** `runner`

Tests log-file cleanup by age and by count.

| Test | Purpose |
|------|---------|
| `TestCleanupLogsMissingDirReturnsNil` | Verifies that cleanup returns nil (not an error) when the logs directory does not exist. |
| `TestCleanupLogsRemovesFilesPastMaxAge` | Verifies that `.log` files older than `MaxLogAgeDays` are deleted. |
| `TestCleanupLogsKeepsFilesWithinAgeLimit` | Verifies that `.log` files within the age limit are retained. |
| `TestCleanupLogsByCountDeletesOldest` | Verifies that when file count exceeds `MaxLogFiles`, the oldest files are removed first. |
| `TestCleanupLogsNonLogFilesNotDeleted` | Verifies that non-`.log` files in the logs directory are never deleted by cleanup. |
| `TestCleanupLogsSubdirsNotDeleted` | Verifies that subdirectories inside the logs directory are not deleted by cleanup. |
| `TestCleanupLogsZeroLimitsDisableBothPolicies` | Verifies that setting both limits to zero disables both the age and count cleanup policies. |

---

### src/platform/autostart/autostart_windows_test.go

**Location:** `src/platform/autostart/autostart_windows_test.go`
**Build Tags:** `//go:build windows`

Tests Windows autostart via shortcuts in the Startup folder.

| Test | Purpose |
|------|---------|
| `TestParseRegistryRunValue` | Verifies that legacy `HKCU\...\Run` entry values are parsed correctly from `reg query` output (for migration/cleanup). |
| `TestSameWindowsPathIgnoresCaseAndQuotes` | Verifies that Windows path comparison is case-insensitive and handles quote marks correctly. |
| `TestSameWindowsPathHandlesSpaces` | Verifies that Windows path comparison matches paths with and without surrounding quotes. |
| `TestSameWindowsPathStripsExtendedLengthPrefix` | Verifies that `\\?\`-prefixed paths are compared correctly after stripping the prefix. |
| `TestSameWindowsPathMatchesShortNameViaFilesystem` | Verifies that 8.3 short names are resolved to long names for comparison. |
| `TestStartupShortcutPathUsesUserStartupFolder` | Verifies that the shortcut path resolves into `%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup`. |
| `TestCreateStartupShortcutHandlesCyrillicPath` | Verifies that `.lnk` files are created correctly when the executable path contains Cyrillic characters. |
| `TestCreateStartupShortcutHandlesSpaces` | Verifies that `.lnk` files are created with correct `TargetPath` and `--start-in-tray` arguments when the path contains spaces. |

---

### src/platform/autostart/autostart_linux_test.go

**Location:** `src/platform/autostart/autostart_linux_test.go`
**Build Tags:** `//go:build linux`

Tests Linux autostart via XDG Desktop Entry files.

| Test | Purpose |
|------|---------|
| `TestLinuxAutostartStartsInTray` | Verifies that the XDG Desktop Entry is created with `--start-in-tray` in the `Exec=` field. |
| `TestLinuxAutostartRemovesLegacyDesktopEntry` | Verifies that enabling autostart also removes legacy PySentry service files left by earlier builds. |

---

### src/ui/jobs_view_test.go

**Package:** `ui`

Tests pure helper functions in the jobs view (no Fyne widget construction).

| Test | Purpose |
|------|---------|
| `TestFilterValue` | Verifies that `filterValue` returns the correct display string for the current folder filter. |
| `TestFolderOptionsAlwaysIncludesSentinels` | Verifies that the folder filter list always starts with "All" and "No folder" sentinel entries. |
| `TestFolderOptionsAppendsUniqueFolders` | Verifies that folder names from the job list are appended once each, in order, without duplicates. |
| `TestFilteredJobIndexesAll` | Verifies that the "All" filter returns indexes for every job. |
| `TestFilteredJobIndexesByNamedFolder` | Verifies that filtering by a named folder returns only jobs in that folder. |
| `TestFilteredJobIndexesNoFolder` | Verifies that the "No folder" filter returns only jobs with an empty folder field. |
| `TestFilteredJobIndexesEmptySlice` | Verifies that filtering an empty job slice returns an empty index list. |

---

## Test Design Principles

1. **Isolation** — Tests use `t.TempDir()` for file operations and `t.Setenv()` for environment variables to avoid affecting system state.

2. **Cross-platform** — Platform-specific tests use `//go:build` tags and `runtime.GOOS` checks to skip when not applicable.

3. **Fake clocks and runners** — The scheduler is exercised with an injected fake `Clock`; the service operations tests inject a fake `runJob` function to avoid spawning real processes.

4. **Event-driven correctness** — `app` tests subscribe to the event bus and assert that the expected events are emitted, rather than inspecting internal fields directly.

5. **Path Handling** — Extensive tests cover Windows path quoting, spaces in paths, and case-insensitive matching to avoid subtle shell escaping bugs.

6. **Start-Only Mode** — Special handling for long-running processes that should be launched but not waited on, tested separately from normal execution flow.

---

## Remaining Test Coverage Gaps

- GUI integration tests — Fyne widget interaction is not yet tested end-to-end
- Job history on-disk retrieval — RunRecord file reading is not covered
