# GUI layout review — custom layouts and composition

Findings for the *"GUI review — custom layouts and composition"* item in
[ROADMAP.md](ROADMAP.md). Scope is composition only: the four custom
`fyne.Layout` implementations in [`src/ui/layout.go`](../src/ui/layout.go), the
tuned constants the views drive them with, and how the assembled views behave
when the window or the theme scale changes. It is not a general code review.

Reviewed at Fyne v2.7.4. Every number below was measured with a throwaway
headless probe (`test.NewApp()` + `theme.DefaultTheme()`), not estimated: at the
default text size (14; `theme.Padding()` = 4, `theme.InnerPadding()` = 8) and,
where scale matters, at text size 20. The probes were deleted after the
measurements were taken; the numbers are reproducible from the recipes quoted in
each finding.

Nothing in the code was changed by this pass. Each finding carries a
disposition: **single fix** (goes straight in) or **roadmap** (larger than one
fix, comes back to ROADMAP).

## Summary

The four custom layouts are mostly justified — but one of them
(`compactVBoxLayout`) is a re-implementation of a stock Fyne layout that has
existed since 2.5, and another (`minWidthLayout`) is applied at nine sites of
which eight enforce a width that turns out to have no visual effect at all.

The bigger result is about the constants rather than the layouts. Three raw-pixel
width constants (`settingsLabelWidth`, `settingsControlWidth`,
`minJobsSidebarWidth`) are *floors*, and content already reaches or exceeds all
three at the default text size or one step above it. They no longer shape
anything on screen. What they still do is set the minimum size of the window —
and that minimum (**1165×543** for a typical install) is **wider than the
1024×660 window the app asks for at startup**. GoSentry cannot open at its own
default size, and the user cannot drag it narrower.

## Per-layout verdict

The roadmap asks three questions of each layout: is it still needed, is it the
smallest thing that works, does it hold up at other window sizes and theme
scales.

| Layout | Call sites | Still needed | Smallest thing that works | Holds up when scaled |
|---|---|---|---|---|
| `minWidthLayout` | 9 | Partly — 1 site is load-bearing, 8 are not | No — see F2, F7 | Yes: it takes the max with the child's own minimum, so it can never clip |
| `compactVBoxLayout` | 3 | **No** — stock `layout.NewCustomPaddedVBoxLayout` is identical | No — delete the type (F4) | Yes, but the *spacing values* do not (F5) |
| `fixedHeightLayout` | 1 | **Yes** — keep | Yes (see below) | Yes: its one caller derives the height from the theme |
| `captionValueLayout` | 1 (via `detailRow`, 11 rows) | Yes | Yes | Yes for the caption; the value column has no floor of its own (F9) |

On `fixedHeightLayout`, which the roadmap singles out as a one-call-site type:
there is no stock layout that forces an exact height. The closest stock
construction is `container.NewStack(list, rect)` with a transparent
`canvas.Rectangle` carrying `SetMinSize(fyne.NewSize(0, activityRowsHeight(3)))`,
which is equivalent *here* only because the parent is a `Border` bottom slot and
a bottom slot grants exactly `MinSize().Height`. That is a 29-line type traded
for a one-line invisible-rectangle trick that depends on the parent staying a
`Border`. **Keep the type** — and note the codebase already uses the
invisible-rectangle trick twice in `settings_view.go` (F8), so the two idioms
currently coexist for no reason. Standardise on the named layout.

## Per-constant verdict

| Constant | Value | What content actually needs | Binds? |
|---|---|---|---|
| `settingsLabelWidth` | 180 | 169.2 — widest caption, *"Default overlap policy"* | Only below text size 15 (180.1 at 15). Visible effect: yes, it sets the caption column |
| `settingsControlWidth` | 330 | 306.7 — widest control, the notifications checkbox | Only below text size 16. Visible effect: **none** (F2) |
| `minJobsSidebarWidth` | 400 | 448.0 — the 5-button toolbar row | **Never** (F7) |
| `detailRowSpacing` | −8 | = −`theme.InnerPadding()` at the default theme | Yes — but the relation to the theme is only implicit (F5) |
| `jobRowSpacing` | −8 | as above | as above |
| `settingsRowSpacing` | −6 | no principled relation to any theme metric | as above |
| `logColumnMinWidth` / `MaxWidth` / `Padding` | 240 / 520 / 24 | a real log name measures 268.9 at text size 14, 373.7 at 20 | Yes; the min/max are sane, the 24 is a guess at `2×InnerPadding` = 16 (F14) |
| History column widths | 150/90/170/90/260 | 148.4/75.7/114.9/86.7/144.2 at text size 14 | Yes — Time has 1.6 px of headroom, State 3.3, which a scaled UI eats (F6) |
| `activityRowsHeight()` | derived | — | **Correct by construction — this is the model** |
| `detailCaptionWidth()` | derived | — | **Correct by construction — this is the model** |

## Findings

### F1 — The window cannot open at the size it asks for *(high, roadmap)*

`run.go:54-56` asks for 1024×660 (or the persisted preference). The assembled
content's minimum is **1165.5×542.9** for an install whose config path is 53
characters (`C:\Users\alice\AppData\Roaming\GoSentry\gosentry.json`). Fyne
enforces that minimum in two places: `window.Resize` takes
`size.Max(content.MinSize())`, and `fitContent` calls
`view.SetSizeLimits(minWidth, minHeight, …)`. So the window silently opens ~140 px
wider than requested and cannot be dragged narrower.

The height is fine — 543 leaves room on a 720p screen, which is what the
condensed details pane was for. The problem is entirely horizontal, and it comes
from the Settings tab: `AppTabs` reports the maximum of its children, and the
three tabs measure 920.0 (Jobs), 40.0 (History), **1165.5 (Settings)**.

The Settings minimum is also not a constant — it tracks the length of the config
file path: 1040 for a 10-character path, 1165 for 53, **1501 for 75**. A user
with a long Windows account name gets a window that will not fit on a 1366×768
laptop screen.

`container.NewVScroll` (`settings_view.go:304`) caps the tab's minimum *height*
at 32, and the comment there explains exactly why. The same reasoning was never
applied to the width. Two independent causes, F2 and F3, both need fixing to
bring the floor under 1024; measured together they take it to **993.3**, at which
point the binding row is the *Notifications* checkbox (490.7) and the tab no
longer widens with the config path at all.

This also interacts with the frozen *Window size persistence* roadmap item: a
restored width below the content minimum would be silently widened anyway.

### F2 — `settingsControlWidth` wrappers change nothing but the minimum *(medium, single fix)*

Seven rows wrap their control in `container.New(minWidthLayout{width: 330}, …)`
(`settings_view.go:248, 252, 253, 254, 262, 263, 264`). Measured with a select at
three row widths, wrapped versus bare:

| Row width | Wrapped select size | Bare select size |
|---|---|---|
| 700 | 516 | 516 |
| 514 | 330 | 330 |
| 420 | 236 | 236 |

Identical, at every width, because `settingsRow` puts the control in a `Border`
centre slot, which already stretches it to whatever the column gives it — and
`minWidthLayout.Layout` in turn resizes its child to the container size. The
wrapper's *only* observable effect is on `MinSize`: the row reports 514 instead
of 311.9, ×2 columns, which is the 1040 floor in F1.

**Fix:** delete the constant and the seven wrappers. No visual change at any
window size the user can reach; seven containers fewer; the Settings floor drops
by 46.7 px (the notifications checkbox then binds at 490.7). `minWidthLayout`
itself stays — its remaining caller, the caption box in `settingsRow`, sits in a
`Border` *left* slot, which grants exactly `MinSize().Width`, so there the
constant is what renders.

### F3 — One label in Settings is not truncated, and it sets the window width *(medium, single fix)*

`settingsRow("Config JSON", widget.NewLabel(store.Paths.ConfigPath))`
(`settings_view.go:273`) is the only value label in the tab with default
wrapping, so its minimum width is the full pixel width of the path: the label
measures 392.8 for a 53-character path (12.7 truncated), which makes the row 576.8
against 196.7. Fyne's grid gives every column the widest cell's width, so that row
alone costs **2×** its width in the tab minimum.

The rule this row is missing is already established one screen away —
`autostartStatus.Wrapping = fyne.TextTruncate` at `settings_view.go:128`, with the
comment *"Truncating keeps a long status message from forcing the column wider."*
The read-only path row simply never got it.

**Fix:** truncate it like its neighbour. The full path stays readable whenever the
window is wide enough, which it will be by default.

### F4 — `compactVBoxLayout` re-implements a stock Fyne layout *(medium, single fix)*

`layout.NewCustomPaddedVBoxLayout(padding float32)` has existed since Fyne 2.5
and does exactly what `compactVBoxLayout` does, negative padding included.
Compared directly at spacings −8, −6, 0 and 4, the two produce **identical**
`MinSize` values and identical position and size for every child; they also agree
on a hidden middle child (62.16), an empty object list (0×0) and a single child
(43.25×35.08). The stock layout is a strict superset — it additionally
distributes `layout.Spacer` objects, which the local copy ignores.

**Fix:** delete the type (≈40 lines) and replace its three call sites with
`container.New(layout.NewCustomPaddedVBoxLayout(spacing), …)`. Keep the *comment*
explaining why the spacing is negative — that rationale is not in the stock docs.

### F5 — The negative spacings are magic numbers with a theme expression available *(medium, single fix)*

`detailRowSpacing = -8` and `jobRowSpacing = -8` are exactly
`-theme.InnerPadding()` at the default theme, and the match is not accidental:
two stacked labels contribute 8 px of inner padding each at their shared edge, so
−8 removes one label's worth and leaves the other. Written as −8 the reasoning is
invisible and the value stops tracking a theme that changes
`SizeNameInnerPadding` — the exact class of breakage the roadmap flags.

Text does not currently overlap at either measured scale (line height 19.1 within
a 35.1 label at text size 14; 27.2 within 43.2 at text size 20), so this is a
robustness fix, not a rendering bug.

`settingsRowSpacing = -6` has no such derivation — it is 0.75 of the inner
padding, chosen by eye.

**Fix:** replace the three constants with one function, in the style
`activityRowsHeight` and `detailCaptionWidth` already set:

```go
// rowOverlap pulls stacked label rows together by exactly one label's vertical
// inner padding, which is the whitespace two adjacent labels double up on.
func rowOverlap() float32 { return -theme.InnerPadding() }
```

A function, not a `const`, because `theme.InnerPadding()` must be read after the
app exists. Settings then either adopts the same value (−8, a 2 px change) or
keeps a documented fraction of it.

### F6 — History column widths are raw pixels and truncate on a scaled UI *(medium, single fix)*

`table.SetColumnWidth(0…4, 150/90/170/90/260)` leaves as little as 1.6 px of
headroom at the default text size (Time, holding `2026-06-01 10:00:00`) and 3.3
(State, holding `Succeeded`). At text size 20 three of the five columns truncate
their own content:

| Column | Width | Needs at text size 20 |
|---|---|---|
| Time | 150 | 204.9 (`2026-06-01 10:00:00`) |
| Trigger | 90 | 101.3 (`Schedule`) |
| State | 90 | 117.0 (`Succeeded`) |

The Log column is already immune — `logColumnWidth` measures its content with
`fyne.MeasureText(…, theme.TextSize(), …)`. The remaining five columns should be
sized the same way, from a representative sample string per column (a timestamp,
the longest trigger and state names) rather than from a pixel count.

### F7 — `minJobsSidebarWidth` never binds *(low, single fix)*

The Jobs sidebar's natural minimum is **448.0**, set by the toolbar row of five
buttons; the constant is 400. It has never had an effect at the default theme,
and scaling only widens the gap. The `Border` left slot gives the sidebar exactly
its `MinSize` width, so the wrapper contributes nothing.

**Fix:** delete the constant and the wrapper. One caveat: `jobsSidebar` in
[`jobs_view_test.go`](../src/ui/jobs_view_test.go:141) locates the sidebar by
looking for a container whose layout is `minWidthLayout`, so it needs a different
anchor in the same change. See also F15 — an `HSplit` would remove the question.

### F8 — The settings button row is indented twice as far as its comment claims *(low, single fix)*

`settings_view.go:299-311` builds two transparent `canvas.Rectangle` spacers, the
second to *"[indent] the buttons from the left edge the same amount"* as
`theme.Padding()`. Measured, the first button lands at **x = 8**, not 4: an
`HBox` inserts its own `theme.Padding()` gap *after* the spacer, so the inset is
doubled.

**Fix:** drop both rectangles for the stock
`layout.NewCustomPaddedLayout(2*theme.Padding(), 0, theme.Padding(), 0)` wrapped
around a plain `HBox`. That reproduces the current 12 px top gap exactly (VBox
padding + 2× padding) while giving the intended 4 px left inset — and removes the
codebase's second spacing idiom (see the `fixedHeightLayout` note above). If the
8 px inset is what was actually wanted, the constant should say so instead.

### F9 — The details value column has no floor of its own *(low, roadmap)*

`captionValueLayout` gives the caption a fixed width and hands the remainder to
the value, with no lower bound: `valueWidth = size.Width - captionWidth -
padding`, clamped at 0. Measured across the pane's reachable widths:

| Window width | Caption | Value |
|---|---|---|
| 1583 | 116.2 | 439.3 |
| 1024 | 116.2 | 159.8 |
| 920 (the panel's own minimum) | 116.2 | 107.8 |

So the value never actually vanishes — but only because
`commandOutputScroll.SetMinSize(fyne.NewSize(460, 70))`
([`jobs_view_details.go:62`](../src/ui/jobs_view_details.go:62)) keeps the pane
460 px wide, and that constant exists for an unrelated reason (readable command
output). Lower it and the value column silently starves; feed the layout 240 px
directly and the value renders at width 0 with no warning.

The coupling is invisible in both files. Either give `captionValueLayout` its own
minimum (shrink the caption once the value would drop below some floor, so the
caption truncates first), or record the dependency at both ends.

Related, same type: `MinSize` and `Layout` both `return` silently when
`len(objects) != 2`. A miswired caller renders an empty row rather than failing.
The type is package-private with one constructor (`detailRow`), so this is a
documentation-grade nit, not a defect.

### F10 — The detail caption list is written twice *(low, single fix)*

`detailCaptionWidth()` ([`jobs_view_details.go:165`](../src/ui/jobs_view_details.go:165))
hard-codes the eleven caption strings to measure the widest; `container()` writes
the same eleven strings again, thirty lines above, to build the rows. They match
today. Add a twelfth row and forget the list, and the new caption silently
truncates — with no test to catch it, because the width is correct for the
captions the function knows about.

**Fix:** build the rows from one `[]struct{caption string; value fyne.CanvasObject}`
and derive the width from that same slice.

### F11 — The History table re-sorts its whole backing slice once per cell *(medium, single fix)*

```go
func(id widget.TableCellID, item fyne.CanvasObject) {
    label.SetText(historyCellText(id, sortedEvents()))
```

`sortedEvents()` copies the event slice and `sort.SliceStable`s it — and it is
called from the per-cell update callback. Measured with 300 events in a 1200×800
window: **126 `UpdateCell` calls per `Refresh`, so 126 copies and 126 sorts of a
300-element slice** for one redraw. A redraw runs on every recorded run, every
service error, and every UI action, since `mainwindow.go`'s observer calls
`refresh()` unconditionally.

**Fix:** sort once per refresh into a slice the callback reads. The same callback
also assigns a constant `fyne.TextStyle{}` that the row template already carries,
then calls `label.Refresh()` for that assignment; with the assignment gone the
`Refresh` goes too, because `SetText` already refreshes.

### F12 — Jobs handlers repeat the work `refreshView` is about to do *(low, single fix)*

`refreshView` already calls `syncFromService()`, recomputes `filteredJobs`,
updates the details panel and calls `list.Refresh()`. Six handlers call
`list.Refresh()` immediately before calling `refreshView()`
(`jobs_view.go:238, 256, 270, 302, 315, 347`, plus `181`/`191` in the folder
filter), and the pause handler additionally repeats `syncFromService()`.
`widget.List.Refresh()` re-creates the row template and re-measures the row
height, so this is not free. Where the earlier `syncFromService()` is genuinely
needed — `folderOptions(jobs)` reads the refreshed slice — it should stay, with
only the redundant `list.Refresh()` removed.

### F13 — `settings_view.go` is 445 lines and speaks two dialects *(low, roadmap)*

Past the ~250-line guideline in [ARCHITECTURE.md](ARCHITECTURE.md), and the split
`jobs_view.go` already demonstrates the shape. Three seams are visible in the
file as it stands:

- **`settings_view.go`** — `settingsView`: field construction, save/load/validate.
- **`settings_view_layout.go`** — `settingsSection`, `settingsRow`, the two
  columns, the button row.
- **`settings_view_helpers.go`** — `fyneVersion`, `mustParseURL`,
  `settingsFolderPath`, `openFolder`, the file/folder pickers.

Two composition inconsistencies to settle in the same pass rather than carry
across the split:

- The *Application* and *About* blocks use `settingsSection` (condensed spacing);
  *Queue* and *Storage* inline `container.NewVBox(header, rows…)` (theme
  spacing). Two spellings of "a titled block of rows" in one function. Both
  comments justify the difference, but a `settingsSection(title, spacing, rows…)`
  — or two named constructors — would say it once.
- `chooseFile` and `chooseJSONFile` are the same eight lines apart from
  `SetFilter`. One function taking a filter (`nil` for none) removes the copy.
  Note `chooseFile` is also used by `job_dialog.go`, so it belongs with the
  helpers, not with Settings.

### F14 — `logColumnPadding = 24` *(low, single fix)*

The cell is a `widget.Label`, whose text is inset by `theme.InnerPadding()` on
each side: 16 px, plus table padding. 24 is a hand-tuned guess at that. Express
it as `2*theme.InnerPadding()` (plus whatever margin is wanted, named) so it
tracks the theme like the width it is added to already does. The three constants
are also untyped `int` while every other width constant in the package is a typed
`float32`.

### F15 — Master/detail is a fixed `Border`, not a split *(low, roadmap)*

`jobs_view.go:365-366` pins the sidebar at its `MinSize` in a `Border` left slot,
so the user can never give the details pane more room or the job list less. This
is what `container.NewHSplit` is for: a draggable divider, `SetOffset` for the
initial ratio, and no `minWidthLayout` wrapper or `minJobsSidebarWidth` constant
needed. It changes behaviour rather than just structure, so it is a roadmap item,
not a cleanup — but it subsumes F7 outright, gives F9 a user-controlled escape
(drag the divider left to widen the value column), and is the idiomatic Fyne
composition for this screen.

## What holds up — do not "fix" these

- **`activityRowsHeight()` and `detailCaptionWidth()`.** Both derive their result
  by measuring a real widget under the current theme. They are the pattern
  everything else in the package should converge on, and they already behave
  correctly at text size 20 (114.2→138.7 and 116.2→159.1).
- **`minWidthLayout` degrades safely.** Because `MinSize` takes the *max* of the
  configured width and the child's own minimum, a width routed through this layout
  can never clip its content — it can only inflate a minimum. That is why F1 is a
  sizing problem and not a rendering one, and it is the property the History
  column widths in F6 lack.
- **The compact/detailed row mechanism.** `applyRowMode` expressing the mode as
  visibility, with the comment about `widget.List` caching the template's
  `MinSize`, is correct and non-obvious; both call sites are needed.
- **The negative spacing itself.** It does not overlap text at either measured
  scale. F5 is about how the number is written, not about abandoning the
  technique.
- **`container.NewVScroll` around Settings.** It correctly keeps the tab from
  dictating the window's minimum height. F1 is the same idea left half-applied.

## Suggested order

1. F2 + F3 together — they are the two causes of F1, and neither changes anything
   visible above the minimum window width. Measured result: 1165.5 → **993.3**.
   Verify by asserting the assembled content's `MinSize().Width` stays under the
   1024 default in a test; that is the regression guard F1 has been missing. If
   more headroom is wanted afterwards, deriving `settingsLabelWidth` from the
   widest caption the way `detailCaptionWidth` does buys another 21.7 (→ 971.7).
2. F4, F7, F14 — deletions, no behaviour change. F7 needs the test helper
   re-anchored in the same commit.
3. F11, F12 — redraw cost, self-contained.
4. F5, F8, F10 — the workaround-shaped constants, once the deletions have settled.
5. F6 — needs a per-column sample string decided first.
6. F9, F13, F15 — back to [ROADMAP.md](ROADMAP.md).
