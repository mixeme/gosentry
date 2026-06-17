# GoSentry Architecture

This document shows the current component interaction model. GoSentry is still a
single desktop process: the GUI, scheduler, storage, and command runner live in
one application and communicate through Go function calls and shared in-memory
job state.

## Component Diagram

```mermaid
flowchart LR
    user["Desktop user"]
    gui["src/gui - Fyne windows, tabs, dialogs"]
    store["src/core Store - YAML config and jobs"]
    scheduler["src/core Scheduler - @every and cron timing"]
    runner["src/core Runner - shell command execution"]
    autostart["src/core Autostart - Windows Startup shortcut / Linux desktop startup"]
    config["gosentry.yaml - application settings"]
    jobs["jobs.yaml - job definitions"]
    logs["logs_dir - per-run command output logs"]
    shell["Platform shell - cmd.exe /C or sh -c"]

    user -->|"edits jobs, settings, runs commands"| gui
    gui -->|"OpenStore, SaveConfig, SaveJobs"| store
    store -->|"read/write"| config
    store -->|"read/write"| jobs

    gui -->|"Start, Pause, RunNow, RefreshSchedule"| scheduler
    scheduler -->|"SaveJobs after state changes"| store
    scheduler -->|"RunJob(trigger)"| runner
    runner -->|"execute command"| shell
    runner -->|"write stdout/stderr log"| logs
    runner -->|"RunRecord with status, duration, log path"| scheduler
    scheduler -->|"onChange RunRecord"| gui
    gui -->|"display History, command output, job state"| user

    gui -->|"SetAutostart, AutostartStatus"| autostart
    autostart -->|"use executable path from resolved Paths"| config
```

## Main Flows

1. Startup:
   The executable starts `cmd/gosentry`, which calls the GUI package. The GUI
   opens the store, loads `gosentry.yaml` and `jobs.yaml`, creates the main tabs,
   then starts the scheduler with the loaded job slice.

2. Editing settings or jobs:
   The GUI updates the in-memory job/config state and asks `Store` to write YAML
   back to disk. Job definitions stay in one `jobs.yaml`; runtime command output
   is not stored there.

3. Scheduled run:
   `Scheduler` checks due jobs on a one-second ticker. When a job is due, it marks
   the job as running, saves state, and starts `Runner` asynchronously.

4. Manual run:
   `Run now` calls the same scheduler path as scheduled execution, but the
   resulting history record uses the `Manual` trigger.

5. Command execution:
   `Runner` executes the command through the platform shell, captures stdout and
   stderr, writes one timestamped `.log` file, and returns a `RunRecord`.

6. History update:
   The scheduler receives the `RunRecord`, updates the matching job, saves YAML,
   runs log cleanup, and calls the GUI callback so the `History` tab refreshes.

7. Autostart:
   The Settings tab calls the platform autostart implementation. Windows uses a
   shortcut in the current user's Startup folder. Linux uses a desktop-session
   startup entry. Both autostart mechanisms pass `--start-in-tray`, so the
   scheduler starts without opening the main window after sign-in.
