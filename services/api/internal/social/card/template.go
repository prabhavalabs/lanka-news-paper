package card

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"
)

//go:embed assets/fonts/* assets/brand/masthead.png
var assets embed.FS

//go:embed card.html
var pageTemplate string

var colombo = func() *time.Location {
	l, err := time.LoadLocation("Asia/Colombo")
	if err != nil {
		panic(err)
	}
	return l
}()
var cardTemplate = template.Must(template.New("card").Parse(pageTemplate))
var fontCSS = func() template.CSS {
	var css strings.Builder
	for _, font := range []struct {
		slug, name string
		weight     int
	}{{"abhaya-libre", "Abhaya Libre", 700}, {"noto-serif-sinhala", "Noto Serif Sinhala", 400}, {"noto-sans-sinhala", "Noto Sans Sinhala", 400}} {
		for _, subset := range []string{"sinhala", "latin"} {
			filename := fmt.Sprintf("assets/fonts/%s-%s-%d-normal.woff2", font.slug, subset, font.weight)
			data, err := assets.ReadFile(filename)
			if err != nil {
				panic(err)
			}
			unicodeRange := "U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+0304,U+0308,U+0329,U+2000-206F,U+20AC,U+2122,U+2191,U+2193,U+2212,U+2215,U+FEFF,U+FFFD"
			if subset == "sinhala" {
				unicodeRange = "U+0964-0965,U+0D81-0DF4,U+1CF2,U+200C-200D,U+25CC,U+111E1-111F4"
			}
			fmt.Fprintf(&css, "@font-face{font-family:'%s';font-style:normal;font-weight:%d;font-display:block;src:url(data:font/woff2;base64,%s) format('woff2');unicode-range:%s;}\n", font.name, font.weight, base64.StdEncoding.EncodeToString(data), unicodeRange)
		}
	}
	// Only build-time embedded fonts are trusted CSS; all card text is escaped.
	return template.CSS(css.String())
}()

// masthead is the Thulawa brand header, inlined because the card CSP only allows data: images.
var masthead = func() template.URL {
	data, err := assets.ReadFile("assets/brand/masthead.png")
	if err != nil {
		panic(err)
	}
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(data))
}()

// Logos are embedded as data URLs because the card CSP only allows data: images.
var logoTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true, "image/svg+xml": true}

const (
	maxMarksShown = 10
	spectrumWidth = 952 // CSS px, the card's inner width
	tileWidth     = 62
	tileGap       = 6
)

type markView struct {
	Name    string
	Logo    template.URL
	Initial string
	X       int // tile left edge
	Center  int // tile centre
	Target  int // exact lean position on the bar
}

type coverageView struct {
	Spectrum *Spectrum
	Placed   []markView // sources placed on the spectrum by lean
	Unrated  []markView // sources without analysed coverage (or all sources when there is no spectrum)
	More     int
}

func toMarkView(m SourceMark) markView {
	view := markView{Name: m.Name, Initial: string([]rune(strings.TrimSpace(m.Name))[:1])}
	if len(m.Logo) > 0 && logoTypes[m.LogoType] {
		view.Logo = template.URL("data:" + m.LogoType + ";base64," + base64.StdEncoding.EncodeToString(m.Logo))
	}
	return view
}

// spread places tiles as close as possible to their targets without overlapping:
// overlapping tiles form a block centred on the mean of their targets, kept inside the card.
func spread(marks []markView) {
	sort.SliceStable(marks, func(i, j int) bool { return marks[i].Target < marks[j].Target })
	slot := float64(tileWidth + tileGap)
	type block struct {
		first, count int
		sum          float64
	}
	var blocks []block
	start := func(b block) float64 {
		left := b.sum/float64(b.count) - slot*float64(b.count-1)/2 - tileWidth/2.0
		return max(0, min(float64(spectrumWidth)-slot*float64(b.count-1)-tileWidth, left))
	}
	for i, m := range marks {
		blocks = append(blocks, block{i, 1, float64(m.Target)})
		for len(blocks) > 1 {
			prev, last := blocks[len(blocks)-2], blocks[len(blocks)-1]
			if start(prev)+slot*float64(prev.count) <= start(last) {
				break
			}
			blocks = append(blocks[:len(blocks)-2], block{prev.first, prev.count + last.count, prev.sum + last.sum})
		}
	}
	for _, b := range blocks {
		left := start(b)
		for k := 0; k < b.count; k++ {
			marks[b.first+k].X = int(left + slot*float64(k) + 0.5)
			marks[b.first+k].Center = marks[b.first+k].X + tileWidth/2
		}
	}
}

// coverage places each credited source on a continuous left-to-right spectrum by its
// average analysed lean. Sources without analysis are listed separately, never guessed.
func coverage(c Card) coverageView {
	marks := c.Marks
	if len(marks) == 0 {
		for _, name := range c.Sources {
			marks = append(marks, SourceMark{Name: name})
		}
	}
	if len(marks) > maxMarksShown {
		marks = marks[:maxMarksShown]
	}
	view := coverageView{Spectrum: c.Spectrum, More: max(0, c.SourceCount-len(marks))}
	for _, m := range marks {
		mv := toMarkView(m)
		if c.Spectrum != nil && m.Lean != nil {
			mv.Target = int((*m.Lean+1)/2*spectrumWidth + 0.5)
			view.Placed = append(view.Placed, mv)
		} else {
			view.Unrated = append(view.Unrated, mv)
		}
	}
	spread(view.Placed)
	return view
}

func renderHTML(c Card) (string, error) {
	if err := Validate(c); err != nil {
		return "", err
	}
	view := struct {
		Card
		Fonts    template.CSS
		Breaking bool
		Coverage coverageView
		Date     string
		Masthead template.URL
	}{c, fontCSS, c.Kind == KindBreaking, coverage(c), c.PublishedAt.In(colombo).Format("2006.01.02 · 15:04"), masthead}
	var output bytes.Buffer
	if err := cardTemplate.Execute(&output, view); err != nil {
		return "", fmt.Errorf("render card HTML: %w", err)
	}
	return output.String(), nil
}

const fitScript = `(async () => {
 await Promise.all([document.fonts.load('700 80px "Abhaya Libre"', 'ශ්‍රී ලංකා ප්‍රධාන'),document.fonts.load('400 36px "Noto Serif Sinhala"','ශ්‍රී ලංකා'),document.fonts.load('400 24px "Noto Sans Sinhala"','මූලාශ්‍ර')]);
 await document.fonts.ready;
 function fit(el,start,min,lines,maxHeight=Infinity) {
  for(let size=start;size>=min;size--) {
   el.style.fontSize=size+'px';
   const lh=parseFloat(getComputedStyle(el).lineHeight);
   if(el.scrollHeight<=Math.min(lh*lines+2,maxHeight) && el.scrollWidth<=el.clientWidth+1) return true;
  }
  return false;
 }
 const story=document.querySelector('.story'), headline=document.querySelector('h1'), summary=document.querySelector('.summary');
 const style=getComputedStyle(story);
 const available=story.clientHeight-parseFloat(style.paddingTop)-parseFloat(style.paddingBottom);
 if(!fit(headline,72,30,4,available-61-(summary.textContent?215:0))) throw new Error('Headline cannot fit');
 const summaryHeight=available-headline.getBoundingClientRect().height-61;
 if(!fit(summary,36,26,5,summaryHeight)) {
  const lines=Math.min(5,Math.floor(summaryHeight/parseFloat(getComputedStyle(summary).lineHeight)));
  if(lines<1 && summary.textContent) throw new Error('Summary cannot fit');
  summary.style.display='-webkit-box';summary.style.webkitBoxOrient='vertical';summary.style.webkitLineClamp=String(lines);summary.style.overflow='hidden';
 }
 await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
 const card=document.querySelector('#card');
 if(card.scrollHeight>1350 || card.scrollWidth>1080) throw new Error('Card overflow');
})()`
