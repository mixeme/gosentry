#!/usr/bin/env python3
"""Rasterize the GoSentry wordmark to PNG at several widths, reusing the
same font outlines + dial geometry as gen_logo.py (no SVG rasterizer needed)."""
import os, math
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.path import Path
from matplotlib.patches import PathPatch, Circle
from matplotlib.lines import Line2D
from fontTools.ttLib import TTFont
from fontTools.pens.basePen import BasePen
from fontTools.pens.boundsPen import BoundsPen
from fontTools.pens.transformPen import TransformPen

HERE = os.path.dirname(os.path.abspath(__file__))
OUT  = os.path.join(HERE, "out"); os.makedirs(OUT, exist_ok=True)
AMBER, PETROL, WHITE = "#F7A80C", "#0A4A58", "#FFFFFF"
LS = -30

f = TTFont(os.path.join(HERE, "SpaceGrotesk-600.ttf"))
cmap, hmtx, gs = f.getBestCmap(), f["hmtx"], f.getGlyphSet()

class MplPen(BasePen):
    def __init__(self, glyphSet):
        super().__init__(glyphSet); self.v=[]; self.c=[]
    def _moveTo(self,p): self.v.append(p); self.c.append(Path.MOVETO)
    def _lineTo(self,p): self.v.append(p); self.c.append(Path.LINETO)
    def _curveToOne(self,p1,p2,p3):
        self.v += [p1,p2,p3]; self.c += [Path.CURVE4]*3
    def _qCurveToOne(self,p1,p2):
        self.v += [p1,p2]; self.c += [Path.CURVE3]*2
    def _closePath(self):
        self.v.append((0,0)); self.c.append(Path.CLOSEPOLY)

def glyph_mplpath(ch, dx):
    pen = MplPen(gs)
    gs[cmap[ord(ch)]].draw(TransformPen(pen,(1,0,0,1,dx,0)))
    return Path(pen.v, pen.c), hmtx[cmap[ord(ch)]][0]

# layout
x=0.0
paths_amber=[]; paths_petrol=[]
p,adv = glyph_mplpath("G",x); paths_amber.append(p); x+=adv+LS
og=cmap[ord("o")]; bp=BoundsPen(gs); gs[og].draw(bp)
oxmin,oymin,oxmax,oymax=bp.bounds; o_adv=hmtx[og][0]
cx=x+(oxmin+oxmax)/2; cy=(oymin+oymax)/2
R=((oxmax-oxmin)+(oymax-oymin))/4
x+=o_adv+LS
for ch in "Sentry":
    p,adv=glyph_mplpath(ch,x); paths_petrol.append(p); x+=adv+LS
x-=LS

SW=R*0.30; Rmid=R-SW/2; HW=R*0.21
hour_len=R*0.54; min_len=R*0.46; ang=math.radians(62)
mx=cx+min_len*math.sin(ang); my=cy+min_len*math.cos(ang); cap=R*0.14

# bounds
allb=BoundsPen(gs); gs[cmap[ord('G')]].draw(allb)
xx=hmtx[cmap[ord('G')]][0]+LS+o_adv+LS
for ch in "Sentry":
    gs[cmap[ord(ch)]].draw(TransformPen(allb,(1,0,0,1,xx,0))); xx+=hmtx[cmap[ord(ch)]][0]+LS
bx0,by0,bx1,by1=allb.bounds
bx0=min(bx0,cx-R-SW/2); bx1=max(bx1,cx+R+SW/2)
by0=min(by0,cy-R-SW/2); by1=max(by1,cy+R+SW/2)
PAD=60
X0,X1=bx0-PAD,bx1+PAD; Y0,Y1=by0-PAD,by1+PAD
W=X1-X0; H=Y1-Y0

def render(path_png, width_px, sentry_color, bg=None, mono=False):
    dpi=100
    fw=width_px/dpi; fh=fw*H/W
    fig=plt.figure(figsize=(fw,fh),dpi=dpi)
    ax=fig.add_axes([0,0,1,1]); ax.set_xlim(X0,X1); ax.set_ylim(Y0,Y1)
    ax.set_aspect('equal'); ax.axis('off')
    if bg: fig.patch.set_facecolor(bg); ax.set_facecolor(bg)
    else:  fig.patch.set_alpha(0)
    amberc = sentry_color if mono else AMBER
    for p in paths_amber: ax.add_patch(PathPatch(p,facecolor=amberc,edgecolor='none',antialiased=True))
    for p in paths_petrol: ax.add_patch(PathPatch(p,facecolor=sentry_color,edgecolor='none',antialiased=True))
    pt_per_unit = fw/W*72
    ax.add_patch(Circle((cx,cy),Rmid,fill=False,edgecolor=amberc,linewidth=SW*pt_per_unit))
    for (ex,ey) in [(cx,cy+hour_len),(mx,my)]:
        ax.add_line(Line2D([cx,ex],[cy,ey],color=amberc,linewidth=HW*pt_per_unit,
                    solid_capstyle='round'))
    ax.add_patch(Circle((cx,cy),cap,facecolor=amberc,edgecolor='none'))
    fig.savefig(path_png,dpi=dpi,transparent=(bg is None))
    plt.close(fig)
    print("wrote",os.path.basename(path_png))

for w in (256,512,1024,2048):
    render(os.path.join(OUT,f"gosentry-logo-{w}.png"),w,PETROL)
    render(os.path.join(OUT,f"gosentry-logo-dark-{w}.png"),w,WHITE)
render(os.path.join(OUT,"gosentry-logo-onlight-1024.png"),1024,PETROL,bg="#FFFFFF")
render(os.path.join(OUT,"gosentry-logo-ondark-1024.png"),1024,WHITE,bg="#04262E")
render(os.path.join(OUT,"gosentry-logo-mono-1024.png"),1024,PETROL,mono=True)
print(f"aspect {W:.0f}x{H:.0f}")
