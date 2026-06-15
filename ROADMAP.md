# Roadmap

This file tracks planned PySentry work that is larger than a single bug fix.

## Delivery And Packaging

Keep a single portable binary as the baseline delivery format. It is simple to
test, easy to copy between machines, and matches the current storage model where
runtime YAML files live next to the executable by default.

Planned delivery variants:

- Windows portable `.zip` with `pysentry.exe`, `README.md`, and `CHANGELOG.md`.
- Linux portable `.tar.gz` archives for `linux-amd64` and `linux-arm64`.
- Debian/Ubuntu `.deb` package once the Linux runtime paths are settled.
- Windows installer later, likely Inno Setup first and MSI/WiX only if needed.
- AppImage as a possible Linux GUI-friendly format after the core workflow is stable.
- Flatpak only after the desktop integration story is clearer.
- winget manifest after stable public Windows releases exist.

Packaging design note:

- Portable builds can keep settings and jobs next to the executable.
- Installer/package builds should move runtime data to per-user locations:
  `%APPDATA%\PySentry` on Windows, and XDG directories such as
  `~/.config/pysentry` and `~/.local/share/pysentry` on Linux.

Initial priority:

1. Windows portable `.zip`.
2. Linux portable `.tar.gz` for amd64 and arm64.
3. Debian/Ubuntu `.deb`.
4. Windows installer.
