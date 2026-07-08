# GoSentry — Development

Build instructions, project layout, and dependency information for contributors.

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

### Windows

```powershell
# Builds dist\windows\gosentry-<version>-windows-amd64.exe. The script changes
# to the repository root first, so double-clicking it from Explorer works. It
# also adds MSYS2 UCRT64 to PATH for this process only, embeds the Windows icon
# when windres is available, and uses the Windows GUI subsystem so no console
# window opens at startup.
.\scripts\build-windows.bat
```

The Windows build is created as a GUI application, so it does not open a terminal window.

The binary is written to:

```text
dist\windows\gosentry-0.9.0-windows-amd64.exe
```

### Linux

```bash
# Make the helper executable once, then build a linux/amd64 Fyne binary.
chmod +x ./scripts/build-linux.sh
./scripts/build-linux.sh
```

The binary is written to:

```text
dist/linux/gosentry-0.9.0-linux-amd64
```

### Linux using Docker

```bash
# Builds the Linux binary inside Docker using the versioned image tag
# gitea.mixdep.ru/mix/gosentry-builder:<version>. Useful from hosts or CI jobs
# where the native Linux/Fyne packages are not installed locally.
chmod +x ./scripts/build-linux-docker.sh
./scripts/build-linux-docker.sh
```

The binary is copied to:

```text
dist/linux/gosentry-0.9.0-linux-amd64
```

### Release build from Linux

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
dist/linux/gosentry-0.9.0-linux-amd64
dist/linux/gosentry-0.9.0-linux-arm64
dist/windows/gosentry-0.9.0-windows-amd64.exe
```

### Automated release builds (CI)

Tagged releases are built automatically on both GitHub and Codeberg:

- `.github/workflows/release.yml` — GitHub Actions.
- `.forgejo/workflows/release.yml` — Forgejo Actions (Codeberg).

Both run inside `golang:1.22-bookworm` (the same base image as the
[Dockerfile](../Dockerfile)), install the cross toolchain, and call
`scripts/ci-build-release.sh`, which builds and packages all three artifacts:

```text
dist/linux/gosentry-<version>-linux-amd64.tar.gz
dist/linux/gosentry-<version>-linux-arm64.tar.gz
dist/windows/gosentry-<version>-windows-amd64.zip
```

The Windows binary is cross-compiled with MinGW-w64 from the Linux job, so no
Windows runner is required. Each archive contains the executable plus `README.md`
and `CHANGELOG.md`, matching the local `package-*` scripts.

To cut a release, bump `src/app/version.go` and push a matching `v` tag:

```bash
git tag v0.11.5
git push origin v0.11.5   # and to the Codeberg remote
```

The workflow strips the leading `v` from the tag and injects it as the version,
so the tag must match `version.go`. Pushing the tag triggers the build and
attaches the archives to a release on that forge. `workflow_dispatch` also allows
a manual, publish-free build to smoke-test the pipeline.

Codeberg publishing needs a repository secret named `RELEASE_TOKEN` (a Codeberg
access token with the `write:repository` scope) under
**Settings → Actions → Secrets**. GitHub uses the built-in `GITHUB_TOKEN`.

## Run From Source

Windows:

```powershell
# Fyne requires CGO on Windows. MSYS2 UCRT64 provides the C compiler and native
# libraries used by the desktop backend.
$env:Path = 'C:\msys64\ucrt64\bin;' + $env:Path
$env:CGO_ENABLED = '1'

# go run starts the app from source. Use scripts\build-windows.bat when you need
# a standalone .exe without a console window.
& 'C:\Program Files\Go\bin\go.exe' run ./cmd/gosentry
```

Linux:

```bash
# CGO must stay enabled because the Fyne GUI links against native Linux desktop
# libraries.
CGO_ENABLED=1 go run ./cmd/gosentry
```

## Project Layout

- `cmd/gosentry` — entry point; starts the desktop app.
- `src/domain` — pure value types: `Job`, `Config`, `RunRecord`, `Schedule`, `JobRuntime`.
- `src/app` — `Service`: sole owner of job and runtime state; emits typed events to the UI.
- `src/scheduler` — pure timing loop; calls `Service.RunDue` on every tick.
- `src/runner` — shell command execution, log file writing, and log cleanup.
- `src/storage` — JSON persistence (`gosentry.json`, `jobs.json`).
- `src/platform/autostart` — `Manager` interface with Windows (shortcut) and Linux (XDG) implementations.
- `src/platform/desktop` — display-scale helper (Linux only).
- `src/platform/winproc` — hidden-window startup flags (Windows only).
- `src/ui` — Fyne windows, tabs, and dialogs; reads service state through events.
- `assets` — app icons embedded into the application binary.
- `scripts` — build helpers.
- `docs` — architecture notes, changelog, and roadmap.

Build outputs are written to `dist/`.

## Dependencies

GoSentry keeps the direct dependency list intentionally small:

- [`fyne.io/fyne/v2`](https://fyne.io/) for the native GUI.
- `github.com/robfig/cron/v3` for cron schedule parsing.

The remaining entries in `go.mod` are indirect dependencies pulled by Fyne and the Go module resolver.

Source repositories for mirroring:

- Go toolchain: https://go.googlesource.com/go
- Fyne: https://github.com/fyne-io/fyne
- robfig/cron: https://github.com/robfig/cron

To list every direct and indirect Go module used by the current checkout:

```bash
go list -m all
```
