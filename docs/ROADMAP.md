# Roadmap

This file tracks planned GoSentry work that is larger than a single bug fix.
Completed work is recorded in [CHANGELOG.md](CHANGELOG.md), not here.

## Open Items

### History tab — column filters (Trigger / Job / State)

Add dropdown filters above the History table so the user can narrow rows by
trigger source, job name, or run state. Blocked on Fyne native support: the
current `widget.Table` has no built-in filter API, and a filter bar built from
`widget.Select` widgets above the table feels visually out-of-place. Revisit
when Fyne adds first-class column filtering or a composable data-grid widget.
