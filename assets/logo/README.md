# GoSentry logo

Recommended wordmark variant from [`../gosentry-logo.html`](../gosentry-logo.html):
**Space Grotesk SemiBold** with an amber **G**, a **clock-dial "o"** (ring + hands),
and a petrol **"Sentry"**. The dial sits exactly in the `o` slot, so the schedule /
"watch" theme lives inside the letters instead of a bolt-on icon.

## Colors

| token  | hex       | use                         |
|--------|-----------|-----------------------------|
| amber  | `#F7A80C` | `G`, dial ring + hands      |
| petrol | `#0A4A58` | `Sentry` (light background)  |
| white  | `#FFFFFF` | `Sentry` (dark background)  |

## Files

Vector (self-contained — glyphs are outlined to paths, no font required):

- `gosentry-logo.svg` — transparent, petrol `Sentry` (for light backgrounds)
- `gosentry-logo-dark.svg` — transparent, white `Sentry` (for dark backgrounds)
- `gosentry-logo-mono.svg` — single-color petrol

Raster (transparent PNG, aspect ≈ 4326×1034 ≈ 4.18:1):

- `gosentry-logo-{256,512,1024,2048}.png` — petrol `Sentry`
- `gosentry-logo-dark-{256,512,1024,2048}.png` — white `Sentry`

## Regenerating

Requires `fonttools` and `matplotlib`, plus the Space Grotesk variable font
(SIL OFL) instanced to weight 600 as `SpaceGrotesk-600.ttf`:

```sh
python gen_logo.py   # writes SVGs into ./out
python raster.py     # writes PNGs into ./out
```

`gen_logo.py` (SVG) and `raster.py` (PNG) share the same layout + dial geometry,
so both outputs stay identical. Space Grotesk is licensed under the SIL Open Font
License; outlining its glyphs into a logo is permitted.
