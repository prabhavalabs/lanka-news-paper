package card

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"strings"
	"time"
)

//go:embed assets/fonts/*
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

func renderHTML(c Card) (string, error) {
	if err := Validate(c); err != nil {
		return "", err
	}
	shown := min(5, len(c.Sources))
	view := struct {
		Card
		Fonts    template.CSS
		Breaking bool
		Credits  []string
		More     int
		Date     string
	}{c, fontCSS, c.Kind == KindBreaking, c.Sources[:shown], c.SourceCount - shown, c.PublishedAt.In(colombo).Format("2006.01.02 · 15:04")}
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
 if(!fit(headline,88,30,4,available-61-(summary.textContent?215:0))) throw new Error('Headline cannot fit');
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
