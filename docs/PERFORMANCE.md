# Performance Notes

Measured performance findings for GoSentry. Each entry records the method so the
numbers can be reproduced and re-checked after relevant changes.

## Startup Time

### Finding (2026-06-22)

After the Phase 4 refactor, cold startup time (the History "Window shown in …"
metric) increased by **~290 ms**. The increase is caused entirely by the
**Fyne v2.5.3 → v2.6.3 upgrade** (task T4.1), **not** by the Service / domain /
UI restructuring.

### Method

Env-gated phase timers (`GOSENTRY_TIMING`) were added across the startup path
(`Run` in `src/ui/run.go` and `newMainView` in `src/ui/mainwindow.go`) and the
equivalent points in the pre-refactor entry point (`src/gui/app.go` at commit
`c5e0ef9`, the last commit before T4.1). Both were built with the CGO / MSYS2
UCRT64 toolchain and run 5× each; the first run of each is a cold-disk outlier
and is excluded. The timed span (`started` → `w.Show()`) is identical in both
builds, so the comparison is fair.

### Results (warm-run averages)

| Phase (cumulative from start) | Old (Fyne 2.5.3) | New (Fyne 2.6.3) | Δ |
|-------------------------------|------------------|------------------|--------|
| after single-instance check   | ~0.5 ms          | ~0.6 ms          | —      |
| after Fyne app + window + tray | ~277 ms         | ~285 ms          | +8 ms  |
| `app.Open()` done             | +3 ms            | +3 ms            | 0      |
| views built + `svc.Start()`   | +42 ms           | +43 ms           | ~0     |
| after `SetContent`            | ~348 ms          | ~353 ms          | +5 ms  |
| **after `w.Show()` (TOTAL)**  | **~348 ms**      | **~644 ms**      | **+~290 ms** |

### Interpretation

- Everything up to and including `SetContent` costs the same in both versions
  (~350 ms). The refactor-specific code — `app.Open()` (~3 ms) and the new
  `app.Service` plus view construction (~42 ms) — is unchanged, so the
  restructuring added no measurable startup cost.
- The entire regression lands in **`w.Show()`**: ~0 ms under Fyne 2.5.3,
  ~290 ms under 2.6.3. Fyne 2.6 reworked main-thread marshaling (the change that
  introduced `fyne.Do`) and front-loads first-window GL/driver realization into
  the `Show()` call.
- The cost is a fixed, one-time Fyne expense, not a leak in GoSentry code, and
  the upgrade cannot be reverted because `fyne.Do` requires Fyne ≥ 2.6.
- The tray / autostart path (`--start-in-tray`) skips `w.Show()` until the user
  opens the window, so it is unaffected.

### Finding (2026-06-23, Fyne 2.7.4)

**Fyne upgraded from v2.6.3 → v2.7.4** as part of Phase 6 (P6.1).

### Method

Same `GOSENTRY_TIMING` env-gated phase timers as the 2026-06-22 baseline:
`fmt.Fprintf(os.Stderr, ...)` checkpoints added to `src/ui/run.go` across the
startup path, built with the CGO / MSYS2 UCRT64 toolchain and run 5×; run 1
(cold disk) is excluded. Timers were reverted before committing.

### Results (warm-run averages, runs 2–5)

| Phase (cumulative from start) | Fyne 2.6.3 | Fyne 2.7.4 | Δ |
|-------------------------------|------------|------------|---------|
| after single-instance check   | ~0.5 ms    | ~1 ms      | —       |
| after Fyne app + window + tray | ~285 ms   | ~193 ms    | −92 ms  |
| after newMainView             | ~328 ms    | ~249 ms    | −79 ms  |
| after SetContent              | ~353 ms    | ~252 ms    | −101 ms |
| **after w.Show() [TOTAL]**    | **~644 ms**| **~414 ms**| **−230 ms** |

### Interpretation

Fyne 2.7.4 is **~230 ms faster** (~36%) than Fyne 2.6.3 end-to-end. The gain is
spread evenly: Fyne app + window + tray is 92 ms faster, `w.Show()` itself is
~161 ms faster. This confirms Fyne 2.7.0's "Massive performance increases on
rendering" and driver/threading improvements are real and material for GoSentry.
The upgrade is a net win: `fyne.Do` threading (required since 2.6) is retained
and the startup cost is substantially reduced.
