# PySentry

PySentry is a cross-platform desktop scheduler inspired by cron. It provides a native GUI for creating, grouping, pausing, running, and monitoring scheduled shell commands.

PySentry is being designed and implemented with assistance from OpenAI Codex.

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
- Version shown in the window title, Settings, and build artifact names.

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
# Go builds the application, gcc is required by CGO/Fyne, and the OpenGL/X11
# development packages provide the native desktop headers used by Fyne.
sudo apt install golang gcc libgl1-mesa-dev xorg-dev
```

## Build

Windows:

```powershell
# Builds dist\windows\pysentry-<version>-windows-amd64.exe. The script changes
# to the repository root first, so double-clicking it from Explorer works. It
# also adds MSYS2 UCRT64 to PATH for this process only, embeds the Windows icon
# when windres is available, and uses the Windows GUI subsystem so no console
# window opens at startup.
.\scripts\build-windows.bat
```

The Windows build is created as a GUI application, so it does not open a terminal window.

The binary is written to:

```text
# GUI executable produced by scripts\build-windows.bat.
dist\windows\pysentry-0.1.0-windows-amd64.exe
```

Linux:

```bash
# Make the helper executable once, then build a linux/amd64 Fyne binary.
chmod +x ./scripts/build-linux.sh
./scripts/build-linux.sh
```

The binary is written to:

```text
# Linux executable produced by scripts/build-linux.sh.
dist/linux/pysentry-0.1.0-linux-amd64
```

Linux using Docker:

```bash
# Builds the Linux binary inside Docker using the image tag
# gitea.mixdep.ru/mix/pysentry-builder. Useful from hosts or CI jobs where the
# native Linux/Fyne packages are not installed locally.
chmod +x ./scripts/build-linux-docker.sh
./scripts/build-linux-docker.sh
```

The binary is copied to:

```text
# Linux executable copied out of the Docker build image.
dist\linux\pysentry-0.1.0-linux-amd64
```

Release build from Linux:

```bash
# Builds Linux amd64, Linux arm64, and Windows amd64 artifacts from one
# Linux/Docker workflow. The Dockerfile includes Linux Fyne dependencies plus
# cross-compilers for arm64 Linux and the Windows .exe.
chmod +x ./scripts/build-release-linux.sh
./scripts/build-release-linux.sh
```

The binaries are copied to:

```text
# Linux artifact.
dist/linux/pysentry-0.1.0-linux-amd64

# Linux arm64 artifact.
dist/linux/pysentry-0.1.0-linux-arm64

# Windows artifact cross-compiled from Linux.
dist/windows/pysentry-0.1.0-windows-amd64.exe
```

## Run From Source

Windows:

```powershell
# Fyne requires CGO on Windows. MSYS2 UCRT64 provides the C compiler and native
# libraries used by the desktop backend.
$env:Path = 'C:\msys64\ucrt64\bin;' + $env:Path
$env:CGO_ENABLED = '1'

# go run starts the app from source. Use scripts\build-windows.bat when you need
# a standalone .exe without a console window.
& 'C:\Program Files\Go\bin\go.exe' run ./cmd/pysentry
```

Linux:

```bash
# CGO must stay enabled because the Fyne GUI links against native Linux desktop
# libraries.
CGO_ENABLED=1 go run ./cmd/pysentry
```

## Storage

PySentry creates its runtime files next to the executable by default.

`pysentry.yaml` stores application settings:

```yaml
# Directory containing jobs.yaml. "." means "the folder where the PySentry
# executable lives"; an absolute path can be used when jobs should live elsewhere.
jobs_dir: .

# Directory for per-run command output logs. Relative paths are resolved against
# the program folder, just like jobs_dir.
logs_dir: logs

# Keep at most this many .log files after cleanup. Newest logs are preserved.
max_log_files: 100

# Delete .log files older than this many days during cleanup.
max_log_age_days: 30

# Start PySentry automatically when the current desktop user signs in.
start_on_login: false

# Closing the window hides it to the tray instead of stopping the scheduler.
keep_running_in_tray: true

# Reserved for desktop failure notifications; the setting is stored now so the
# UI and config format do not need to change when notifications are wired fully.
notify_on_failure: true
```

`jobs.yaml` stores only job definitions:

```yaml
jobs:
  # A harmless sample job created on first run so the scheduler can be tested
  # immediately. Runtime fields such as last run time, next run time, and command
  # output are intentionally not stored here; they are displayed in the GUI and
  # written to separate log files.
  - id: 1
    # Human-readable name shown in the jobs list and used in log file names.
    name: Hello scheduler

    # Optional grouping label. Omit it or leave it empty to put the job under
    # the "No folder" filter.
    folder: Examples

    # Either @every with a Go duration, or a standard five-field cron expression.
    schedule: '@every 1m'

    # Command passed to the platform shell: cmd.exe /C on Windows, sh -c on Linux.
    command: echo PySentry test job: scheduler is alive

    # Disabled jobs remain in jobs.yaml but are skipped by the scheduler.
    enabled: true
```

Command output is written to separate files under `logs_dir`. File names include the run timestamp and job name, for example:

```text
# Format: YYYYMMDD-HHMMSS_<sanitized job name>.log
20260614-224306_Hello_scheduler.log
```

## Schedules

Fast interval schedules:

```text
# Go duration syntax after @every; useful for tests and simple intervals.
@every 10s
@every 5m
@every 1h30m
```

Standard 5-field cron schedules:

```text
# Standard five-field cron: minute hour day-of-month month day-of-week.
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

The `Start on login` setting shows an `OK` or `Problem` status next to the checkbox. Saving settings with the checkbox enabled rewrites the autostart entry using the current executable path.

## Autostart

PySentry is a user desktop application, not a system daemon, so autostart should be configured per user.

Linux:

```ini
# PySentry writes a systemd user unit and enables it with
# systemctl --user enable --now pysentry.service when Start on login is enabled.
# A user unit starts after login and can run the tray/GUI app in the user's
# desktop session.
[Unit]
Description=PySentry desktop scheduler

[Service]
ExecStart=/opt/pysentry/pysentry-0.1.0-linux-amd64
Restart=on-failure

[Install]
WantedBy=default.target
```

Windows:

```text
# PySentry writes an HKCU Run entry when Start on login is enabled. It needs no
# administrator rights and starts PySentry when the current user signs in. Task
# Scheduler remains a later option if delayed start or elevated tasks become
# necessary. Saving settings with the checkbox enabled rewrites this entry, so it
# repairs an old path after the executable was moved or renamed.
HKCU\Software\Microsoft\Windows\CurrentVersion\Run\PySentry
```

## Project Layout

- `cmd/pysentry` starts the desktop app.
- `src/gui` contains the GUI.
- `src/core` contains YAML storage, command execution, scheduling, and log cleanup.
- `assets` contains app icons that are embedded into the application binary.
- `scripts` contains build helpers.

Build outputs are written to `dist/`. The old local `bin/` directory is not used.

## Dependencies

PySentry keeps the direct dependency list intentionally small:

- `fyne.io/fyne/v2` for the native GUI.
- `github.com/robfig/cron/v3` for cron schedule parsing.
- `gopkg.in/yaml.v3` for YAML settings and jobs.

The remaining entries in `go.mod` are indirect dependencies pulled by Fyne and the Go module resolver.
