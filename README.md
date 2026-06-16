# PySentry

PySentry is a cross-platform desktop scheduler inspired by cron. It provides a native GUI for creating, grouping, pausing, running, and monitoring scheduled shell commands.

PySentry is being designed and implemented with assistance from OpenAI Codex.

Project notes:

- [Changelog](docs/CHANGELOG.md)
- [Roadmap](docs/ROADMAP.md)
- [Architecture](docs/ARCHITECTURE.md)

## Features

- Native desktop GUI built with [Fyne](https://fyne.io/).
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

- [Go](https://go.dev/) 1.22 or newer.

Windows:

- MSYS2 with UCRT64 GCC in `C:\msys64\ucrt64\bin`.

Install these dependencies on Windows:

```powershell
# 1. Install Go 1.22 or newer from https://go.dev/dl/.
#    The default installer path is C:\Program Files\Go.
go version

# 2. Install MSYS2 from https://www.msys2.org/.
#    Use the default installation path so UCRT64 tools are placed under
#    C:\msys64\ucrt64\bin.

# 3. Open "MSYS2 UCRT64" from the Start menu and install GCC plus windres.
pacman -Syu
pacman -S --needed mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-binutils

# 4. In PowerShell, check that the compiler is available where the build script
#    expects it. build-windows.bat prepends this directory automatically.
Test-Path C:\msys64\ucrt64\bin\gcc.exe
Test-Path C:\msys64\ucrt64\bin\windres.exe
```

Linux:

- A C compiler.
- [Fyne](https://fyne.io/) native build dependencies, including OpenGL/X11 development packages.

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
dist\windows\pysentry-0.2.5-windows-amd64.exe
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
dist/linux/pysentry-0.2.5-linux-amd64
```

Linux using Docker:

```bash
# Builds the Linux binary inside Docker using the versioned image tag
# gitea.mixdep.ru/mix/pysentry-builder:<version>. Useful from hosts or CI jobs
# where the native Linux/Fyne packages are not installed locally.
chmod +x ./scripts/build-linux-docker.sh
./scripts/build-linux-docker.sh
```

The binary is copied to:

```text
# Linux executable copied out of the Docker build image.
dist\linux\pysentry-0.2.5-linux-amd64
```

Release build from Linux:

```bash
# Interactively choose Linux amd64, Linux arm64, Windows amd64, or all artifacts
# from one Linux/Docker workflow. The Dockerfile contains the builder
# environment; the build commands live in this script. Docker runs the build
# with the current user's UID/GID so dist/ files are not owned by root.
chmod +x ./scripts/build-release-linux.sh
./scripts/build-release-linux.sh
```

Non-interactive release builds can pass target names:

```bash
# Build only Linux arm64 and Windows amd64 artifacts.
./scripts/build-release-linux.sh linux-arm64 windows-amd64
```

The binaries are copied to:

```text
# Linux artifact.
dist/linux/pysentry-0.2.5-linux-amd64

# Linux arm64 artifact.
dist/linux/pysentry-0.2.5-linux-arm64

# Windows artifact cross-compiled from Linux.
dist/windows/pysentry-0.2.5-windows-amd64.exe
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

## Troubleshooting

### Windows, VirtualBox, RDP, And OpenGL

PySentry uses [Fyne](https://fyne.io/), and Fyne uses GLFW/OpenGL to create the
desktop window. In a Windows virtual machine, especially when the session is
opened through RDP inside VirtualBox, the available video driver can fail OpenGL
initialization.

Typical error:

```text
Fyne error: window creation error
Cause: APIUnavailable: WGL: The driver does not appear to support OpenGL
At: fyne.io/fyne/v2@v2.5.3/internal/driver/glfw/driver.go:149
```

Known workaround:

1. Download a Windows Mesa build from
   [mesa-dist-win](https://github.com/pal1000/mesa-dist-win/releases).
2. Open the downloaded archive and use the `x64` build.
3. Copy the Mesa OpenGL DLL files from `x64` into the same directory as the
   PySentry `.exe`, for example:

```text
dist\windows\
  pysentry-0.2.5-windows-amd64.exe
  opengl32.dll
  ...
```

This makes Windows load Mesa's software OpenGL implementation next to the
application binary, which lets the Fyne window start even when the VirtualBox/RDP
driver does not provide usable OpenGL.

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
Autostart entries add `--start-in-tray`, so scheduled jobs begin running after sign-in without opening the main window.

## Autostart

PySentry is a user desktop application, not a system daemon, so autostart should be configured per user.

Linux:

```ini
# PySentry writes an XDG Autostart desktop entry when Start on login is enabled.
# This is better for a GUI/tray application than a systemd user service because
# the desktop environment starts it inside the graphical user session.
# Saving the setting also removes the old ~/.config/systemd/user/pysentry.service
# unit if it was created by an earlier PySentry build.
~/.config/autostart/pysentry.desktop

[Desktop Entry]
Type=Application
Name=PySentry
Exec=/opt/pysentry/pysentry-0.2.5-linux-amd64 --start-in-tray
Terminal=false
```

Windows:

```text
# PySentry writes a shortcut to the current user's Startup folder when Start on
# login is enabled. A .lnk stores the executable path as a structured TargetPath,
# and stores --start-in-tray as Arguments, so paths with spaces do not need
# fragile command-line quoting. Saving settings rewrites the shortcut and removes
# old HKCU Run entries from earlier builds.
%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\GoSentry.lnk
```

## Project Layout

- `cmd/pysentry` starts the desktop app.
- `src/gui` contains the GUI.
- `src/core` contains YAML storage, command execution, scheduling, and log cleanup.
- `assets` contains app icons that are embedded into the application binary.
- `scripts` contains build helpers.
- `docs` contains architecture notes, the changelog, and the roadmap.

Build outputs are written to `dist/`. The old local `bin/` directory is not used.

## Dependencies

PySentry keeps the direct dependency list intentionally small:

- [`fyne.io/fyne/v2`](https://fyne.io/) for the native GUI.
- `github.com/robfig/cron/v3` for cron schedule parsing.
- [`go.yaml.in/yaml/v4`](https://github.com/yaml/go-yaml) for YAML settings and jobs.

The remaining entries in `go.mod` are indirect dependencies pulled by Fyne and the Go module resolver.

Source repositories for mirroring:

- Go toolchain: https://go.googlesource.com/go
- Fyne: https://github.com/fyne-io/fyne
- robfig/cron: https://github.com/robfig/cron
- yaml/go-yaml: https://github.com/yaml/go-yaml

To list every direct and indirect Go module used by the current checkout:

```bash
go list -m all
```
