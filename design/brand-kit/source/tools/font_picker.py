"""Write the Sinhala/English font + tagline picker page with a live seal preview."""
import json, sys
centre = open("/tmp/brandwork/canva/centre640.b64").read()
out = sys.argv[1]

SI_FONTS = [
    ("Abhaya Libre", "Serif", [400, 500, 600, 700, 800], "Current logo font. Classic book serif, sharp contrast; feels like a printed newspaper."),
    ("Noto Serif Sinhala", "Serif", [300, 400, 500, 600, 700, 800, 900], "Even, modern serif from Google. Very readable, wide weight range."),
    ("Maname", "Display", [400], "Ornamental, calligraphic strokes. Distinctive for a logo; one weight only."),
    ("Noto Sans Sinhala", "Sans", [300, 400, 500, 600, 700, 800, 900], "Clean, neutral sans. The safest choice for UI and small text."),
    ("Yaldevi", "Sans", [300, 400, 500, 600, 700], "Rounded, friendly sans with open counters. Approachable, digital-first."),
    ("Gemunu Libre", "Sans", [300, 400, 500, 600, 700, 800], "Tall, condensed headline style. Strong and urgent, like a banner."),
    ("Stick No Bills", "Display", [300, 400, 500, 600, 700, 800], "Stencil poster style. Loud; risky for a serious news brand."),
    ("Google Sans", "Sans", [400, 500, 600, 700], "Google's geometric sans with Sinhala. Contemporary, tech-product look."),
]
EN_FONTS = [
    ("Cinzel", "Serif caps", [400, 500, 600, 700, 800, 900], "Roman inscription capitals. The classic seal and crest look."),
    ("Abhaya Libre", "Serif", [400, 500, 600, 700, 800], "Current ring font. Matches the Sinhala name if you keep Abhaya."),
    ("Cormorant SC", "Serif caps", [500, 600, 700], "Elegant small caps with fine detail. Refined, literary."),
    ("Spectral SC", "Serif caps", [400, 500, 600, 700, 800], "Sturdy book small caps. Serious and readable at small sizes."),
    ("Playfair Display", "Serif", [400, 500, 600, 700, 800, 900], "High-contrast newspaper display serif. Very editorial."),
    ("Bodoni Moda", "Serif", [400, 500, 600, 700, 800, 900], "Classic Bodoni, sharp hairlines. Fashion-magazine elegance."),
    ("Libre Baskerville", "Serif", [400, 700], "Traditional Baskerville. Trustworthy, old-press feel."),
    ("EB Garamond", "Serif", [400, 500, 600, 700, 800], "Timeless Garamond. Calm and literary."),
    ("Marcellus", "Serif caps", [400], "Flared Roman capitals, lighter than Cinzel. Dignified."),
    ("Montserrat", "Sans", [400, 500, 600, 700, 800], "Geometric sans. Modern, clean, a little corporate."),
    ("Josefin Sans", "Sans", [400, 600, 700], "Vintage geometric sans with tall caps. Stylish and light."),
    ("Oswald", "Sans", [400, 500, 600, 700], "Condensed sans, headline energy. Pairs with Gemunu Libre."),
    ("Inter", "Sans", [400, 500, 600, 700, 800], "Neutral interface sans. Plain and very legible."),
]
TAGLINES = [
    ("පැත්තක් නෑ. පැති ඔක්කොම.", "No side. Every side.", "Both sides",
     "Short, spoken Sinhala, easy to remember. Says we don't take a side, we show them all.", True),
    ("කිරා මැන බලන පුවත්", "News, weighed and measured.", "Weighing",
     "Uses the everyday idiom කිරා මැන බලා (think it through carefully), tying straight back to the scale in the logo.", True),
    ("අසමතුලිත ලෝකයකට සමතුලිත පුවත්", "Balanced news for an unbalanced world.", "Balance pun",
     "Wordplay: තුලාව and සමතුලිත share the same root. Editorial and confident.", True),
    ("සත්‍යයට බරක් තියෙනවා", "Truth has weight.", "Weighing",
     "Poetic and short. Fits the scale symbol. සත්‍ය is a joined letter, so check it renders in Canva.", True),
    ("එක කනකින් නොවෙයි, කන් දෙකෙන්ම", "Not with one ear. With both.", "Both sides",
     "Twists the saying about hearing with one ear. Very Sri Lankan, a little playful.", False),
    ("ඇස් දෙකම ඇරලා කියවන්න", "Read with both eyes open.", "Reader",
     "Speaks directly to the reader. Hints at not being fooled by one-sided reporting.", False),
    ("කවුද කිව්වේ? කොහොමද කිව්වේ?", "Who said it? How did they say it?", "Reader",
     "Describes what the product does: who reported a story and with what slant.", False),
    ("පුවත තරාදියට", "Put the news on the scale.", "Weighing",
     "Uses තරාදිය, the everyday word for a scale, next to the formal තුලාව name.", False),
    ("ඇත්ත කිරා බලන තැන", "Where the truth is weighed.", "Weighing",
     "Simple and warm. Positions Thulawa as a place, not just a feed.", False),
    ("එකම පුවත, හැම කෝණයකින්ම", "One story, from every angle.", "Both sides",
     "Clear and descriptive, but close in idea to Ground News.", False),
    ("පැත්තක් නොගෙන, සියලු පැති", "Taking no side, showing all sides.", "Both sides",
     "Formal written-Sinhala version of the first option. Suits print and the seal ring.", False),
    ("බර බලන්න.", "Weigh it.", "Short",
     "Two words. A call to action for posts and buttons more than a full tagline.", False),
    ("පුවතේ සෑම පැත්තක්ම", "Every side of the story.", "Current",
     "Current tagline, for comparison. Nearly a word-for-word translation of Ground News's slogan.", False),
]

def fam(fonts):
    return "&".join("family=" + f[0].replace(" ", "+") + (":wght@" + ";".join(map(str, f[2])) if len(f[2]) > 1 else "") for f in fonts)
names = {f[0]: f for f in SI_FONTS}
gf = "https://fonts.googleapis.com/css2?" + fam(SI_FONTS) + "&" + fam([f for f in EN_FONTS if f[0] not in names]) + "&display=swap"

R = 411
html = r"""<!doctype html>
<html lang="si"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Thulawa: choose fonts and tagline</title>
<link rel="preconnect" href="https://fonts.googleapis.com"><link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="__GF__" rel="stylesheet">
<style>
:root{--navy:#13233A;--deep:#0B1626;--gold:#C9A24A;--gl:#E9D08C;--gd:#94702A;--cream:#F5ECD8;--cd:#EADFC4;--ink:#1d2533;--mut:#6b6f78}
*{box-sizing:border-box}
body{margin:0;background:var(--cream);color:var(--ink);font-family:Inter,"Noto Sans Sinhala",system-ui,sans-serif}
header{padding:28px 32px 8px} h1{margin:0 0 6px;font:600 26px Inter} header p{margin:0;color:var(--mut);max-width:900px;line-height:1.5}
.layout{display:grid;grid-template-columns:minmax(0,1fr) 540px;gap:28px;padding:20px 32px 60px;align-items:start}
@media(max-width:1150px){.layout{grid-template-columns:1fr}.side{position:static!important}}
h2{font:600 18px Inter;margin:30px 0 4px} .hint{color:var(--mut);font-size:14px;margin:0 0 14px}
.tabs{display:flex;gap:8px;margin:0 0 14px;flex-wrap:wrap}
.tabs button{border:1px solid var(--gd);background:transparent;color:var(--gd);padding:7px 14px;border-radius:999px;font:500 14px Inter;cursor:pointer}
.tabs button.on{background:var(--navy);border-color:var(--navy);color:var(--cream)}
.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:14px}
.card{background:#fffaf0;border:1px solid var(--cd);border-radius:14px;overflow:hidden;cursor:pointer;transition:.15s}
.card:hover{border-color:var(--gold)} .card.sel{outline:3px solid var(--gold);border-color:var(--gold)}
.card .sw{display:grid;grid-template-columns:1fr 1fr}
.card .sw div{padding:18px 6px 14px;text-align:center;line-height:1.3;white-space:nowrap;overflow:hidden}
.card .n{background:var(--deep);color:var(--gold)} .card .c{background:var(--cream);color:var(--navy)}
.card .meta{padding:10px 14px 12px;font-size:13px;color:var(--mut)} .card .meta b{color:var(--ink);font-size:14px}
.wts{display:flex;gap:6px;flex-wrap:wrap;margin-top:8px} .wts span{font-size:12px;padding:2px 8px;border-radius:6px;background:var(--cd);color:var(--ink);cursor:pointer}
.wts span.on{background:var(--gold);color:var(--deep)}
.tl{background:#fffaf0;border:1px solid var(--cd);border-radius:14px;padding:14px 16px;margin-bottom:10px;cursor:pointer;display:grid;grid-template-columns:1fr auto;gap:4px 12px}
.tl:hover{border-color:var(--gold)} .tl.sel{outline:3px solid var(--gold);border-color:var(--gold)}
.tl .si{font-size:26px;color:var(--navy);line-height:1.4} .tl .en{font:italic 500 15px Inter;color:var(--gd)}
.tl .why{grid-column:1/-1;font-size:13px;color:var(--mut);line-height:1.5}
.tag{font:600 11px Inter;letter-spacing:.06em;text-transform:uppercase;padding:3px 8px;border-radius:6px;background:var(--cd);color:var(--gd);align-self:start;justify-self:end}
.tag.rec{background:var(--navy);color:var(--gl)}
.side{position:sticky;top:16px;max-height:calc(100vh - 32px);overflow:auto}
.sealbox{background:#fffaf0;border:1px solid var(--cd);border-radius:14px;padding:16px;display:grid;place-items:center}
.sealbox svg{width:100%;max-width:440px;height:auto}
.opts{display:flex;gap:16px;flex-wrap:wrap;font-size:13px;color:var(--mut);margin-top:10px;justify-content:center}
.cover{margin-top:14px;background:radial-gradient(ellipse at 50% 30%,#1a3050 0%,var(--deep) 70%);border-radius:14px;aspect-ratio:1640/924;position:relative;overflow:hidden;display:flex;align-items:center;gap:16px;padding:0 26px;border:2px solid var(--gd)}
.cover:before{content:"";position:absolute;inset:10px;border:1px solid rgba(201,162,74,.5);border-radius:8px;pointer-events:none}
.cover .mini{width:40%;flex:none} .cover .t{flex:1;min-width:0}
.cover .nm{color:var(--gold);font-size:70px;line-height:1.2;white-space:nowrap}
.cover .rule{height:2px;background:linear-gradient(90deg,var(--gold),transparent);margin:6px 0 10px}
.cover .tg{color:var(--gl);font-size:18px;line-height:1.45} .cover .en2{color:var(--cream);opacity:.85;font-size:12px;letter-spacing:.2em;margin-top:8px}
.pick{margin-top:14px;background:var(--navy);color:var(--cream);border-radius:14px;padding:14px 16px;font-size:14px;line-height:1.6}
.pick code{display:block;background:var(--deep);color:var(--gl);padding:10px 12px;border-radius:8px;margin-top:8px;font-size:13px;white-space:pre-wrap}
.pick button{margin-top:8px;background:var(--gold);color:var(--deep);border:0;border-radius:8px;padding:8px 14px;font:600 13px Inter;cursor:pointer}
textarea,input[type=text]{width:100%;font-size:20px;padding:10px;border-radius:10px;border:1px solid var(--cd);background:#fffaf0;font-family:inherit;color:var(--navy)}
.note{font-size:13px;color:var(--mut);line-height:1.6;background:#fffaf0;border-left:3px solid var(--gold);padding:10px 14px;border-radius:0 10px 10px 0}
</style></head><body>
<header><h1>Thulawa: choose fonts and tagline</h1>
<p>Pick a Sinhala font for the name <b>තුලාව</b>, a Sinhala font for the tagline, an English font for <b>THULAWA NEWS</b>, and a tagline. The round seal, the cover and the lockup on the right all update live. Click a weight chip to change thickness. When you're done, press <b>Copy</b> and paste it back.</p></header>
<div class="layout"><main>
<h2>1 · Sinhala name font: තුලාව</h2><p class="hint">Used for the big name on the cover, lockups, and the ribbon on the seal (if you turn the ribbon on).</p>
<div class="tabs" id="nameFilter"></div><div class="grid" id="nameGrid"></div>

<h2>2 · Sinhala tagline font</h2><p class="hint">Used for the tagline on the seal ring and under the name. Readability matters most at small sizes.</p>
<div class="grid" id="tagGrid"></div>

<h2>3 · English font: THULAWA NEWS</h2><p class="hint">Used on the top of the seal ring and the English lines. Serif capitals suit a seal; sans fonts feel more modern.</p>
<div class="tabs" id="enFilter"></div><div class="grid" id="enGrid"></div>

<h2>4 · Tagline</h2>
<div class="note" style="margin-bottom:14px">Research notes: the current tagline <b>පුවතේ සෑම පැත්තක්ම</b> is nearly a word-for-word translation of Ground News's slogan, <i>"See every side of every news story"</i>. Newsfirst (Sirasa) already uses <i>"We report, you decide"</i>, so "you decide" lines are avoided. <b>Top pick</b> marks my recommendations.</div>
<div id="tagList"></div>
<p class="hint" style="margin-top:16px">Or write your own Sinhala tagline:</p>
<textarea id="custom" rows="2" placeholder="ඔබේම ටැග්ලයින් එක මෙතැන ලියන්න"></textarea>
<p class="hint" style="margin-top:12px">English text on top of the seal:</p>
<input type="text" id="topText" value="THULAWA NEWS">
</main>
<aside class="side">
<div class="sealbox">
<svg id="seal" viewBox="0 0 1000 1000" xmlns="http://www.w3.org/2000/svg">
<defs><clipPath id="disc"><circle cx="500" cy="500" r="348"/></clipPath>
<path id="arcTop" d="M__L__ 500A__R__ __R__ 0 0 1 __RR__ 500"/>
<path id="arcBot" d="M__L__ 500A__R__ __R__ 0 0 0 __RR__ 500"/></defs>
<g clip-path="url(#disc)"><rect width="1000" height="1000" fill="#F5ECD8"/>
<image href="data:image/jpeg;base64,__CENTRE__" x="186.8" y="186.8" width="626.4" height="626.4"/></g>
<path fill-rule="evenodd" fill="#13233A" d="M10 500a490 490 0 1 0 980 0a490 490 0 1 0-980 0ZM142 500a358 358 0 1 0 716 0a358 358 0 1 0-716 0Z"/>
<circle cx="500" cy="500" r="474" fill="none" stroke="#C9A24A" stroke-width="5"/>
<circle cx="500" cy="500" r="464" fill="none" stroke="#C9A24A" stroke-width="1.5"/>
<circle cx="500" cy="500" r="353" fill="none" stroke="#C9A24A" stroke-width="10"/>
<g fill="#C9A24A" id="stars"></g>
<g id="ribbon" style="display:none">
<path d="M262 698H340V782H262L290 740Z" fill="#0B1626" stroke="#C9A24A" stroke-width="3"/>
<path d="M738 698H660V782H738L710 740Z" fill="#0B1626" stroke="#C9A24A" stroke-width="3"/>
<path d="M318 760L340 782V760Z" fill="#05101d"/><path d="M682 760L660 782V760Z" fill="#05101d"/>
<path d="M318 676Q500 650 682 676V760Q500 734 318 760Z" fill="#13233A" stroke="#C9A24A" stroke-width="3.5"/>
<text id="ribTxt" x="500" y="733" text-anchor="middle" fill="#F5ECD8" font-size="70">තුලාව</text></g>
<text id="tTop" fill="#F5ECD8" dominant-baseline="central"><textPath href="#arcTop" startOffset="50%" text-anchor="middle">THULAWA NEWS</textPath></text>
<text id="tBot" fill="#E9D08C" dominant-baseline="central"><textPath href="#arcBot" startOffset="50%" text-anchor="middle"></textPath></text>
</svg>
<div class="opts"><label><input type="checkbox" id="ribOn"> Show තුලාව ribbon on seal</label>
<label>Top size <input type="range" id="topSize" min="50" max="100" value="80"></label>
<label>Tagline size <input type="range" id="botSize" min="36" max="80" value="58"></label></div>
</div>
<div class="cover"><svg class="mini" viewBox="0 0 1000 1000"><use href="#seal"/></svg>
<div class="t"><div class="nm" id="pvName">තුලාව</div><div class="rule"></div><div class="tg" id="pvTag"></div><div class="en2" id="pvEn">THULAWA NEWS</div></div></div>
<div class="pick">Your choice. Copy this and send it back:<code id="pick"></code><button id="copy">Copy</button></div>
</aside></div>
<script>
const SI=__SI__, EN=__EN__, TAGS=__TAGS__;
const st={name:"Abhaya Libre",nw:800,tag:"Abhaya Libre",tw:700,en:"Cinzel",ew:700,tl:0,custom:"",top:"THULAWA NEWS",rib:false,ts:80,bs:58};
const $=s=>document.querySelector(s);
const filt={name:"All",en:"All"};
// stars at the ring sides
(function(){const g=$("#stars");for(const x of [90,910]){let p=[];for(let i=0;i<10;i++){const a=-Math.PI/2+i*Math.PI/5,r=i%2?16*.42:16;p.push((x+r*Math.cos(a)).toFixed(1)+","+(500+r*Math.sin(a)).toFixed(1))}
 const e=document.createElementNS("http://www.w3.org/2000/svg","polygon");e.setAttribute("points",p.join(" "));g.appendChild(e)}})();
function card(f,kind){
  const key={name:["name","nw"],tag:["tag","tw"],en:["en","ew"]}[kind];
  const isSel=st[key[0]]===f.f;
  const w=isSel?st[key[1]]:(f.w.includes(700)?700:f.w[f.w.length-1]);
  const txt=kind==="name"?"තුලාව":kind==="tag"?"කිරා මැන බලන පුවත්":"THULAWA NEWS";
  const size=kind==="name"?54:kind==="tag"?18:24;
  const ls=kind==="en"?"letter-spacing:.12em;":"";
  const el=document.createElement("div");el.className="card"+(isSel?" sel":"");
  el.innerHTML=`<div class="sw"><div class="n" style="font-family:'${f.f}';font-weight:${w};font-size:${size}px;${ls}">${txt}</div><div class="c" style="font-family:'${f.f}';font-weight:${w};font-size:${size}px;${ls}">${txt}</div></div>
  <div class="meta"><b>${f.f}</b> · ${f.c}<br>${f.d}<div class="wts">${f.w.map(x=>`<span data-w="${x}" class="${isSel&&x===w?'on':''}">${x}</span>`).join("")}</div></div>`;
  el.onclick=e=>{st[key[0]]=f.f;st[key[1]]=e.target.dataset.w?+e.target.dataset.w:w;render()};
  return el;
}
function tabs(id,which,cats){const t=$(id);t.innerHTML="";cats.forEach(k=>{const b=document.createElement("button");b.textContent=k;b.className=k===filt[which]?"on":"";b.onclick=()=>{filt[which]=k;render()};t.appendChild(b)})}
function fit(el,size,max){el.setAttribute("font-size",size);const len=el.getComputedTextLength();if(len>max)el.setAttribute("font-size",(size*max/len).toFixed(1))}
function render(){
  tabs("#nameFilter","name",["All","Serif","Sans","Display"]);
  const ng=$("#nameGrid");ng.innerHTML="";SI.filter(f=>filt.name==="All"||f.c===filt.name).forEach(f=>ng.appendChild(card(f,"name")));
  const tg=$("#tagGrid");tg.innerHTML="";SI.filter(f=>f.f!=="Stick No Bills").forEach(f=>tg.appendChild(card(f,"tag")));
  tabs("#enFilter","en",["All","Serif","Serif caps","Sans"]);
  const eg=$("#enGrid");eg.innerHTML="";EN.filter(f=>filt.en==="All"||f.c===filt.en).forEach(f=>eg.appendChild(card(f,"en")));
  const tl=$("#tagList");tl.innerHTML="";
  TAGS.forEach((t,i)=>{const d=document.createElement("div");d.className="tl"+(st.tl===i&&!st.custom?" sel":"");
    d.innerHTML=`<div class="si" style="font-family:'${st.tag}';font-weight:${st.tw}">${t.si}</div><span class="tag ${t.rec?'rec':''}">${t.rec?'Top pick':t.a}</span><div class="en">${t.en}</div><div class="why">${t.why}</div>`;
    d.onclick=()=>{st.tl=i;st.custom="";$("#custom").value="";render()};tl.appendChild(d)});
  const tagText=st.custom||TAGS[st.tl].si;
  // seal
  const top=$("#tTop"),bot=$("#tBot");
  top.style.fontFamily=`'${st.en}'`;top.style.fontWeight=st.ew;top.style.letterSpacing=".12em";top.querySelector("textPath").textContent=st.top;
  bot.style.fontFamily=`'${st.tag}'`;bot.style.fontWeight=st.tw;bot.querySelector("textPath").textContent=tagText;
  fit(top,st.ts,1060);fit(bot,st.bs,1060);
  $("#ribbon").style.display=st.rib?"":"none";
  const rt=$("#ribTxt");rt.style.fontFamily=`'${st.name}'`;rt.style.fontWeight=st.nw;rt.textContent="තුලාව";
  rt.setAttribute("font-size",70);const rl=rt.getComputedTextLength();if(rl>330)rt.setAttribute("font-size",(70*330/rl).toFixed(1));
  // cover
  const n=$("#pvName");n.style.fontFamily=`'${st.name}'`;n.style.fontWeight=st.nw;
  const t=$("#pvTag");t.textContent=tagText;t.style.fontFamily=`'${st.tag}'`;t.style.fontWeight=st.tw;
  const e=$("#pvEn");e.textContent=st.top;e.style.fontFamily=`'${st.en}'`;e.style.fontWeight=st.ew;
  $("#pick").textContent=`Sinhala name font: ${st.name} ${st.nw}\nSinhala tagline font: ${st.tag} ${st.tw}\nEnglish font: ${st.en} ${st.ew}\nTagline: ${tagText}\nSeal top text: ${st.top}\nSeal ribbon: ${st.rib?"on":"off"}\nSeal sizes: top ${st.ts}, tagline ${st.bs}`;
}
$("#custom").oninput=e=>{st.custom=e.target.value.trim();render()};
$("#topText").oninput=e=>{st.top=e.target.value||"THULAWA NEWS";render()};
$("#ribOn").onchange=e=>{st.rib=e.target.checked;render()};
$("#topSize").oninput=e=>{st.ts=+e.target.value;render()};
$("#botSize").oninput=e=>{st.bs=+e.target.value;render()};
$("#copy").onclick=()=>{navigator.clipboard.writeText($("#pick").textContent);$("#copy").textContent="Copied"};
render();
document.fonts.ready.then(render);document.fonts.onloadingdone=()=>render();
</script></body></html>"""

conv = lambda fs: json.dumps([dict(f=f[0], c=f[1], w=f[2], d=f[3]) for f in fs], ensure_ascii=False)
html = (html.replace("__GF__", gf).replace("__CENTRE__", centre)
        .replace("__L__", str(500 - R)).replace("__RR__", str(500 + R)).replace("__R__", str(R))
        .replace("__SI__", conv(SI_FONTS)).replace("__EN__", conv(EN_FONTS))
        .replace("__TAGS__", json.dumps([dict(si=t[0], en=t[1], a=t[2], why=t[3], rec=t[4]) for t in TAGLINES], ensure_ascii=False)))
open(out, "w").write(html)
print("ok", len(html))
