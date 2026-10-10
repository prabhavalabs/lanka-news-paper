"""Shape text with HarfBuzz (hb-shape) and emit outlined SVG path data via fontTools."""
import json, subprocess, sys
from fontTools.ttLib import TTFont
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.pens.boundsPen import BoundsPen

_cache = {}

def _fix_oo(glyphs, text, font):
    """Some fonts (Gemunu Libre) drop the hal on the aela-pilla of U+0DDD; swap in the .virama glyph."""
    if "\u0DDD" not in text:
        return glyphs
    order = font.getGlyphOrder()
    if "sinMatraAa.virama" not in order:
        return glyphs
    alt = order.index("sinMatraAa.virama")
    raw = text.encode("utf-8")
    cls = sorted({g["cl"] for g in glyphs}) + [len(raw)]
    nxt = {c: cls[i + 1] for i, c in enumerate(cls[:-1])}
    adv = font["hmtx"][ "sinMatraAa.virama"][0]
    for g in glyphs:
        if order[g["g"]] == "sinMatraAa" and "\u0DDD" in raw[g["cl"]:nxt[g["cl"]]].decode("utf-8", "ignore"):
            g["g"] = alt
            g["ax"] = adv
    return glyphs


def outline(text, font_path, size, tracking=0.0, x=0.0, y=0.0, features=""):
    """Return (path_d, width) with baseline at y. tracking is in em units."""
    if font_path not in _cache:
        _cache[font_path] = TTFont(font_path)
    font = _cache[font_path]
    upem = font["head"].unitsPerEm
    gs = font.getGlyphSet()
    order = font.getGlyphOrder()
    cmd = ["hb-shape", "--output-format=json", "--no-glyph-names", font_path, text]
    if features:
        cmd.insert(1, "--features=" + features)
    glyphs = _fix_oo(json.loads(subprocess.check_output(cmd)), text, font)
    scale = size / upem
    pen = SVGPathPen(gs)
    bp = BoundsPen(gs)
    pen_x = 0.0
    track = tracking * upem
    clusters = {g["cl"] for g in glyphs}
    last_cl = None
    for g in glyphs:
        name = order[g["g"]]
        # translate glyph into place, flip y for SVG
        t = (scale, 0, 0, -scale, x + (pen_x + g["dx"]) * scale, y - g["dy"] * scale)
        gs[name].draw(TransformPen(pen, t))
        gs[name].draw(TransformPen(bp, t))
        pen_x += g["ax"]
        if track and g["ax"]:
            pen_x += track
    if track:
        pen_x -= track
    return pen.getCommands(), pen_x * scale, bp.bounds

if __name__ == "__main__":
    d, w, b = outline(sys.argv[1], sys.argv[2], float(sys.argv[3]), float(sys.argv[4]) if len(sys.argv) > 4 else 0)
    print(w, b)


def clusters(text, font_path, size, tracking=0.0):
    """Shape text; return list of (center_x, path_d_relative_to_cluster_center, width) per cluster, and total width."""
    if font_path not in _cache:
        _cache[font_path] = TTFont(font_path)
    font = _cache[font_path]
    upem = font["head"].unitsPerEm
    gs = font.getGlyphSet(); order = font.getGlyphOrder()
    glyphs = _fix_oo(json.loads(subprocess.check_output(["hb-shape", "--output-format=json", "--no-glyph-names", font_path, text])), text, font)
    sc = size / upem
    groups = []
    pen_x = 0.0
    for g in glyphs:
        if not groups or groups[-1]["cl"] != g["cl"]:
            groups.append({"cl": g["cl"], "start": pen_x, "glyphs": [], "adv": 0.0})
        groups[-1]["glyphs"].append((g, pen_x))
        pen_x += g["ax"]
        groups[-1]["adv"] += g["ax"]
        if g["ax"]:
            pen_x += tracking * upem
            groups[-1]["adv"] += tracking * upem
    total = (pen_x - (tracking * upem if tracking else 0)) * sc
    out = []
    for gr in groups:
        c = (gr["start"] + (gr["adv"] - (tracking * upem if tracking else 0)) / 2)
        pen = SVGPathPen(gs)
        for g, px in gr["glyphs"]:
            t = (sc, 0, 0, -sc, (px + g["dx"] - c) * sc, -g["dy"] * sc)
            gs[order[g["g"]]].draw(TransformPen(pen, t))
        out.append((c * sc, pen.getCommands(), gr["adv"] * sc))
    return out, total
