# GoSentry Test Suite

All tests are located alongside source code in the `src/core/` package. Tests follow Go conventions with `*_test.go` filename patterns.

## Test Files Overview

### store_test.go
**Location:** `src/core/store_test.go`  
**Package:** `core`

Tests YAML serialization and storage behavior.

| Test | Purpose |
|------|---------|
| `TestJobsYAMLDoesNotPersistRuntimeNoise` | Verifies that `jobs.yaml` does not persist runtime state fields (LastRun, NextRun, LastState, Output, etc.). Only job definitions are stored; runtime data is kept in memory and log files. |

---

### scheduler_test.go
**Location:** `src/core/scheduler_test.go`  
**Package:** `core`

Tests schedule parsing and job invocation output formatting.

| Test | Purpose |
|------|---------|
| `TestNextRunTimeSupportsEvery` | Verifies `@every` duration syntax (e.g., `@every 10s`) correctly calculates next run time. Tests with 10-second interval. |
| `TestNextRunTimeSupportsCron` | Verifies standard 5-field cron expressions (e.g., `*/5 * * * *`) correctly calculate next run time. Tests 5-minute interval. |
| `TestRunningOutputIncludesInvocation` | Verifies the running job output header includes all relevant invocation details: command, arguments, success exit codes, start time, and trigger type. |

---

### runner_test.go
**Location:** `src/core/runner_test.go`  
**Package:** `core`

Tests command execution, exit code handling, output capture, and Windows-specific process behavior.

#### Log File Tests

| Test | Purpose |
|------|---------|
| `TestRunJobWritesLogFile` | Verifies that each job execution creates a `.log` file in the configured logs directory with sanitized job name in filename and proper metadata (trigger type, job name, command output). |

#### Command Execution Tests

| Test | Platform | Purpose |
|------|----------|---------|
| `TestRunJobRunsQuotedWindowsExecutable` | Windows | Verifies that executable paths with quotes (e.g., `"C:\Program Files\..."`) are executed correctly via cmd.exe. |
| `TestRunJobRunsUnquotedWindowsProgramPathWithSpaces` | Windows | Verifies that unquoted executable paths with spaces (e.g., `C:\Program Files\App\app.exe`) are quoted and executed correctly. |
| `TestRunJobRunsWindowsCommandWithSeparateArguments` | Windows | Verifies that command and arguments separated in the Job struct are combined and executed correctly. |

#### Exit Code Handling Tests

| Test | Purpose |
|------|---------|
| `TestRunJobAcceptsConfiguredExitCode` | Verifies that exit codes listed in `SuccessExitCodes` (e.g., `"0,1"`) result in "OK" status even if nonzero. Includes detail message about accepted exit code. |
| `TestRunJobRejectsUnconfiguredExitCode` | Verifies that exit codes not listed in `SuccessExitCodes` result in "Failed" status with exit code detail. |

#### Start-Only Mode Tests

| Test | Purpose |
|------|---------|
| `TestRunJobStartOnlyDoesNotWaitForExitCode` | Verifies that jobs with `StartOnly: true` launch the process and return "OK" immediately without waiting for process exit or checking exit code. |
| `TestRunJobStartOnlyReportsStartFailure` | Verifies that jobs with `StartOnly: true` still report "Failed" if the process fails to start (e.g., executable not found). |

#### Utility Function Tests

| Test | Platform | Purpose |
|------|----------|---------|
| `TestParseExitCodes` | All | Verifies that exit code strings with mixed separators (comma, semicolon, newline) are correctly parsed into integer slice. |
| `TestDirectCommandDoesNotHideWindow` | Windows | Verifies that direct executable commands (with explicit path and arguments) do not request hidden window startup. |
| `TestShellCommandHidesWindow` | Windows | Verifies that shell commands (passed to cmd.exe) request hidden window startup to prevent console flash. |
| `TestShellCommandUsesWindowsSafeQuoting` | Windows | Verifies that shell commands use cmd.exe `/S /C` syntax with proper outer quoting to handle paths with spaces and special characters. |
| `TestWindowsShellCommandLineQuotesUnquotedProgramPath` | Windows | Verifies that unquoted program paths in shell commands are quoted while preserving already-quoted arguments. |

---

### autostart_windows_test.go
**Location:** `src/core/autostart_windows_test.go`  
**Package:** `core`  
**Build Tags:** `//go:build windows` (Windows only)

Tests Windows autostart entry creation via shortcuts in the Startup folder.

| Test | Purpose |
|------|---------|
| `TestParseRegistryRunValue` | Verifies that legacy Windows Registry `Run` entry values are correctly parsed from `reg query` output (for migration/cleanup). |
| `TestSameWindowsPathIgnoresCaseAndQuotes` | Verifies that Windows path comparison is case-insensitive and handles quote marks correctly (e.g., `"D:\..."` matches `d:\...`). |
| `TestSameWindowsPathHandlesSpaces` | Verifies that Windows path comparison correctly matches paths with spaces both with and without quotes. |
| `TestStartupShortcutPathUsesUserStartupFolder` | Verifies that the startup shortcut path resolves to the user's Startup folder using `%APPDATA%` environment variable. |
| `TestCreateStartupShortcutHandlesSpaces` | Verifies that `.lnk` shortcut files are created with correct `TargetPath` and `Arguments` (--start-in-tray) even when target path contains spaces. |

---

### autostart_linux_test.go
**Location:** `src/core/autostart_linux_test.go`  
**Package:** `core`  
**Build Tags:** `//go:build linux` (Linux only)

Tests Linux autostart entry creation via XDG Desktop Entry files.

| Test | Purpose |
|------|---------|
| `TestLinuxAutostartStartsInTray` | Verifies that the XDG Desktop Entry is created with the `--start-in-tray` argument in the `Exec=` field, so scheduled jobs run immediately after login without displaying the window. |
| `TestLinuxAutostartRemovesLegacyDesktopEntry` | Verifies that legacy autostart entries (from old PySentry implementation) are cleaned up when enabling autostart through the new system. |

---

## Running Tests

### Run all tests in the package
```bash
cd D:\Local\Git\gosentry
go test ./src/core
```

### Run tests with verbose output
```bash
go test -v ./src/core
```

### Run specific test by name
```bash
go test -run TestRunJobWritesLogFile ./src/core
```

### Run Windows-only tests (on Windows)
```bash
go test -v ./src/core  # Windows build tags are active
```

### Run Linux-only tests (on Linux)
```bash
go test -v ./src/core  # Linux build tags are active
```

### Run with code coverage
```bash
go test -cover ./src/core
go test -coverprofile=coverage.out ./src/core
go tool cover -html=coverage.out
```

---

## Test Design Principles

1. **Isolation** — Tests use `t.TempDir()` for file operations and `t.Setenv()` for environment variables to avoid affecting system state.

2. **Cross-platform** — Platform-specific tests use `//go:build` tags and `runtime.GOOS` checks to skip when not applicable.

3. **Exit Code Flexibility** — The `SuccessExitCodes` field allows jobs to treat nonzero exit codes as success, tested explicitly.

4. **Path Handling** — Extensive tests cover Windows path quoting, spaces in paths, and case-insensitive matching to avoid subtle shell escaping bugs.

5. **Start-Only Mode** — Special handling for long-running processes that should be launched but not waited on, tested separately from normal execution flow.

---

## Future Test Coverage Gaps

Potential areas for additional tests:
- Job group/folder filtering and persistence
- Log cleanup (max file count and max age)
- Settings persistence and migration
- GUI integration tests (currently untested)
- Concurrent job execution
- Job history and run record storage
