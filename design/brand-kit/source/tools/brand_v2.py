"""Thulawa brand kit v2: Stick No Bills name, Gemunu Libre tagline, Bodoni Moda English.

usage: brand_v2.py OUT_DIR
"""
import base64, io, math, os, subprocess, sys
sys.path.insert(0, os.path.dirname(__file__))
from outline import clusters, outline
from PIL import Image

F = os.path.join(os.path.dirname(__file__), "..", "fonts")
NAME_FONT = os.path.join(F, "stick-700.ttf")
TAG_FONT = os.path.join(F, "gemunu-700.ttf")
EN_FONT = os.path.join(F, "bodoni-700.ttf")
CENTRE = os.path.join(os.path.dirname(__file__), "..", "ai", "centre-v1.png")

NAME = "තුලාව"
TAG = "අසමතුලිත ලෝකයකට සමතුලිත පුවත්"
EN = "THULAWA NEWS"
TAG_EN = "Balanced news for an unbalanced world"
P = {"navy": "#13233A", "deep": "#0B1626", "gold": "#C9A24A", "gl": "#E9D08C", "gd": "#94702A", "cream": "#F5ECD8", "cd": "#EADFC4"}
C, R_TEXT, MAXLEN = 500, 411, 1060
TOP_SIZE, BOT_SIZE, TOP_TRACK = 77, 62, 0.12

out = sys.argv[1]
for d in ("logo", "facebook", "web", "wordmarks", "layers"):
    os.makedirs(os.path.join(out, d), exist_ok=True)
centre_b64 = base64.b64encode(open(CENTRE, "rb").read()).decode()


def render(svg, path, w, h=None):
    tmp = path[:-4] + ".svg"
    open(tmp, "w").write(svg)
    cmd = ["rsvg-convert", tmp, "-w", str(w), "-o", path]
    if h:
        cmd += ["-h", str(h)]
    subprocess.check_call(cmd)
    return tmp


def doc(body, w=1000, h=1000):
    return f'<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 {w} {h}" width="{w}" height="{h}">{body}</svg>'


def arc(text, font, size, top, fill, tracking=0.0):
    """Text centred on the ring; ink centre sits on R_TEXT like CSS dominant-baseline=central."""
    _, w, b = outline(text, font, size, tracking)
    if w > MAXLEN:
        size *= MAXLEN / w
    cl, total = clusters(text, font, size, tracking)
    _, _, b = outline(text, font, size, tracking)
    h = -(b[1] + b[3]) / 2  # ink centre height above baseline
    r = R_TEXT - h if top else R_TEXT + h
    outp = []
    for cx, d, _ in cl:
        off = (cx - total / 2) / r
        th = (-math.pi / 2 + off) if top else (math.pi / 2 - off)
        rot = math.degrees(th) + (90 if top else -90)
        x, y = C + r * math.cos(th), C + r * math.sin(th)
        outp.append(f'<path d="{d}" fill="{fill}" transform="translate({x:.2f} {y:.2f}) rotate({rot:.3f})"/>')
    return "".join(outp)


def star(x, y, r, fill):
    pts = []
    for i in range(10):
        a = -math.pi / 2 + i * math.pi / 5
        rr = r if i % 2 == 0 else r * 0.42
        pts.append(f"{x + rr*math.cos(a):.2f},{y + rr*math.sin(a):.2f}")
    return f'<polygon points="{" ".join(pts)}" fill="{fill}"/>'


RING = (f'<path fill-rule="evenodd" fill="{P["navy"]}" d="M10 500a490 490 0 1 0 980 0a490 490 0 1 0-980 0ZM142 500a358 358 0 1 0 716 0a358 358 0 1 0-716 0Z"/>'
        f'<circle cx="500" cy="500" r="474" fill="none" stroke="{P["gold"]}" stroke-width="5"/>'
        f'<circle cx="500" cy="500" r="464" fill="none" stroke="{P["gold"]}" stroke-width="1.5"/>'
        f'<circle cx="500" cy="500" r="353" fill="none" stroke="{P["gold"]}" stroke-width="10"/>'
        + star(90, 500, 16, P["gold"]) + star(910, 500, 16, P["gold"]))


def illus(uid="disc"):
    return (f'<defs><clipPath id="{uid}"><circle cx="500" cy="500" r="348"/></clipPath></defs>'
            f'<g clip-path="url(#{uid})"><rect width="1000" height="1000" fill="{P["cream"]}"/>'
            f'<image href="data:image/png;base64,{centre_b64}" x="186.8" y="186.8" width="626.4" height="626.4"/></g>')


LETTERING = arc(EN, EN_FONT, TOP_SIZE, True, P["cream"], TOP_TRACK) + arc(TAG, TAG_FONT, BOT_SIZE, False, P["gl"])
SEAL = illus() + RING + LETTERING


def seal_at(x, y, size, uid):
    return f'<g transform="translate({x} {y}) scale({size/1000})">{illus(uid)}{RING}{LETTERING}</g>'


def word(text, font, size, x, y, fill, anchor="middle", tracking=0.0):
    """Outlined text; y is the vertical ink centre."""
    _, w, b = outline(text, font, size, tracking)
    ox = {"middle": -(b[0] + b[2]) / 2, "start": -b[0], "end": -b[2]}[anchor]
    oy = -(b[1] + b[3]) / 2
    d, _, _ = outline(text, font, size, tracking, x + ox, y + oy)
    return f'<path d="{d}" fill="{fill}"/>', (b[2] - b[0], b[3] - b[1])


# ---------- logo: seal + layers ----------
files = []
layers = {"layer-1-ring": RING, "layer-2-illustration": illus(), "layer-3-lettering": LETTERING}
for name, body in layers.items():
    files.append(render(doc(body), os.path.join(out, "layers", name + ".png"), 2000))
for size in (2000, 1000, 512):
    files.append(render(doc(SEAL), os.path.join(out, "logo", f"thulawa-seal-{size}.png"), size))
open(os.path.join(out, "logo", "thulawa-seal.svg"), "w").write(doc(SEAL))

# ---------- wordmarks (transparent) ----------
COL = {"gold": P["gold"], "cream": P["cream"], "navy": P["navy"], "gold-light": P["gl"], "gold-dark": P["gd"]}
WM = [("name", NAME, NAME_FONT, 400, 0.0), ("english", EN, EN_FONT, 200, TOP_TRACK), ("tagline", TAG, TAG_FONT, 160, 0.0)]
for key, text, font, size, tr in WM:
    _, w, b = outline(text, font, size, tr)
    pad = size * 0.08
    W, H = b[2] - b[0] + 2 * pad, b[3] - b[1] + 2 * pad
    d, _, _ = outline(text, font, size, tr, pad - b[0], pad - b[1])
    for cn, col in COL.items():
        svgdoc = doc(f'<path d="{d}" fill="{col}"/>', round(W), round(H))
        render(svgdoc, os.path.join(out, "wordmarks", f"{key}-{cn}.png"), round(W) * 2)

# ---------- lockups ----------
def lockup(bg, name_c, en_c, tag_c, rule_c, fname):
    w, h = 1600, 600
    body = f'<rect width="{w}" height="{h}" fill="{bg}"/>' if bg else ""
    body += seal_at(60, 60, 480, "dl" + fname[:3])
    n, (nw, nh) = word(NAME, NAME_FONT, 210, 620, 200, name_c, "start")
    body += n + f'<rect x="624" y="318" width="880" height="4" fill="{rule_c}"/>'
    e, _ = word(EN, EN_FONT, 54, 624, 378, en_c, "start", TOP_TRACK)
    t, _ = word(TAG, TAG_FONT, 52, 624, 452, tag_c, "start")
    body += e + t
    files.append(render(doc(body, w, h), os.path.join(out, "logo", fname), w))

lockup(P["cream"], P["navy"], P["navy"], P["gd"], P["gold"], "lockup-horizontal-cream.png")
lockup(P["deep"], P["gold"], P["cream"], P["gl"], P["gold"], "lockup-horizontal-navy.png")
lockup(None, P["navy"], P["navy"], P["gd"], P["gold"], "lockup-horizontal-transparent.png")

# ---------- backgrounds ----------
def rays(cx, cy, r0, r1, n=96, op=0.07):
    o = []
    for i in range(n):
        a = 2 * math.pi * i / n
        wv = 0.012 if i % 2 == 0 else 0.005
        o.append(f'M{cx + r0*math.cos(a):.1f} {cy + r0*math.sin(a):.1f}L{cx + r1*math.cos(a-wv):.1f} {cy + r1*math.sin(a-wv):.1f}L{cx + r1*math.cos(a+wv):.1f} {cy + r1*math.sin(a+wv):.1f}Z')
    return f'<path d="{"".join(o)}" fill="{P["gold"]}" opacity="{op}"/>'


def navy_bg(w, h, cx, cy):
    return (f'<defs><radialGradient id="bg" cx="{cx/w}" cy="{cy/h}" r="0.9"><stop offset="0" stop-color="#1C3354"/>'
            f'<stop offset="0.55" stop-color="{P["deep"]}"/><stop offset="1" stop-color="#070E19"/></radialGradient></defs>'
            f'<rect width="{w}" height="{h}" fill="url(#bg)"/>' + rays(cx, cy, 0, max(w, h)))


# ---------- Facebook covers 1640x924 (no border) ----------
# Facebook shows the cover at about 2.7:1 (visible band y 158..766) and centres the profile picture over
# the bottom middle: desktop circle ~ (820, 641) r 152, mobile circle ~ (820, 536) r 268.
def cover(variant):
    w, h = 1640, 924
    if variant == "classic":
        # Original layout: name block centre-left, seal on the right.
        body = navy_bg(w, h, 1300, 462) + seal_at(1300 - 230, 462 - 230, 460, "cv1")
        n, _ = word(NAME, NAME_FONT, 230, 640, 345, P["gold"])
        body += n + f'<rect x="330" y="470" width="620" height="3" fill="{P["gold"]}"/>' + star(640, 471.5, 14, P["gold"])
        t, _ = word(TAG, TAG_FONT, 60, 640, 548, P["gl"])
        e, _ = word(EN, EN_FONT, 34, 640, 628, P["cream"], "middle", 0.3)
        return body + t + e
    if variant == "mobile":
        # Centre column (x 550..1090) left clear for the large mobile profile picture.
        body = navy_bg(w, h, 820, 536)
        n, (nw, _) = word(NAME, NAME_FONT, 222, 300, 455, P["gold"])
        RX = 1335
        e, (ew, _) = word(EN, EN_FONT, 36, RX, 392, P["cream"], "middle", 0.25)
        t, (tw, _) = word(TAG, TAG_FONT, 41, RX, 512, P["gl"])
        assert max(nw, tw, ew) <= 490, (nw, tw, ew)
        return (body + n + e + f'<rect x="{RX-220}" y="448" width="440" height="3" fill="{P["gold"]}"/>'
                + star(RX, 449.5, 13, P["gold"]) + t)
    # desktop: centred stack above the smaller desktop profile picture (top of circle ~ y 481)
    body = navy_bg(w, h, 820, 641)
    n, (nw, nh) = word(NAME, NAME_FONT, 160, 820, 250, P["gold"])
    t, (tw, _) = word(TAG, TAG_FONT, 44, 820, 398, P["gl"])
    e, (ew, _) = word(EN, EN_FONT, 26, 820, 452, P["cream"], "middle", 0.3)
    assert 250 - nh / 2 > 165 and 250 + nh / 2 < 340, nh
    return (body + n + f'<rect x="560" y="349" width="520" height="3" fill="{P["gold"]}"/>' + star(820, 350.5, 13, P["gold"]) + t + e)


COVERS = {"classic": "facebook-cover-1-classic-1640x924", "mobile": "facebook-cover-2-mobile-1640x924",
          "desktop": "facebook-cover-3-desktop-1640x924"}
for variant, fname in COVERS.items():
    files.append(render(doc(cover(variant), 1640, 924), os.path.join(out, "facebook", fname + ".png"), 1640))

# ---------- Facebook profile 720 ----------
body = f'<rect width="720" height="720" fill="{P["deep"]}"/>' + seal_at(14, 14, 692, "pf")
files.append(render(doc(body, 720, 720), os.path.join(out, "facebook", "facebook-profile-720.png"), 720))

# ---------- Open Graph 1200x630 ----------
w, h = 1200, 630
body = navy_bg(w, h, 315, 315) + seal_at(65, 65, 500, "og")
n, _ = word(NAME, NAME_FONT, 170, 880, 205, P["gold"])
body += n + f'<rect x="640" y="315" width="480" height="3" fill="{P["gold"]}"/>'
e, _ = word(EN, EN_FONT, 40, 880, 372, P["cream"], "middle", 0.2)
t, _ = word(TAG, TAG_FONT, 44, 880, 445, P["gl"])
body += e + t
files.append(render(doc(body, w, h), os.path.join(out, "web", "og-image-1200x630.png"), w))

# ---------- app icons ----------
def icon(size_px, fname, rounded=True, pad=0.0):
    k = 1000
    rx = 220 if rounded else 0
    s = 1 - 2 * pad
    inner = f'<g transform="translate({500 - 500*s*0.92} {500 - 500*s*0.92}) scale({s*0.92})">{illus("ic"+fname[:4])}<circle cx="500" cy="500" r="353" fill="none" stroke="{P["gold"]}" stroke-width="16"/></g>'
    # illustration disc scaled up so the r=348 disc fills ~82% of the tile
    zoom = 0.82 * 500 / 348 * s
    inner = (f'<g transform="translate({500 - 500*zoom} {500 - 500*zoom}) scale({zoom})">{illus("ic"+fname[:5])}'
             f'<circle cx="500" cy="500" r="352" fill="none" stroke="{P["gold"]}" stroke-width="{14/zoom*1.2:.1f}"/></g>')
    body = f'<rect width="{k}" height="{k}" rx="{rx}" fill="{P["navy"]}"/>' + inner
    files.append(render(doc(body, k, k), os.path.join(out, "web", fname), size_px))

icon(512, "icon-512.png")
icon(192, "icon-192.png")
icon(180, "apple-touch-icon.png", rounded=False)
icon(512, "maskable-512.png", rounded=False, pad=0.1)
icon(64, "favicon-64.png")
# Meta / store app icon: square, full bleed (the store applies its own mask).
icon(1024, "app-icon-1024.png", rounded=False)
im = Image.open(os.path.join(out, "web", "icon-512.png"))
im.save(os.path.join(out, "web", "favicon.ico"), sizes=[(16, 16), (32, 32), (48, 48)])
for s in (16, 32):
    im.resize((s, s), Image.LANCZOS).save(os.path.join(out, "web", f"favicon-{s}.png"))

# ---------- social card masthead (952 x 150, transparent, 2x) ----------
w, h = 952, 150
body = seal_at(0, 3, 144, "cm")
n, _ = word(NAME, NAME_FONT, 92, 162, 75, P["navy"], "start")
e, _ = word(EN, EN_FONT, 24, 952, 52, P["navy"], "end", TOP_TRACK)
t, _ = word(TAG, TAG_FONT, 30, 952, 98, P["gd"], "end")
files.append(render(doc(body + n + e + t, w, h), os.path.join(out, "web", "social-card-masthead.png"), w * 3))

# ---------- Facebook cover safe-area guide ----------
w, h = 1640, 924
guide = (f'<image href="cover.png" width="{w}" height="{h}"/>'
         f'<rect x="0" y="0" width="{w}" height="158" fill="#E2483D" opacity="0.35"/><rect x="0" y="766" width="{w}" height="158" fill="#E2483D" opacity="0.35"/>'
         f'<circle cx="820" cy="536" r="268" fill="none" stroke="#E2483D" stroke-width="6" stroke-dasharray="18 12"/>'
         f'<circle cx="820" cy="641" r="152" fill="none" stroke="#FFB020" stroke-width="6" stroke-dasharray="18 12"/>'
         f'<text x="30" y="100" font-family="Helvetica" font-size="38" fill="#fff">Hidden on desktop and mobile (top ~158 px)</text>'
         f'<text x="30" y="870" font-family="Helvetica" font-size="38" fill="#fff">Hidden (bottom ~158 px) · red circle: mobile profile picture · amber: desktop</text>')
import shutil
shutil.copy(os.path.join(out, "facebook", COVERS["mobile"] + ".png"), os.path.join(out, "facebook", "cover.png"))
files.append(render(doc(guide, w, h), os.path.join(out, "facebook", "facebook-cover-safe-area-guide.png"), w))
os.remove(os.path.join(out, "facebook", "cover.png"))

# ---------- JPG copies for upload ----------
for rel in [f"facebook/{f}.png" for f in COVERS.values()] + ["facebook/facebook-profile-720.png", "web/og-image-1200x630.png"]:
    Image.open(os.path.join(out, rel)).convert("RGB").save(os.path.join(out, rel[:-4] + ".jpg"), quality=92)
# 2048 profile for high-density uploads
render(doc(f'<rect width="720" height="720" fill="{P["deep"]}"/>' + seal_at(14, 14, 692, "pf2"), 720, 720),
       os.path.join(out, "facebook", "facebook-profile-2048.png"), 2048)

for f in files:
    if f.endswith(".svg") and os.path.exists(f):
        os.remove(f)
for f in os.listdir(os.path.join(out, "facebook")):
    if f.endswith(".svg"):
        os.remove(os.path.join(out, "facebook", f))
print("ok")
