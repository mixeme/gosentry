# GoSentry — instructions for Claude Code

Cross-platform desktop scheduler (Go + Fyne GUI). Single process: GUI,
application service, scheduler, storage, and command runner in one binary.

## Read before changing code

- [docs/STANDARDS.md](docs/STANDARDS.md) — **required.** Code-quality rules and
  the list of intentional behavior. Do not "fix" anything listed there as
  intentional; if a change contradicts it, update the document in the same commit.
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — package contracts and event flow.
- [docs/TESTS.md](docs/TESTS.md) — test layout and conventions.
- [docs/ROADMAP.md](docs/ROADMAP.md) — deliberately out of scope.

## Key rules (full list in STANDARDS.md)

- `src/app.Service` is the sole owner of job and runtime state; the UI reads it
  through typed events, never through shared mutable state.
- User-facing errors go to `dialog.ShowError` or a History event — never a silent
  `return`.
- Pure helpers get a unit test in the same package; fixes of severity ≥ medium get
  a regression test.
- UI view constructors accept an injected `*app.Service`; `app.Open()` is called
  only from `run.go`.
- Off-main-thread widget updates must go through `fyne.Do` (Fyne v2.6.x).

## Build and test

CGO is required — the Fyne GUI links native libraries. On Windows the toolchain
is MSYS2 UCRT64; the default shell environment here has CGO off, so set it
explicitly:

```powershell
$env:Path = 'C:\msys64\ucrt64\bin;' + $env:Path; $env:CGO_ENABLED = '1'
```

Then:

```powershell
scripts\test.bat
```

which runs `go vet ./...` and `go test -race ./...`. Release binaries come from
`scripts\build-windows.bat` / `scripts/build-linux.sh` — see
[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

## Repository conventions

- Commit directly to `main`; do not create feature branches.
- Notable changes get a [docs/CHANGELOG.md](docs/CHANGELOG.md) entry under the
  current version.
- The window/taskbar icon comes from the `gosentry.ico` PE resource — regenerate
  it from the PNGs whenever an icon changes, not just the embedded asset.
