# Implementation plan — compact / detailed job list view

## Context

The Jobs tab sidebar renders every job as a three-line block (bold name, a
metadata line with folder + schedule + command, and a status line) — see the
`widget.NewList` call in [jobs_view.go:108](../src/ui/jobs_view.go). That is
informative but tall: with many jobs the sidebar needs constant scrolling.

Add a second, opt-in rendering of the same list: **Compact** shows one line per
job (name left, status right); **Detailed** is the current three-line block and
stays the default. The choice is switchable from the Jobs tab itself and is
persisted in `gosentry.json`, so it survives a restart like `Config.Theme` does.

## Design decisions

- **Control lives in the Jobs sidebar header** — a toggle button on the same row
  as the existing "Folder" filter, to its right. Switching is one click, next to
  what it affects. Fyne 2.7.4 has no dedicated toggle widget, so the button
  flips its own text and icon on tap, exactly like `stopAllButton`
  ("Disable auto" / "Enable auto") already does in `src/ui/jobs_view.go`. Like
  that button it is labelled with the *action*, not the current state:
  `Compact` + `theme.ListIcon()` while detailed, `Detailed` +
  `theme.ViewFullScreenIcon()` while compact. The toolbar row (New job / Edit /
  Run now / Pause / Delete) is left untouched.
- **Compact row = name + status** on a single line. Same height as one label,
  so per-job health stays visible at a glance.
- **One list, one row template.** `widget.List` caches the row template's
  `MinSize` as `itemMin`; `list.Refresh()` re-creates the template and
  recomputes it (verified in Fyne 2.7.4 `widget/list.go`,
  `listRenderer.Refresh`), so the rows genuinely shrink/grow on a mode switch
  without rebuilding the widget. The template therefore always holds the same
  four objects, and the mode is expressed by `Show()`/`Hide()`. Both
  `compactVBoxLayout` (`src/ui/layout.go`) and Fyne's border layout skip hidden
  children in `MinSize`, so a hidden line contributes no height.

## Changes

### 1. `src/domain/config.go` — persisted preference

- New string type `JobListView` with `JobListViewDetailed = "detailed"` and
  `JobListViewCompact = "compact"`, documented like `Theme` as a UI-only choice.
- New field `JobListView JobListView \`json:"job_list_view,omitempty"\`` on
  `Config`; comment that empty means detailed so pre-existing configs keep the
  current look.
- `DefaultConfig()` returns `JobListViewDetailed`.
- Method `func (v JobListView) IsCompact() bool` — only the exact `"compact"`
  value is compact — so every consumer normalizes unknown/legacy values the
  same way.

### 2. `src/app/operations.go` — save the choice

New `func (s *Service) SetJobListView(view domain.JobListView) error`, modelled
on `SetGlobalPause` (`src/app/operations.go:168`) but deliberately lighter:
normalize a non-`compact` value to `JobListViewDetailed`, take `mu`, return
early if unchanged, set `s.store.Config.JobListView`, call
`s.store.SaveConfig()`, unlock. No `SaveJobs` (no job changed), no event emitted
(this is presentational only — the Jobs view refreshes its own list; an event
would trigger a pointless whole-window refresh). A comment records that
reasoning.

### 3. `src/ui/jobs_view.go` — rendering and the toggle

- Read the initial mode into a `compactList` local:
  `svc.Store().Config.JobListView.IsCompact()`.
- **CreateItem**: build `name` (bold, `Wrapping = fyne.TextTruncate` so a long
  name cannot push the status off the row), `inlineStatus` (compact-only,
  right-hand side), `meta`, and `status`. Put the first two on one line via
  `container.NewBorder(nil, nil, nil, inlineStatus, name)` and keep the outer
  `compactVBoxLayout{spacing: jobRowSpacing}` container with
  `[nameLine, meta, status]`. Apply the mode's visibility here too — this is
  what makes `itemMin` correct for the mode.
- **UpdateItem**: reach `nameLine` through `row.Objects[0].(*fyne.Container)`
  (`NewBorder` appends the border slots after the center object, so
  `Objects[0]` is `name` and `Objects[1]` is `inlineStatus` — worth a comment,
  matching the existing index-based access in this callback). Set all four texts
  from the existing helpers (`app.DisplayFolder`, `app.DisplayInvocation`,
  `app.StatusText`) and **re-apply visibility on every update**: a full
  `Refresh` reuses the already-visible row objects, which were created under the
  old mode, so visibility cannot be left to `CreateItem` alone.
- **View toggle button**: `viewButton := widget.NewButtonWithIcon(…)` built with
  the text/icon for the current mode. Its `OnTapped` flips the mode
  (`nextJobListView`), persists via `svc.SetJobListView` — on error
  `dialog.ShowError` and roll the local mode back without touching the button,
  per the error rule in `docs/STANDARDS.md` — then applies the new
  `SetText`/`SetIcon` and calls `list.Refresh()`. This mirrors `stopAllButton`'s
  flip-and-revert handler in the same file. The list selection and the details
  panel are untouched by the switch.
- **Header layout**: keep the `"Folder"` caption and `folderSelect` exactly as
  they are, and put the button beside the select on the same row —
  `container.NewBorder(nil, nil, nil, viewButton, folderSelect)`. The border
  layout gives the button its `MinSize` on the right and lets the select fill
  the rest, so the header gains no height and the toolbar row is not touched.

### 4. `src/ui/jobs_view_helpers.go` — button state helpers

Two pure helpers so the on-disk strings never reach the user and the button's
wording is testable without a running GUI:

- `nextJobListView(current domain.JobListView) domain.JobListView` — flips the
  mode, treating anything that is not `compact` as detailed (via `IsCompact`).
- `viewToggleText(current domain.JobListView) string` — the action label:
  `"Compact"` while detailed, `"Detailed"` while compact.

The icon choice stays inline at the button, next to `SetText`/`SetIcon`, so the
helpers file keeps its "no widget imports" character.

### 5. Tests

- `src/domain/config_test.go` (new): `JobListView.IsCompact` for `"compact"`,
  `"detailed"`, `""`, and a junk value.
- `src/ui/jobs_view_test.go`: `nextJobListView` flips both ways and maps an
  empty/unknown value to compact (since such a value reads as detailed), and
  `viewToggleText` returns the action label for each mode.
- `src/app/operations_test.go`: `TestSetJobListViewPersistsToConfigFile`
  modelled on `TestSetGlobalPausePersistsToConfigFile`
  (`src/app/operations_test.go:553`) — switch to compact, unmarshal
  `svc.store.Paths.ConfigPath`, assert the field, switch back; plus a case
  asserting an unknown value is stored as `detailed`.
- `src/storage/store_test.go`: one added assertion in the defaults test beside
  the existing `Theme` check (~line 174) that a fresh config is `detailed`.

### 6. Docs and version

- `docs/CHANGELOG.md`: new `## 0.14.0 - 2026-07-26` section (a feature → minor
  bump) covering the compact/detailed toggle and the new `job_list_view` field,
  noting legacy configs stay detailed.
- `src/app/version.go`: `0.13.0` → `0.14.0`.
- `docs/ARCHITECTURE.md`: add the two new helpers to the `jobs_view_helpers.go`
  row of the file-split table (~line 195).

## Verification

1. Build, vet, and test the whole module through the cgo-enabled shell (the
   default Bash env has CGO off, so GUI packages will not compile there):

```bash
export PATH="/c/msys64/ucrt64/bin:$PATH"; export CGO_ENABLED=1; go build ./... && go vet ./... && go test -race ./...
```

2. Run the app (`go run ./cmd/gosentry`) with several jobs configured and check:
   - Default launch is Detailed and looks exactly as before; the button reads
     "Compact" and sits to the right of the folder filter without making the
     header taller.
   - Tapping it collapses every row to one line, name left / status right, many
     more jobs fit without scrolling, and the button now reads "Detailed".
   - Selection and the details panel keep working after switching, in both
     directions, including with a folder filter active and with an empty list.
   - A running job's status still updates live in compact rows.
   - `gosentry.json` gains `"job_list_view": "compact"`; restarting reopens in
     compact, and switching back removes/flips the field.
