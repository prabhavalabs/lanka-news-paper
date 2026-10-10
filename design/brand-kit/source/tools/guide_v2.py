"""Thulawa brand guidelines (v2) as HTML -> PDF. usage: guide_v2.py KIT_DIR OUT_PDF"""
import os, subprocess, sys
kit, pdf = os.path.abspath(sys.argv[1]), os.path.abspath(sys.argv[2])
F = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "fonts")
K = lambda p: "file://" + os.path.join(kit, p)
NAVY, DEEP, GOLD, GL, GD, CREAM, CD = "#13233A", "#0B1626", "#C9A24A", "#E9D08C", "#94702A", "#F5ECD8", "#EADFC4"

def page(body, bg=CREAM, cls=""):
    return f'<section class="pg {cls}" style="background:{bg}">{body}</section>'

def head(n, title, sub=""):
    return f'<div class="hd"><span class="no">{n:02d}</span><h2>{title}</h2>{f"<p class=sub>{sub}</p>" if sub else ""}</div>'

def foot(dark=False):
    c = "rgba(245,236,216,.6)" if dark else "#94702A"
    return f'<div class="ft" style="color:{c}">Thulawa News · Brand guidelines · v2 · October 2026</div>'

sw = lambda hexv, name, use, txt="#fff", border="": f'<div class="sw"><div class="chip" style="background:{hexv};color:{txt};{border}">{hexv}</div><b>{name}</b><span>RGB {int(hexv[1:3],16)} {int(hexv[3:5],16)} {int(hexv[5:7],16)}</span><span>{use}</span></div>'

pages = []
pages.append(page(f'''
<img src="{K("logo/thulawa-seal-1000.png")}" style="position:absolute;right:90px;top:110px;width:500px">
<div style="position:absolute;left:90px;top:170px;width:560px">
<img src="{K("wordmarks/name-gold.svg")}" style="height:150px">
<div style="height:3px;background:{GOLD};width:460px;margin:30px 0 26px"></div>
<img src="{K("wordmarks/tagline-gold-light.svg")}" style="height:36px"><br>
<img src="{K("wordmarks/english-cream.svg")}" style="height:22px;margin-top:22px">
<p style="color:{GL};font:500 20px Inter,Helvetica;letter-spacing:.2em;margin-top:70px">BRAND GUIDELINES</p></div>''' + foot(True), DEEP))

pages.append(page(head(1, "The idea", "A scale that weighs every side before the reader decides.") + f'''
<div class="cols"><div>
<h3>Name</h3><p><b class="si">තුලාව</b> (Thulawa) means <i>the scale</i>, the instrument that weighs. The brand promise is to put many reports of the same story side by side, show how each one leans, and let the reader judge.</p>
<h3>Tagline</h3><img src="{K("wordmarks/tagline-navy.svg")}" style="height:34px;margin:6px 0 10px"><p><i>Balanced news for an unbalanced world.</i> The words තුලාව and සමතුලිත share the same root, so the tagline echoes the name.</p>
<h3>Voice</h3><p>Calm, exact, fair. Report what each source says and how; avoid loaded words and never take a side in our own copy.</p>
</div><div>
<h3>The seal</h3><p>An engraved crest: a fountain pen forms the post of a balance, with two equal pans, a sunburst, and a wreath of Sri Lankan paddy. The pen stands for journalism, the level beam for balance, the paddy for Sri Lanka.</p>
<img src="{K("logo/thulawa-seal-512.png")}" style="width:300px;margin-top:16px">
</div></div>''' + foot()))

pages.append(page(head(2, "Logo: the seal", "Primary mark. Built from three layers so it can be edited without redrawing.") + f'''
<div class="row" style="gap:40px;align-items:center">
<img src="{K("logo/thulawa-seal-1000.png")}" style="width:330px">
<div class="row" style="gap:18px">
<figure><div class="ck"><img src="{K("layers/layer-1-ring.png")}"></div><figcaption>1 · Ring</figcaption></figure>
<figure><div class="ck"><img src="{K("layers/layer-2-illustration.png")}"></div><figcaption>2 · Illustration (AI-assisted)</figcaption></figure>
<figure><div class="ck"><img src="{K("layers/layer-3-lettering.png")}"></div><figcaption>3 · Lettering</figcaption></figure>
</div></div>
<div class="cols" style="margin-top:26px"><div><h3>Clear space</h3><p>Keep empty space around the seal at least equal to the width of the navy ring (about 13% of the diameter).</p></div>
<div><h3>Minimum size</h3><p>Full seal: 96 px / 25 mm wide. Below that, use the app icon (illustration only) instead.</p></div></div>''' + foot()))

pages.append(page(head(3, "Lockups and wordmarks", "Use the supplied artwork. Never retype the name or tagline.") + f'''
<img src="{K("logo/lockup-horizontal-cream.png")}" style="width:540px;border:1px solid {CD}">
<img src="{K("logo/lockup-horizontal-navy.png")}" style="width:540px;margin-left:20px">
<div class="row" style="margin-top:22px;gap:50px;align-items:flex-end">
<div><img src="{K("wordmarks/name-navy.svg")}" style="height:84px"><p class="cap">Name · Stick No Bills 700</p></div>
<div><img src="{K("wordmarks/english-navy.svg")}" style="height:30px"><p class="cap">English · Bodoni Moda 700, +12% tracking</p>
<img src="{K("wordmarks/tagline-gold-dark.svg")}" style="height:32px;margin-top:16px"><p class="cap">Tagline · Gemunu Libre 700</p></div></div>
<p class="note">Each wordmark ships in navy, gold, light gold, dark gold and cream as SVG and PNG (folder <b>wordmarks/</b>).</p>''' + foot()))

pages.append(page(head(4, "Colour", "Navy and gold carry the brand. Cream is the light ground.") + f'''
<div class="row" style="gap:22px;margin-top:10px">
{sw(NAVY,"Navy","Ring, headlines on light, UI primary")}
{sw(DEEP,"Deep navy","Dark backgrounds, covers")}
{sw(GOLD,"Gold","Name, rules, accents",DEEP)}
{sw(GL,"Light gold","Tagline on navy",DEEP)}
{sw(GD,"Dark gold","Tagline and accents on cream")}
{sw(CREAM,"Cream","Light backgrounds, seal centre",NAVY,"border:1px solid #d8cfb8")}
</div>
<div class="cols" style="margin-top:30px"><div><h3>Pairings</h3><p>Gold or cream on navy. Navy or dark gold on cream. Do not put gold text on cream (too little contrast) or navy on deep navy.</p></div>
<div><h3>Website reading colours</h3><p>Long reading text stays on the existing paper <b>#F7F5F0</b> with ink <b>#141414</b>. Brand navy and gold are used for the masthead, links, buttons and accents. Bias colours stay as before: left <b>#B96D70</b>, right <b>#587EAD</b>, breaking <b>#A52B26</b>.</p></div></div>''' + foot()))

pages.append(page(head(5, "Typography", "Three display faces for the identity, plus Noto for reading.") + f'''
<div class="type"><div class="tl">Name</div><div style="font:700 66px 'Stick No Bills';color:{NAVY}">තුලාව</div><div class="tm">Stick No Bills · Bold 700<br>Logo name only, never body text.</div></div>
<div class="type"><div class="tl">Tagline / Sinhala display</div><img src="{K("wordmarks/tagline-navy.svg")}" style="height:34px"><div class="tm">Gemunu Libre · Bold 700<br>Taglines and short Sinhala headings.</div></div>
<div class="type"><div class="tl">English</div><div style="font:700 36px 'Bodoni Moda';letter-spacing:.12em;color:{NAVY}">THULAWA NEWS</div><div class="tm">Bodoni Moda · Bold 700, +12% tracking<br>English name and English headings.</div></div>
<div class="type"><div class="tl">Reading</div><div style="font:500 26px 'NotoSi';color:#141414">පුවත් කියවීමට පැහැදිලි අකුරු</div><div class="tm">Noto Serif Sinhala / Noto Sans Sinhala<br>Article text, UI and long copy.</div></div>
<p class="note"><b>Known font issue:</b> Gemunu Libre draws the vowel ෝ without its hal (ලෝ looks like ලො). The supplied tagline artwork is corrected. When typing Gemunu text live (web or Canva), check words with ෝ or use the artwork.</p>
<p class="note">All fonts are free under the SIL Open Font License from Google Fonts.</p>''' + foot()))

pages.append(page(head(6, "Facebook page", "Upload-ready files are in the Facebook kit.") + f'''
<div class="row" style="gap:30px">
<div><img src="{K("facebook/facebook-cover-safe-area-guide.png")}" style="width:470px"><p class="cap">Cover 1640 × 924. Facebook shows only the middle band and centres the profile picture over it, so the centre is left clear.</p></div>
<div><img src="{K("facebook/facebook-profile-720.png")}" style="width:200px;border-radius:50%"><p class="cap">Profile 720 × 720 (also 2048). Shown as a circle.</p></div></div>
<table><tr><th>Asset</th><th>File</th><th>Size</th></tr>
<tr><td>Cover photo</td><td>facebook-cover-2-mobile (recommended), -3-desktop, -1-classic · .jpg / .png</td><td>1640 × 924</td></tr>
<tr><td>Profile picture</td><td>facebook-profile-720.jpg, facebook-profile-2048.png</td><td>720 / 2048 square</td></tr>
<tr><td>Post card</td><td>Generated by the admin Social publishing page</td><td>1080 × 1350</td></tr>
<tr><td>Link preview</td><td>og-image-1200x630.jpg</td><td>1200 × 630</td></tr></table>''' + foot()))

pages.append(page(head(7, "Web and app icons", "The icon uses the illustration only; the full seal is too detailed below 96 px.") + f'''
<div class="row" style="gap:34px;align-items:flex-end">
<figure><img src="{K("web/icon-512.png")}" style="width:160px"><figcaption>icon-512 · icon-192</figcaption></figure>
<figure><img src="{K("web/maskable-512.png")}" style="width:160px"><figcaption>maskable-512 (Android)</figcaption></figure>
<figure><img src="{K("web/apple-touch-icon.png")}" style="width:100px"><figcaption>apple-touch-icon 180</figcaption></figure>
<figure><img src="{K("web/favicon-64.png")}" style="width:64px"> <img src="{K("web/favicon-32.png")}" style="width:32px"> <img src="{K("web/favicon-16.png")}" style="width:16px"><figcaption>favicon.ico 16/32/48</figcaption></figure></div>
<div class="row" style="gap:30px;margin-top:22px"><img src="{K("web/og-image-1200x630.png")}" style="width:440px;align-self:flex-start"><div><h3>Link preview</h3><p>og-image-1200x630 is used whenever a Thulawa link is shared on Facebook, WhatsApp, X or Viber.</p><h3>Where the files live</h3><p>Website: <b>apps/web/public/brand/</b><br>Admin: <b>apps/admin/public/brand/</b><br>Master kit: <b>design/brand-kit/</b></p></div></div>''' + foot()))

pages.append(page(head(8, "Do and don't") + f'''
<div class="cols"><div><h3 style="color:#2E7D4F">Do</h3><ul>
<li>Use the supplied seal, wordmarks and lockups.</li><li>Place the seal on navy, deep navy or cream.</li><li>Keep clear space equal to the ring width.</li><li>Use the icon (no lettering) at small sizes.</li><li>Keep the tagline exactly: අසමතුලිත ලෝකයකට සමතුලිත පුවත්</li></ul></div>
<div><h3 style="color:#A52B26">Don't</h3><ul>
<li>Retype the name or tagline in another font.</li><li>Recolour, stretch, rotate or add effects to the seal.</li><li>Put the seal on busy photos without a navy or cream backing.</li><li>Put gold text on cream, or navy on deep navy.</li><li>Use the old names ලංකා පුවත් or Lanka News Paper.</li></ul></div></div>''' + foot()))

css = f"""
@font-face{{font-family:'Stick No Bills';src:url(file://{F}/stick-700.ttf);font-weight:700}}
@font-face{{font-family:'Bodoni Moda';src:url(file://{F}/bodoni-700.ttf);font-weight:700}}
@font-face{{font-family:'NotoSi';src:url(file://{F}/noto-sinhala-500.ttf);font-weight:500}}
@page{{size:1280px 720px;margin:0}}*{{box-sizing:border-box}}
body{{margin:0;font:16px/1.55 Helvetica,'NotoSi',sans-serif;color:#1d2533}}
.pg{{width:1280px;height:720px;position:relative;overflow:hidden;padding:44px 80px;page-break-after:always}}
.hd{{margin-bottom:18px}}.no{{font:700 15px Helvetica;color:{GD};letter-spacing:.2em}}h2{{font:700 40px 'Bodoni Moda',serif;color:{NAVY};margin:4px 0 6px}}.sub{{margin:0;color:#6b6f78;font-size:17px}}
h3{{font:700 15px Helvetica;letter-spacing:.14em;text-transform:uppercase;color:{GD};margin:18px 0 6px}}
.cols{{display:grid;grid-template-columns:1fr 1fr;gap:56px}}.row{{display:flex}}.si{{font-family:'NotoSi'}}
.ck{{width:170px;height:170px;background:repeating-conic-gradient(#e9e4d8 0 25%,#fbf8f1 0 50%) 0 0/20px 20px}}.ck img{{width:170px}}
figure{{margin:0}}figcaption,.cap{{font-size:13px;color:#6b6f78;margin-top:8px}}
.note{{font-size:14px;color:#4a4a4a;background:#fffaf0;border-left:3px solid {GOLD};padding:10px 14px;margin-top:18px}}
.sw{{width:170px;font-size:13px;display:flex;flex-direction:column;gap:2px}}.sw .chip{{height:150px;display:flex;align-items:flex-end;padding:10px;font:700 14px Helvetica;margin-bottom:8px}}.sw b{{font-size:15px}}.sw span{{color:#6b6f78}}
.type{{display:grid;grid-template-columns:200px 1fr 330px;align-items:center;border-top:1px solid {CD};padding:6px 0}}.tl{{font:700 12px Helvetica;letter-spacing:.14em;text-transform:uppercase;color:{GD}}}.tm{{font-size:13px;color:#6b6f78}}
table{{border-collapse:collapse;margin-top:8px;font-size:13px;width:100%}}th,td{{text-align:left;padding:4px 10px;border-bottom:1px solid {CD}}}th{{color:{GD};font-size:12px;letter-spacing:.1em;text-transform:uppercase}}
ul{{padding-left:20px;font-size:17px;line-height:1.9}}
.ft{{position:absolute;left:80px;bottom:16px;font-size:12px;letter-spacing:.08em}}
"""
html = f"<!doctype html><html><head><meta charset=utf-8><style>{css}</style></head><body>{''.join(pages)}</body></html>"
src = pdf[:-4] + ".html"
open(src, "w").write(html)
subprocess.check_call(["/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "--headless=new", "--disable-gpu",
    "--allow-file-access-from-files", "--no-pdf-header-footer", "--virtual-time-budget=8000", f"--print-to-pdf={pdf}", "file://" + src],
    stderr=subprocess.DEVNULL)
os.remove(src)
print("ok")
