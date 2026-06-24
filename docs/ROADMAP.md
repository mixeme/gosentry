# Roadmap

This file tracks planned GoSentry work that is larger than a single bug fix.

## Completed: Release 0.10.0

**Refactoring Follow-Ups (T6.1)**

File-size guidelines have been addressed:
- `src/ui/jobs_view.go` split into `jobs_view.go` + `jobs_toolbar.go` and
  `jobs_details.go` to bring it under the ~250 UI file guideline.
- `src/app/operations_test.go` remains at 536 lines (soft limit); revisit when
  next editing if file size becomes a barrier.

**Post-Field-Test Cleanup (T6.2)**

Stale diagnostics and obsolete compatibility code removed:
- Removed autostart-migration code.
- Cleaned `.gitignore` and `.dockerignore` of YAML import rules.
- Kept startup-timing History instrumentation for future performance tracking.

**Delivery And Packaging (T7.1, T7.2)**

Portable distribution variants complete:
- Windows portable `.zip`: `scripts\package-windows.bat` builds and bundles
  `gosentry.exe`, `README.md`, and `CHANGELOG.md`.
- Linux portable `.tar.gz`: `scripts/package-linux.sh` builds `linux-amd64`
  natively and `linux-arm64` via cross-compilation.

Both formats bundle files at the archive root for direct extraction and use.
