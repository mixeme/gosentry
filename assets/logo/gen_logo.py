#!/usr/bin/env python3
"""Generate GoSentry wordmark logo assets (recommended variant:
Space Grotesk SemiBold + amber 'G' + clock-dial 'o' with hands + 'Sentry')."""
import os
from fontTools.ttLib import TTFont
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.pens.boundsPen import BoundsPen

HERE = os.path.dirname(os.path.abspath(__file__))
OUT  = os.path.join(HERE, "out")
os.makedirs(OUT, exist_ok=True)

AMBER  = "#F7A80C"
PETROL = "#0A4A58"
WHITE  = "#FFFFFF"

LS = -30  # letter-spacing -0.03em at 1000 upm

f = TTFont(os.path.join(HERE, "SpaceGrotesk-600.ttf"))
cmap = f.getBestCmap()
hmtx = f["hmtx"]
gs   = f.getGlyphSet()

def glyph_path(ch, dx):
    """Return SVG path 'd' for ch, shifted by dx in font units (y still up)."""
    g = cmap[ord(ch)]
    pen = SVGPathPen(gs)
    tpen = TransformPen(pen, (1, 0, 0, 1, dx, 0))
    gs[g].draw(tpen)
    return pen.getCommands(), hmtx[g][0]

def o_metrics():
    g = cmap[ord("o")]
    bp = BoundsPen(gs); gs[g].draw(bp)
    xmin,ymin,xmax,ymax = bp.bounds
    return hmtx[g][0], xmin, ymin, xmax, ymax

# ---- layout the wordmark ---------------------------------------------------
x = 0.0
G_d, adv = glyph_path("G", x); x += adv + LS

# dial occupies the 'o' advance slot
o_adv, oxmin, oymin, oxmax, oymax = o_metrics()
o_left = x
cx = o_left + (oxmin + oxmax) / 2.0
cy = (oymin + oymax) / 2.0
R  = ((oxmax - oxmin) + (oymax - oymin)) / 4.0   # avg radius, matches the 'o'
x += o_adv + LS

sentry_d = []
for ch in "Sentry":
    d, adv = glyph_path(ch, x)
    sentry_d.append(d)
    x += adv + LS
x -= LS  # no trailing letter-spacing
SENTRY_D = " ".join(sentry_d)

# ---- dial geometry (matched to the 'o', proportions from the HTML mock) ----
SW   = R * 0.30          # ring stroke width
Rmid = R - SW / 2.0      # centreline radius of ring
HW   = R * 0.21          # hand width
hour_len = R * 0.54      # 12 o'clock hand
min_len  = R * 0.46      # ~4 o'clock hand
min_ang  = 62            # degrees clockwise from 12
import math
mx = cx + min_len * math.sin(math.radians(min_ang))
my = cy + min_len * math.cos(math.radians(min_ang))  # font-up: +y is up
cap = R * 0.14

dial = f'''<circle cx="{cx:.1f}" cy="{cy:.1f}" r="{Rmid:.1f}" fill="none" stroke="{AMBER}" stroke-width="{SW:.1f}"/>
    <path d="M{cx:.1f} {cy:.1f} L{cx:.1f} {cy+hour_len:.1f}" stroke="{AMBER}" stroke-width="{HW:.1f}" stroke-linecap="round"/>
    <path d="M{cx:.1f} {cy:.1f} L{mx:.1f} {my:.1f}" stroke="{AMBER}" stroke-width="{HW:.1f}" stroke-linecap="round"/>
    <circle cx="{cx:.1f}" cy="{cy:.1f}" r="{cap:.1f}" fill="{AMBER}"/>'''

# ---- overall bounds (font units, y up) -------------------------------------
bp = BoundsPen(gs)
xall = 0.0
gG = cmap[ord("G")]; gs[gG].draw(TransformPen(bp,(1,0,0,1,0,0)))
xall += hmtx[gG][0] + LS + o_adv + LS
for ch in "Sentry":
    g = cmap[ord(ch)]
    gs[g].draw(TransformPen(bp,(1,0,0,1,xall,0)))
    xall += hmtx[g][0] + LS
bx0,by0,bx1,by1 = bp.bounds
# include the dial extents
bx0 = min(bx0, cx-R-SW/2); bx1 = max(bx1, cx+R+SW/2)
by0 = min(by0, cy-R-SW/2); by1 = max(by1, cy+R+SW/2)

PAD = 60
W = (bx1 - bx0) + 2*PAD
H = (by1 - by0) + 2*PAD
# transform: font(x,y up) -> screen: translate then flip y
tx = PAD - bx0
ty = PAD + by1
transform = f"matrix(1 0 0 -1 {tx:.2f} {ty:.2f})"

def svg(sentry_color, bg=None, name=""):
    bgrect = f'<rect width="{W:.1f}" height="{H:.1f}" fill="{bg}"/>\n' if bg else ""
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W:.1f} {H:.1f}" role="img" aria-label="GoSentry">
{bgrect}<g transform="{transform}">
    <path d="{G_d}" fill="{AMBER}"/>
    <path d="{SENTRY_D}" fill="{sentry_color}"/>
    {dial}
  </g>
</svg>
'''

variants = {
    "gosentry-logo.svg":       svg(PETROL),                     # light bg, transparent
    "gosentry-logo-dark.svg":  svg(WHITE),                      # dark bg, transparent
    "gosentry-logo-onlight.svg": svg(PETROL, bg="#FFFFFF"),
    "gosentry-logo-ondark.svg":  svg(WHITE,  bg="#04262E"),
    "gosentry-logo-mono.svg":  svg(PETROL).replace(AMBER, PETROL),  # single-colour petrol
}
for fn, data in variants.items():
    with open(os.path.join(OUT, fn), "w", encoding="utf-8") as fh:
        fh.write(data)
    print("wrote", fn)
print("viewBox %.1f x %.1f" % (W, H))
