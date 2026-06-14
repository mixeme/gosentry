# PySentry

PySentry is a cross-platform desktop scheduler inspired by cron. It provides a native GUI for creating, grouping, pausing, running, and monitoring scheduled shell commands.

## Features

- Native desktop GUI built with Fyne.
- Job storage in one clean YAML file.
- App settings in a separate YAML file.
- `@every` schedules and standard 5-field cron expressions.
- Manual and scheduled command runs.
- Per-run `.log` files with stdout/stderr.
- Log cleanup by maximum file count and maximum age.
- Global pause/resume for all job execution.
- Windows tray support.

## Requirements

Common:

- Go 1.22 or newer.

Windows:

- MSYS2 with UCRT64 GCC in `C:\msys64\ucrt64\bin`.

Linux:

- A C compiler.
- Fyne native build dependencies, including OpenGL/X11 development packages.

On Debian/Ubuntu, the Linux dependencies are typically:

```bash
sudo apt install golang gcc libgl1-mesa-dev xorg-dev
```

## Build

Windows:

```powershell
.\scripts\build-windows.ps1
```

The binary is written to:

```text
dist\windows\pysentry.exe
```

Linux:

```bash
chmod +x ./scripts/build-linux.sh
./scripts/build-linux.sh
```

The binary is written to:

```text
dist/linux/pysentry
```

Linux using Docker:

```powershell
.\scripts\build-linux-docker.ps1
```

The binary is copied to:

```text
dist\linux\pysentry
```

## Run From Source

Windows:

```powershell
$env:Path = 'C:\msys64\ucrt64\bin;' + $env:Path
$env:CGO_ENABLED = '1'
& 'C:\Program Files\Go\bin\go.exe' run ./cmd/pysentry
```

Linux:

```bash
CGO_ENABLED=1 go run ./cmd/pysentry
```

## Storage

PySentry creates its runtime files next to the executable by default.

`pysentry.yaml` stores application settings:

```yaml
jobs_dir: .
logs_dir: logs
max_log_files: 100
max_log_age_days: 30
keep_running_in_tray: true
notify_on_failure: true
```

`jobs.yaml` stores only job definitions:

```yaml
jobs:
  - id: 1
    name: Hello scheduler
    folder: Examples
    schedule: '@every 10s'
    command: echo PySentry test job: scheduler is alive
    enabled: true
```

Command output is written to separate files under `logs_dir`. File names include the run timestamp and job name, for example:

```text
20260614-224306_Hello_scheduler.log
```

## Schedules

Fast interval schedules:

```text
@every 10s
@every 5m
@every 1h30m
```

Standard 5-field cron schedules:

```text
*/5 * * * *      every five minutes
0 2 * * *        every day at 02:00
30 9 * * 1-5     weekdays at 09:30
```

## Using The App

1. Start PySentry.
2. Use `New job` to create a command.
3. Set `Schedule`, `Command`, optional `Folder`, and `Enabled`.
4. Use `Run now` for a manual test run.
5. Use `Pause` to disable one job.
6. Use `Pause all` as a global stop switch.
7. Open `History` to see whether a run was `Manual`, `Schedule`, or `UI`.
8. Open `Settings` to change `jobs_dir`, `logs_dir`, and log cleanup limits. Use `Browse` to choose directories.

Changing `jobs_dir` saves the current job list to the new directory.

## Project Layout

- `cmd/pysentry` starts the desktop app.
- `internal/app` contains the GUI.
- `internal/core` contains YAML storage, command execution, scheduling, and log cleanup.
- `assets` contains app icons.
- `scripts` contains build helpers.

## Dependencies

PySentry keeps the direct dependency list intentionally small:

- `fyne.io/fyne/v2` for the native GUI.
- `github.com/robfig/cron/v3` for cron schedule parsing.
- `gopkg.in/yaml.v3` for YAML settings and jobs.

The remaining entries in `go.mod` are indirect dependencies pulled by Fyne and the Go module resolver.
