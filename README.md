# GoSentry

GoSentry is a cross-platform desktop scheduler inspired by cron. It provides a native GUI for creating, grouping, pausing, running, and monitoring scheduled shell commands.

GoSentry is being designed and implemented with assistance from OpenAI Codex.

Project notes:

- [Changelog](docs/CHANGELOG.md)
- [Roadmap](docs/ROADMAP.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Development](docs/DEVELOPMENT.md)

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

## Troubleshooting

### Windows, VirtualBox, RDP, And OpenGL

GoSentry uses [Fyne](https://fyne.io/), and Fyne uses GLFW/OpenGL to create the
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
   [mesa-dist-win](https://github.com/pal1000/mesa-dist-win/releases). For a
   regular Windows x64 GoSentry build, use the archive named like
   `mesa3d-<version>-release-mingw.7z`, for example
   `mesa3d-26.1.1-release-mingw.7z`. This matches the MSYS2 GCC toolchain used
   to build GoSentry. The `devel`, `debug-info`, `tests`, and checksum files
   are not needed for this workaround.
2. Open the downloaded archive and use the `x64` build from it.
3. Copy the Mesa OpenGL DLL files from `x64` into the same directory as the
   GoSentry `.exe`, for example:

```text
dist\windows\
  gosentry-0.3.0-windows-amd64.exe
  opengl32.dll
  ...
```

This makes Windows load Mesa's software OpenGL implementation next to the
application binary, which lets the Fyne window start even when the VirtualBox/RDP
driver does not provide usable OpenGL.

## Storage

GoSentry creates its runtime files next to the executable by default.

`gosentry.yaml` stores application settings:

```yaml
# Directory containing jobs.yaml. "." means "the folder where the GoSentry
# executable lives"; an absolute path can be used when jobs should live elsewhere.
jobs_dir: .

# Directory for per-run command output logs. Relative paths are resolved against
# the program folder, just like jobs_dir.
logs_dir: logs

# Keep at most this many .log files after cleanup. Newest logs are preserved.
max_log_files: 100

# Delete .log files older than this many days during cleanup.
max_log_age_days: 30

# Start GoSentry automatically when the current desktop user signs in.
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
    command: echo GoSentry test job: scheduler is alive

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

1. Start GoSentry.
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

GoSentry is a user desktop application, not a system daemon, so autostart should be configured per user.

Linux:

```ini
# GoSentry writes an XDG Autostart desktop entry when Start on login is enabled.
# This is better for a GUI/tray application than a systemd user service because
# the desktop environment starts it inside the graphical user session.
# Saving the setting also removes the old ~/.config/systemd/user/pysentry.service
# unit if it was created by an earlier GoSentry build.
~/.config/autostart/gosentry.desktop

[Desktop Entry]
Type=Application
Name=GoSentry
Exec=/opt/gosentry/gosentry-0.3.0-linux-amd64 --start-in-tray
Terminal=false
```

Windows:

```text
# GoSentry writes a shortcut to the current user's Startup folder when Start on
# login is enabled. A .lnk stores the executable path as a structured TargetPath,
# and stores --start-in-tray as Arguments, so paths with spaces do not need
# fragile command-line quoting. Saving settings rewrites the shortcut and removes
# old HKCU Run entries from earlier builds.
%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\GoSentry.lnk
```

