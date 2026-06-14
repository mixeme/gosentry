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
- Jobs can be created, edited, paused/resumed, and run manually in memory.
- Job persistence, cron parsing, and process execution are planned for the next phase.
