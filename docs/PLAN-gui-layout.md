# Implementation plan — GUI layout cleanup

## Context

[GUI-LAYOUT-REVIEW.md](GUI-LAYOUT-REVIEW.md) recorded fifteen findings against
the `ui` package's custom layouts, tuned constants, and view composition. This
plan turns all fifteen into landable work.

**Part A** (stages 1–6) is the set the review classified as single fixes: each
stage is one commit, none of them changes what the user sees except where the
stage says so. **Part B** (stages 7–8) is the roadmap-sized work — a file split
and a behaviour change. **Stage 9** closes the roadmap item and ships the batch.

Three decisions were open when this plan was written; all three are settled and
folded in below:

- **Scope: everything.** Part A, the `settings_view.go` split (stage 7) and the
  draggable split pane (stage 8) all land, so the roadmap item closes completely
  rather than carrying F9, F13 and F15 forward.
- **The divider position is not persisted** (stage 8). It stays a `ui`-only
  change with the initial ratio computed at build time; no new `Config` field.
- **Row spacing unifies on −8** (stage 2), so `rowOverlap()` is the single
  expression for the whole package.

Finding IDs (F1…F15) refer to the review. One item found while writing this plan
and not in the review is labelled N1.

## Design decisions

These are cross-cutting; settling them once keeps the stages from contradicting
each other.

- **A size that must track the theme is a function, not a `const`.**
  `theme.Padding()` and friends read `fyne.CurrentApp()`, so they cannot be
  evaluated at package init. `activityRowsHeight()` and `detailCaptionWidth()`
  already establish the shape — measure a real widget under the current theme,
  at build time. Every constant this plan replaces converges on that form, and
  Stage 9 writes the rule into [STANDARDS.md](STANDARDS.md).
- **One spacing idiom: named layouts.** The transparent-`canvas.Rectangle`
  spacer disappears (F8). Where an exact size is needed and no stock layout
  expresses it, a named `fyne.Layout` in `layout.go` does — which is why
  `fixedHeightLayout` stays.
- **`minWidthLayout` stays, its callers mostly do not.** The type is sound (it
  takes the max, so it can never clip). Seven of its nine call sites are inert
  (F2) and one never binds (F7); the survivor is the settings caption box, and
  even that stops being a raw pixel count in Stage 6.
- **The assembled window must fit the size the app asks for.** `run.go` opens at
  1024×660; Fyne treats the content minimum as a hard floor. That becomes an
  invariant with a test behind it (Stage 1), not a thing to re-measure by hand.
- **"No visual change" is asserted, not claimed.** Stages 1 and 3 delete widths
  on the grounds that nothing renders differently. Each carries a test that
  measures geometry rather than trusting the argument in this document.
- **No new dependencies.** Everything here is stock Fyne 2.7.4 plus the standard
  library.

## Part A — single fixes

### Stage 1 — bring the window minimum under the default window size (F1, F2, F3)

The headline finding: assembled content measures 1165.5×542.9 against a
requested 1024×660, so Fyne silently widens the window and forbids dragging it
back. Two causes, both in `settings_view.go`.

**Changes**

1. `src/ui/settings_view.go` — **F2**: delete `settingsControlWidth`
   (line 30) and unwrap the seven controls that use it (lines 248, 252, 253,
   254, 262, 263, 264), e.g. `settingsRow("Theme", themeSelect)`. `settingsRow`
   puts the control in a `Border` centre slot, which already stretches it to the
   column width, so the wrapper only ever inflated `MinSize`.
2. `src/ui/settings_view.go:273` — **F3**: give the read-only config-path label
   `Truncation = fyne.TextTruncateClip`, the non-deprecated form established by
   commit 706aa8e. Extract it to a local so the assignment has somewhere to
   live. This is what stops the tab's minimum from tracking the length of the
   user's config path (1040 → 1165 → 1501 for 10-, 53- and 75-character paths).
3. `src/ui/run.go:54-55` — replace the `1024` / `660` literals with
   `defaultWindowWidth` / `defaultWindowHeight` consts so the test in this stage
   asserts against the same numbers the app uses, with a comment recording that
   Fyne enforces the content minimum over them.

**Tests**

- `src/ui/mainwindow_test.go` — `TestMainViewFitsTheDefaultWindowSize`: build
  `newMainView` against a store whose `Paths.ConfigPath` is a deliberately long
  path, then assert `content.MinSize()` is within
  `defaultWindowWidth`×`defaultWindowHeight`. This is the regression guard F1
  never had; the long path is what keeps F3 from silently regressing.
- `src/ui/settings_view_test.go` — `TestSettingsRowStretchesItsControl`: resize a
  `settingsRow` to a width above its minimum and assert the control fills the
  remaining width. That is the property that makes F2's deletion invisible.

**Expected result:** Settings minimum width 1165.5 → 993.3; the binding row
becomes the *Notifications* checkbox, and the tab no longer widens with the
config path.

### Stage 2 — stock layout, theme-derived spacing (F4, F5)

**Changes**

1. `src/ui/layout.go` — delete `compactVBoxLayout` (lines 40–80). Verified
   identical to `layout.NewCustomPaddedVBoxLayout` at spacings −8, −6, 0 and 4,
   for the three-child, hidden-middle-child, single-child and empty cases; the
   stock layout additionally handles `layout.Spacer`, which the copy ignored.
2. `src/ui/jobs_view.go:139`, `src/ui/jobs_view_details.go:124`,
   `src/ui/settings_view.go:323` — switch to
   `container.New(layout.NewCustomPaddedVBoxLayout(rowOverlap()), …)`. Add the
   `fyne.io/fyne/v2/layout` import to the latter two.
3. `src/ui/layout.go` — **F5**: add `rowOverlap()` and delete
   `detailRowSpacing`, `jobRowSpacing` (`jobs_view.go:29,35`) and
   `settingsRowSpacing` (`settings_view.go:36`):

   ```go
   // rowOverlap is the (negative) gap that pulls stacked label rows together by
   // exactly one label's vertical inner padding. Two adjacent labels each inset
   // their text by theme.InnerPadding(), so the whitespace between two lines of
   // text is double what a single row needs; removing one label's worth
   // condenses the block without letting the text lines touch. Derived rather
   // than hard-coded so it follows a theme that changes SizeNameInnerPadding.
   func rowOverlap() float32 { return -theme.InnerPadding() }
   ```

   This carries over the explanation the deleted `compactVBoxLayout` comment
   held, which is the part not documented upstream.

**Decided:** Settings currently uses −6 where the other two use −8; adopting
`rowOverlap()` moves it to −8, a 2 px tightening of the Application and About
blocks, and that is the one deliberate visual change in Part A. Check it in the
running app at step 4 of *Verification*; if it reads too tight, the fallback is a
documented fraction (`rowOverlap() * 0.75`), never a reinstated literal.

**Tests**

- `src/ui/layout_test.go` (new) — `TestRowOverlapMatchesInnerPadding`: assert
  `rowOverlap() == -theme.InnerPadding()` and that it is negative, under the
  default theme and under a theme with a different inner padding.
- Existing `TestJobListViewToggleShrinksRowsAndPersists` already covers that the
  job rows still shrink in compact mode through the replacement layout; no new
  test needed for F4 beyond a green run.

### Stage 3 — delete the sidebar width floor (F7)

**Changes**

1. `src/ui/jobs_view.go:19` — delete `minJobsSidebarWidth`; at line 365 pass
   `sidebar` straight to the `Border` left slot. The toolbar row already needs
   448 against the constant's 400, so it has never bound, and a `Border` left
   slot renders the child at exactly its `MinSize` width either way.
2. `src/ui/jobs_view_test.go:141` — re-anchor `jobsSidebar`, which currently
   finds the sidebar by looking for a `minWidthLayout` container. Anchor it on
   the pane structure instead: the `Border`'s left object is the sidebar
   (`panel.Objects[1]` — `NewBorder` appends slots after the centre object).
   Add the same one-line comment the row-template code uses about `NewBorder`
   ordering, since this is the second place that ordering is relied on. Stage 8
   moves this anchor once more, to the split's `Leading`.

**Tests**

- `src/ui/jobs_view_test.go` — `TestJobsSidebarWidthIsItsContent`: assert the
  sidebar's `MinSize().Width` equals the toolbar row's, i.e. that nothing else
  imposes a floor. The existing `jobsList`/`jobsViewToggle` helpers exercise the
  re-anchored lookup.

### Stage 4 — redraw cost (F11, F12)

**Changes**

1. `src/ui/history_view.go:133-148` — **F11**: hoist the sort out of the per-cell
   callback. Keep a `rows []event` snapshot beside `descending`; a `resort()`
   closure refills it. Call `resort()` when the view is built, from `refresh()`
   (line 179) before `table.Refresh()`, and from the header tap handler
   (line 157) after flipping `descending`. **The length callback must switch
   from `len(*events)` to `len(rows)`** — today it is safe only because each
   cell re-derives the sorted slice from the same source, and a cache breaks
   that agreement. Cells then read `rows` directly.
   Drop the `label.TextStyle = fyne.TextStyle{}` assignment (the template
   already carries the zero value) and the `label.Refresh()` that exists for it,
   since `SetText` refreshes.
2. `src/ui/history_view.go:109-121` — hoist the `headers` slice out of
   `headerText` into a package-level `var historyHeaders = [...]string{…}`; it
   is currently reallocated on every header-cell update.
3. `src/ui/jobs_view.go` — **F12**: delete the `list.Refresh()` calls that
   immediately precede `refreshView()` (lines 238, 256, 270, 302, 315, 347) and
   the duplicate `syncFromService()` at line 314. `refreshView` already does
   both. Keep the `syncFromService()` calls at 228, 253 and 333 — `folderOptions(jobs)`
   reads the refreshed slice on the next line.
   **Careful with line 181:** the folder-filter handler returns early when the
   filter matches nothing, and that path never reaches `refreshView()`, so its
   `list.Refresh()` is load-bearing. Move it into the early-return branch rather
   than deleting it.

**Tests**

- `src/ui/history_view_test.go` — `TestHistorySortToggleKeepsRowsInSync`: build
  the view over N events, flip the sort through the header tap, assert the first
  and last cell text; append an event, call `refresh`, assert the row count and
  the new event's placement in both orders. This is the regression test for the
  cache-versus-length hazard the change introduces (severity medium →
  regression test required by [STANDARDS.md](STANDARDS.md)).
- `src/ui/jobs_view_test.go` — extend an existing button test to assert the
  selection and details survive a handler that lost its `list.Refresh()`.

**Measured motivation:** 300 events in a 1200×800 window produced 126
`UpdateCell` calls per `Refresh`, each copying and sorting the whole 300-element
slice, on every recorded run.

### Stage 5 — content-measured History columns (F6, F14)

The Log column already sizes itself from its content; the other five are pixel
counts with as little as 1.6 px of headroom, and three of them truncate their own
values at text size 20.

**Changes**

1. `src/ui/history_view.go:45-74` — generalise `logColumnWidth` into one helper:

   ```go
   // textColumnWidth returns the width a table column needs to show the widest
   // of the given samples in full, measured under the current theme so it
   // follows text size and DPI, and clamped to [min, max].
   func textColumnWidth(samples []string, min, max float32) float32
   ```

   with `cellPadding()` = `2 * theme.InnerPadding()` replacing the hand-tuned
   `logColumnPadding = 24` (**F14**), and the min/max bounds themselves expressed
   as measured text (`textWidth(strings.Repeat("0", 30))`) rather than raw
   pixels. The three surviving bounds become typed `float32`, matching every
   other width in the package.
2. `src/ui/history_view.go:169-174` — feed each column its samples:
   - **Time** — the timestamp format itself (`2006-01-02 15:04:05` rendered), a
     fixed width; no content scan needed.
   - **Trigger** — the known trigger strings (`Schedule`, `Manual`, `UI`,
     `Unknown`), a closed set.
   - **State** — the known state strings (`Succeeded`, `Failed`, `Started`,
     `Error`, `Jobs loaded`), likewise closed.
   - **Job** and **Detail** — free text: measure the values actually present,
     bounded like the Log column so one long row cannot dominate the table.
3. `refresh` (line 179) recomputes all content-derived columns, not just Log.

**Tests**

- `src/ui/history_view_test.go` — `TestHistoryColumnsFitTheirContent`: for each
  column, at the default text size and at a scaled theme, assert the configured
  width is at least what its widest sample measures. Table-driven, and it fails
  today for Time, Trigger and State at text size 20.
- `TestTextColumnWidthClamps`: below-min, in-range and above-max samples.

**Note:** the trigger and state strings are produced in `app`/`runner`
(`app.StatusText`, the run recorder). The samples live in `ui` next to the
column they size; a comment should point at where the real strings come from so
a new state gets added in both places.

### Stage 6 — one caption-width helper, button row, truncation idiom (F8, F10, N1, F9 nit)

**Changes**

1. `src/ui/layout.go` — **F10**: one helper for both views:

   ```go
   // captionColumnWidth returns the width to reserve for a column of bold
   // captions: the widest of them, measured under the current theme so it tracks
   // text size and DPI instead of a hand-tuned constant.
   func captionColumnWidth(captions ...string) float32
   ```

2. `src/ui/jobs_view_details.go:119-191` — build the metadata rows and their
   width from **one** list instead of two. Today `detailCaptionWidth` hard-codes
   the eleven captions and `container()` writes the same eleven again thirty
   lines away; a twelfth row added to one and not the other silently truncates.

   ```go
   type detailRowSpec struct {
       caption string
       value   fyne.CanvasObject
   }

   func (d *detailsPanel) metadataRows() []detailRowSpec
   ```

   `container()` derives `capW := captionColumnWidth(captions(specs)...)`, then
   walks the slice two at a time into `detailRowPair`, with the odd tail
   (Statistics) falling through to a single `detailRow`. `detailCaptionWidth`
   disappears.
3. `src/ui/settings_view.go:26-29,440-445` — delete `settingsLabelWidth` and
   derive the caption box the same way. Worth 21.7 px per column on top of
   Stage 1 (993.3 → 971.7), and it removes the last raw-pixel width in Settings.
   `settingsRow` gains a `captionWidth float32` first parameter and `settingsView`
   computes it once from the full caption list — exactly how `detailRow` already
   takes the width `container()` measured, rather than re-measuring per row.
4. `src/ui/settings_view.go:299-311` — **F8**: delete both transparent
   `canvas.Rectangle` spacers for

   ```go
   container.New(
       layout.NewCustomPaddedLayout(2*theme.Padding(), 0, theme.Padding(), 0),
       container.NewHBox(saveSettings, cancelSettings, restoreDefaults, settingsStatus),
   )
   ```

   which reproduces the current 12 px top gap exactly while fixing the left
   inset to the 4 px the comment promises — measured, the rectangle version puts
   the first button at x = 8, because `HBox` adds its own padding *after* the
   spacer. The `image/color` and `fyne.io/fyne/v2/canvas` imports go with it.
5. **N1** (not in the review) — `fyne.TextTruncate` as a `Wrapping` value is
   deprecated in Fyne 2.7.4 and commit 706aa8e converted exactly one call site.
   Eight remain: `settings_view.go:128,442`,
   `jobs_view_details.go:67,155,187,195`, `history_view.go:88,139`. Convert them
   to `Truncation = fyne.TextTruncateClip` in one pass. **`jobs_view_details.go:67`
   and `:155` must change together** — the second measures a sample label that
   has to stay identical to the first, which is the whole basis of
   `activityRowsHeight`.
6. `src/ui/layout.go:121-136` — **F9 (nit half)**: `captionValueLayout.MinSize`
   and `Layout` both return silently when `len(objects) != 2`. Document why that
   is acceptable (package-private, one constructor) at the type, so the next
   reader does not have to work it out. The substantive half of F9 is Stage 8.

**Tests**

- `src/ui/jobs_view_test.go` — `TestDetailCaptionWidthCoversEveryCaption`: for
  every spec `metadataRows()` returns, the bold caption measures no wider than
  `captionColumnWidth` returns. This is the guard that makes the single list
  self-enforcing.
- `src/ui/settings_view_test.go` — the same assertion for the settings captions.
- `src/ui/layout_test.go` — `captionColumnWidth` over an empty list, one caption,
  and at two text sizes.

## Part B — larger, but decided

### Stage 7 — split `settings_view.go` (F13)

445 lines against the ~250 guideline in [ARCHITECTURE.md](ARCHITECTURE.md), and
`jobs_view.go` already demonstrates the shape. Stages 1 and 6 remove roughly 25
lines from it, so the split should follow them, not precede them.

**Changes**

- `settings_view.go` — `settingsView`: field construction, save, load, validate.
- `settings_view_layout.go` — `settingsSection`, `settingsRow`, the two columns,
  the button row.
- `settings_view_helpers.go` — `fyneVersion`, `mustParseURL`,
  `settingsFolderPath`, `openFolder`, `chooseFile`/`chooseFolder`.
- Two composition inconsistencies to settle in the same pass rather than carry
  across the split:
  - *Application* and *About* use `settingsSection` (condensed); *Queue* and
    *Storage* inline `container.NewVBox(header, rows…)` (theme spacing). Two
    spellings of "a titled block of rows" in one function. Resolve to
    `settingsSection(title, spacing, rows…)` or two named constructors.
  - `chooseFile` and `chooseJSONFile` differ only by `SetFilter`. One function
    taking a filter (`nil` for none) removes the copy. `chooseFile` is also
    `job_dialog.go`'s, so it belongs with the helpers, not with Settings.
- `docs/ARCHITECTURE.md` — add a file-structure table for `settings_view.go`
  beside the existing `jobs_view.go` one (~line 206).

**Tests:** no new behaviour, so the existing suite is the check. Worth adding
`settings_view_test.go` coverage for the deduplicated picker's filter argument.

### Stage 8 — draggable master/detail split (F15, F9)

`jobs_view.go:365-366` pins the sidebar at its `MinSize` in a `Border` left slot,
so the user can never trade list width for detail width. `container.NewHSplit`
is the idiomatic Fyne answer: a draggable divider, `SetOffset` for the initial
ratio. It gives F9 a user-controlled escape — the details value column bottoms
out at 107.8 px today, kept non-empty only by the unrelated 460 px minimum on the
command-output scroll.

**Decided: the divider position is not persisted.** No `Config` field, no
`domain`/`storage`/`app` changes, no config-compatibility tests — the stage stays
inside `src/ui` and is one commit. A restart reopens at the computed default.
If persistence is wanted later it is an additive `omitempty` field whose zero
value means "compute the default", which is exactly the pattern
[STANDARDS.md](STANDARDS.md) already requires.

**Changes**

1. `src/ui/jobs_view.go:365-366` — replace the `Border` with
   `container.NewHSplit(sidebar, container.NewPadded(dp.container()))`.
2. Initial offset: `SetOffset` takes a ratio, but the sidebar's natural width is
   absolute (448). A fixed ratio is wrong at both ends — 0.44 fits 1024 but hands
   the sidebar 700 px at 1600. Compute it at build time from
   `sidebar.MinSize().Width / defaultWindowWidth` (the const Stage 1 introduces),
   with a comment saying why it is derived rather than a literal.
3. This retires what is left of F7's wrapper. Stage 3 still lands first and on
   its own — Part A has to stand up even if this stage were deferred — which
   costs one extra line in `jobsSidebar`: Stage 3 anchors it on
   `panel.Objects[1]` (the `Border` left slot), this stage moves it to the
   split's `Leading`.

**Tests**

- `src/ui/jobs_view_test.go` — `TestJobsSplitOpensAtTheSidebarWidth`: assert the
  computed offset gives the leading pane at least its content minimum at the
  default window width, so the toolbar is never born clipped.
- Verify manually (Verification step 7) that dragging the divider hard left
  degrades the details pane instead of clipping it: `HSplit` lets either side
  shrink to its content minimum, which for the details pane is the 460 px
  command-output floor.

### Stage 9 — close-out

1. `docs/STANDARDS.md` — add the rule this review established, under *Code
   quality*: **a size that must follow the theme is measured at build time, not
   written as a pixel constant** — `activityRowsHeight`, `captionColumnWidth`,
   `rowOverlap` and `textColumnWidth` as the examples.
2. `docs/ROADMAP.md` — remove the *GUI review — custom layouts and composition*
   item outright. With Part B in scope, nothing from it carries forward.
3. `docs/CHANGELOG.md` — a `## 0.16.0` section. User-visible: the window opens
   at the size it is asked for and can be dragged smaller; the Jobs pane divider
   is draggable; History columns stay readable on a scaled UI; the Settings
   button row is aligned as intended; the details/settings blocks shift by ~2 px.
   The rest is internal and belongs in the commit messages, not here.
4. `src/app/version.go` — `0.15.0` → `0.16.0`.
5. Delete `docs/PLAN-gui-layout.md` and `docs/GUI-LAYOUT-REVIEW.md`, the way
   `docs/plans/per-job-timeout.md` was removed once shipped (2ab5f07) — the
   findings live on in the CHANGELOG and STANDARDS entries, and with the whole
   plan implemented the review has nothing left to hold open.

## Recommended model per stage

Which Claude model to run each stage on. The split is by *judgement density*,
not by diff size: the stages that only move code around are the cheap ones, and
the stages that can break something silently are not.

| Stage | Model | Why |
|---|---|---|
| 1 — window minimum (F2, F3) | Sonnet 5 | Small deletions, but the new geometry test has to assert the right thing. The plan already fixes the numbers, so there is little left to decide. |
| 2 — stock layout, `rowOverlap` (F4, F5) | Sonnet 5 | Mechanical: delete a type, swap three call sites, add two imports. The equivalence it rests on is already measured. |
| 3 — sidebar floor (F7) | Sonnet 5 | One constant and one wrapper out; the only care needed is the `NewBorder` slot ordering in the test helper, which the plan spells out. |
| **4 — redraw cost (F11, F12)** | **Opus 5** | The one stage that can break History silently. Caching the sorted slice forces the length callback off `len(*events)`, and the `list.Refresh()` at `jobs_view.go:181` is load-bearing on an early-return path. Both are easy to get subtly wrong and neither shows up as a compile error. |
| 5 — History columns (F6, F14) | Sonnet 5 | The helper's shape is specified. One cross-package check: the trigger and state sample strings must match what `app`/`runner` actually emit. |
| 6 — caption width, button row, truncation (F8, F10, N1) | Sonnet 5 | Broad but mechanical. The one trap is paired: `jobs_view_details.go:67` and `:155` must change together or `activityRowsHeight` stops mirroring the real row. |
| 7 — split `settings_view.go` (F13) | Sonnet 5 | Pure code movement plus two small dedups. Large diff, low judgement — but re-read the moved file ends for dropped functions. |
| **8 — HSplit (F15, F9)** | **Opus 5** | Behaviour change. The offset derivation and "does the pane degrade or clip when dragged hard left" are judgement calls that a headless test cannot settle. |
| **9 — close-out (docs, version)** | **Opus 5** | CHANGELOG and STANDARDS prose held to the standard the rest of `docs/` sets, which is the expensive part of this repo's doc convention. |

Notes:

- **Escalate on the second failure.** If a stage's tests fail twice for reasons
  the plan did not anticipate, the assumption behind that stage is wrong — move
  it to Opus 5 rather than iterating.
- **Verification is model-independent.** `scripts\test.bat` and the manual GUI
  pass below are the gate regardless of who wrote the diff; a cheaper model does
  not mean a lighter check.
- Haiku 4.5 is deliberately not recommended for any stage here: every one of
  them edits GUI code whose correctness is geometric rather than textual, and
  the cheapest stages are already short enough that the saving is small.
- Fable 5 is left out because its trade-offs are not characterised well enough
  here to recommend it for a specific stage, not because it was judged unfit.

## Verification

Per stage:

```bash
export PATH="/c/msys64/ucrt64/bin:$PATH"; export CGO_ENABLED=1; go build ./... && go vet ./... && go test -race ./...
```

or `scripts\test.bat` from the cgo-enabled PowerShell shell described in
[CLAUDE.md](../CLAUDE.md).

After Part A, run the app (`go run ./cmd/gosentry`) and check the things no
headless test covers:

1. The window opens at 1024×660 and can be dragged **narrower** than it opens —
   the F1 symptom, gone.
2. Settings looks unchanged at the default window width: controls still fill
   their column, captions still align, the config path is readable and clips
   only when the window is genuinely narrow.
3. The Save/Cancel/Defaults row keeps its gap below the separator and sits 4 px
   from the left edge, not 8.
4. Jobs rows and the details metadata block are unchanged in Detailed and in
   Compact; the Application/About blocks in Settings are 2 px tighter (Stage 2's
   one deliberate change).
5. History still sorts both ways from the Time header, new runs still append,
   and columns hold their content.
6. Repeat 2, 4 and 5 with a scaled desktop (or a temporary theme override with a
   larger `SizeNameText`): nothing that used to be a pixel constant should clip.
7. Resize the window to its minimum in both directions and confirm the details
   pane degrades rather than clipping — the check the roadmap item asked for.
