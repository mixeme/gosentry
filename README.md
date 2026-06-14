# PySentry

PySentry is a cross-platform desktop scheduler inspired by cron.

The project is starting with the GUI shell first, then the scheduling core.

## Requirements

- Go 1.22 or newer
- A C compiler for Fyne builds on Windows, for example MSYS2/MinGW-w64

## Run

```powershell
go mod tidy
go run ./cmd/pysentry
```

If Go is installed but not available in `PATH`, use the full path:

```powershell
& 'C:\Program Files\Go\bin\go.exe' run ./cmd/pysentry
```

## Current shape

- `cmd/pysentry` starts the desktop app.
- `internal/app` contains the first Fyne-based interface prototype.
- `internal/core` contains YAML storage, command execution, and the first scheduler loop.
- Jobs can be created, edited, paused/resumed, run manually, and persisted to YAML.
- Settings are stored in `pysentry.yaml` next to the executable.
- Jobs are stored in one `jobs.yaml` file. The job directory is configured by `jobs_dir` and defaults to the executable directory.
- Command output is also written to per-run `.log` files in `logs_dir`. Log filenames include the run timestamp and job name.
- Log cleanup is controlled by `max_log_files` and `max_log_age_days`.
- The current scheduler supports `@every` schedules such as `@every 10s` and `@every 1m`.
- Run history records include a `trigger` value such as `Manual`, `Schedule`, or `UI`.
- Cron expression parsing is planned for the next phase.
