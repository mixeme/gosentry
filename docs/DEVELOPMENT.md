# GoSentry — Development

Toolchain, dependency, build, and release information for contributors.

## Contents

1. [Technology Stack and Tools](#1-technology-stack-and-tools)
   - [Toolchain — Windows](#toolchain--windows)
   - [Toolchain — Linux](#toolchain--linux)
   - [Repository scripts](#repository-scripts)
2. [External Libraries](#2-external-libraries)
3. [Run From Source](#3-run-from-source)
4. [Building the Executable](#4-building-the-executable)
   - [Windows](#windows)
   - [Linux](#linux)
   - [Linux using Docker](#linux-using-docker)
5. [Building a Release](#5-building-a-release)
   - [All targets from Linux](#all-targets-from-linux)
   - [Packaging](#packaging)
6. [CI](#6-ci)
   - [Cutting a release](#cutting-a-release)

## 1. Technology Stack and Tools

GoSentry is a single desktop process written in Go with a Fyne GUI. There is no
server component and no external runtime: the release artifact is one native
executable per platform.

| Layer | Choice |
| --- | --- |
| Language | Go 1.22 or newer |
| GUI toolkit | Fyne v2 (OpenGL desktop backend) |
| Scheduling | `robfig/cron/v3` expression parser |
| Persistence | Plain JSON files (`gosentry.json`, `jobs.json`) |
| Build | `go build` driven by the scripts in `scripts/` |
| Reproducible builds | Docker (`golang:1.22-bookworm` based [Dockerfile](../Dockerfile)) |
| CI | GitHub Actions and Forgejo Actions (Codeberg) |

CGO is mandatory. The Fyne desktop backend links against native OpenGL and
window-system libraries, so a C compiler must be present for every build,
including `go run` and `go test`.

### Toolchain — Windows

- [Go](https://go.dev/) 1.22 or newer.
- MSYS2 with UCRT64 GCC in `C:\msys64\ucrt64\bin` (plus `windres` for the icon
  resource).

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

### Toolchain — Linux

- [Go](https://go.dev/) 1.22 or newer.
- A C compiler.
- [Fyne](https://fyne.io/) native build dependencies, including OpenGL/X11
  development packages.

```bash
# Go builds the application, gcc is required by CGO/Fyne, and the OpenGL/X11
# development packages provide the native desktop headers used by Fyne.
sudo apt install golang gcc libgl1-mesa-dev xorg-dev
```

### Repository scripts

| Script | Purpose |
| --- | --- |
| `scripts/test.bat`, `scripts/test.sh` | `go vet ./...` then `go test -race ./...` |
| `scripts/build-windows.bat` | Windows amd64 executable |
| `scripts/build-linux.sh` | Linux amd64 executable |
| `scripts/build-linux-docker.sh` | Linux amd64 executable, built in Docker |
| `scripts/build-release-linux.sh` | Multi-target release artifacts from one Linux/Docker workflow |
| `scripts/package-windows.bat`, `scripts/package-linux.sh` | Wrap a built binary into a distributable archive |
| `scripts/ci-build-release.sh` | Entry point used by both CI workflows |

Build outputs are written to `dist/`. The package layout is documented in
[ARCHITECTURE.md](ARCHITECTURE.md).

## 2. External Libraries

GoSentry keeps the direct dependency list intentionally small. GoSentry itself
is distributed under the [MIT License](../LICENSE).

| Dependency | Version | Repository | License |
| --- | --- | --- | --- |
| Go toolchain | 1.22+ | https://go.googlesource.com/go | BSD 3-Clause |
| `fyne.io/fyne/v2` | v2.7.4 | https://github.com/fyne-io/fyne | BSD 3-Clause |
| `github.com/robfig/cron/v3` | v3.0.1 | https://github.com/robfig/cron | MIT |

The remaining entries in `go.mod` are indirect dependencies pulled in by Fyne
and the Go module resolver. To list every direct and indirect module used by the
current checkout:

```bash
go list -m all
```

## 3. Run From Source

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

The same environment is required for the test suite — see
[TESTS.md](TESTS.md):

```powershell
scripts\test.bat
```

## 4. Building the Executable

### Windows

```powershell
# Builds dist\windows\gosentry-<version>-windows-amd64.exe. The script changes
# to the repository root first, so double-clicking it from Explorer works. It
# also adds MSYS2 UCRT64 to PATH for this process only, embeds the Windows icon
# when windres is available, and uses the Windows GUI subsystem so no console
# window opens at startup.
.\scripts\build-windows.bat
```

The Windows build is created as a GUI application, so it does not open a
terminal window. The binary is written to:

```text
dist\windows\gosentry-<version>-windows-amd64.exe
```

### Linux

```bash
# Make the helper executable once, then build a linux/amd64 Fyne binary.
chmod +x ./scripts/build-linux.sh
./scripts/build-linux.sh
```

The binary is written to:

```text
dist/linux/gosentry-<version>-linux-amd64
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
dist/linux/gosentry-<version>-linux-amd64
```

## 5. Building a Release

### All targets from Linux

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
dist/linux/gosentry-<version>-linux-amd64
dist/linux/gosentry-<version>-linux-arm64
dist/windows/gosentry-<version>-windows-amd64.exe
```

### Packaging

The `package-*` scripts build the binary for their platform and wrap it in a
distributable archive together with `README.md` and `CHANGELOG.md`:

```powershell
scripts\package-windows.bat
```

```bash
./scripts/package-linux.sh
```

```text
dist/linux/gosentry-<version>-linux-amd64.tar.gz
dist/linux/gosentry-<version>-linux-arm64.tar.gz
dist\windows\gosentry-<version>-windows-amd64.zip
```

The version stamped into the file names and into the binary comes from
`src/app/version.go`.

## 6. CI

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

### Cutting a release

Bump `src/app/version.go`, then create and publish a release with a matching `v`
tag on the forge (GitHub Releases / Codeberg releases). You can do that from the
web UI or the CLI, e.g.:

```bash
git tag v0.11.5
git push origin v0.11.5   # and to the Codeberg remote
gh release create v0.11.5 --generate-notes   # GitHub; publishes the release
```

Publishing the release triggers the workflow: it strips the leading `v` from
the tag and injects it as the version (so the tag must match `version.go`),
builds the archives, and attaches them to that release. `workflow_dispatch`
also allows a manual, upload-free build to smoke-test the pipeline.

Codeberg publishing needs a repository secret named `RELEASE_TOKEN` (a Codeberg
access token with the `write:repository` scope) under
**Settings → Actions → Secrets**. GitHub uses the built-in `GITHUB_TOKEN`.
