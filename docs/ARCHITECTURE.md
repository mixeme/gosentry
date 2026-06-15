# PySentry Architecture

This document shows the current component interaction model. PySentry is still a
single desktop process: the GUI, scheduler, storage, and command runner live in
one application and communicate through Go function calls and shared in-memory
job state.

## Component Diagram

```mermaid
flowchart LR
    user["Desktop user"]
    gui["src/gui\nFyne windows, tabs, dialogs"]
    store["src/core Store\nYAML config and jobs"]
    scheduler["src/core Scheduler\n@every and cron timing"]
    runner["src/core Runner\nshell command execution"]
    autostart["src/core Autostart\nWindows Run / Linux desktop startup"]
    config["pysentry.yaml\napplication settings"]
    jobs["jobs.yaml\njob definitions"]
    logs["logs_dir\nper-run command output logs"]
    shell["Platform shell\ncmd.exe /C or sh -c"]

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
   The executable starts `cmd/pysentry`, which calls the GUI package. The GUI
   opens the store, loads `pysentry.yaml` and `jobs.yaml`, creates the main tabs,
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
   The Settings tab calls the platform autostart implementation. Windows uses the
   current user's Run registry key. Linux uses a desktop-session startup entry.
