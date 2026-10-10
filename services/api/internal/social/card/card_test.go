package card

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"
)

func sampleCard() Card {
	return Card{Kind: KindTopStory, Headline: "ශ්‍රී ලංකාවේ ප්‍රධාන පුවත්", Summary: "රාජ්‍ය ප්‍රතිපත්ති ගැන විශේෂ වාර්තාවක්.", Sources: []string{"ලංකාදීප"}, SourceCount: 1, PublishedAt: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)}
}
func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Card)
		bad    bool
	}{
		{"valid", func(c *Card) {}, false},
		{"limits", func(c *Card) { c.Headline = strings.Repeat("ක", 90); c.Summary = strings.Repeat("ක", 280) }, false},
		{"optional", func(c *Card) { c.Summary = ""; c.Kicker = ""; c.Sources = nil }, false},
		{"empty headline", func(c *Card) { c.Headline = " \n" }, true},
		{"long headline", func(c *Card) { c.Headline = strings.Repeat("ක", 91) }, true},
		{"long summary", func(c *Card) { c.Summary = strings.Repeat("ක", 281) }, true},
		{"bad UTF8", func(c *Card) { c.Headline = "\xff" }, true},
		{"kind", func(c *Card) { c.Kind = "unknown" }, true},
		{"zero count", func(c *Card) { c.SourceCount = 0 }, true},
		{"negative count", func(c *Card) { c.SourceCount = -1 }, true},
		{"huge count", func(c *Card) { c.SourceCount = 10001 }, true},
		{"inconsistent count", func(c *Card) { c.Sources = append(c.Sources, "BBC") }, true},
		{"empty credit", func(c *Card) { c.Sources = []string{" "} }, true},
		{"spectrum", func(c *Card) { c.Spectrum = &Spectrum{0, 50, 100} }, false},
		{"negative spectrum", func(c *Card) { c.Spectrum = &Spectrum{-1, 50, 51} }, true},
		{"high spectrum", func(c *Card) { c.Spectrum = &Spectrum{0, 0, 101} }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := sampleCard()
			tt.change(&c)
			if err := Validate(c); (err != nil) != tt.bad {
				t.Fatalf("Validate()=%v, bad=%v", err, tt.bad)
			}
		})
	}
}
func TestHTML(t *testing.T) {
	c := sampleCard()
	c.Kind = KindBreaking
	c.Kicker = `<img src="https://invalid.test">`
	c.Headline = `<script>alert(1)</script>`
	c.Summary = `A & B`
	c.Sources = []string{"<b>credit</b>", "2", "3", "4", "5", "hidden"}
	c.SourceCount = 12
	c.Spectrum = &Spectrum{25, 50, 25}
	html, err := renderHTML(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"&lt;script&gt;", "&lt;img", "A &amp; B", "&lt;b&gt;credit&lt;/b&gt;", "+7", "BREAKING", "විශේෂ පුවත්", "2026.10.10 · 05:30", "data:font/woff2;base64,", "font-src data:", "වම 25%"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, bad := range []string{"<script>", "<img src=", "hidden</span>", "ZgotmplZ"} {
		if strings.Contains(html, bad) {
			t.Errorf("unexpected %q", bad)
		}
	}
	c.Spectrum = nil
	c.Kicker = ""
	c.Kind = KindTopStory
	html, err = renderHTML(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "දේශපාලන නැඹුරුව") || strings.Contains(html, `class="kicker"`) {
		t.Fatal("optional sections present")
	}
	c.Headline = ""
	if _, err := renderHTML(c); err == nil {
		t.Fatal("invalid card accepted")
	}
}
func TestChromeRender(t *testing.T) {
	r, err := NewChromeRenderer(Options{})
	if err != nil {
		t.Skipf("Chrome unavailable: %v", err)
	}
	c := sampleCard()
	c.Headline = strings.Repeat("ශ්‍රී ලංකාවේ ", 10)
	c.Headline = string([]rune(c.Headline)[:90])
	c.Summary = strings.Repeat("ප්‍රජාතන්ත්‍රවාදය ", 20)
	c.Summary = string([]rune(c.Summary)[:280])
	c.SourceCount = 12
	data, err := r.Render(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != 1080 || config.Height != 1350 {
		t.Fatalf("dimensions %dx%d", config.Width, config.Height)
	}
}
func TestCancelledRender(t *testing.T) {
	r, err := NewChromeRenderer(Options{})
	if err != nil {
		t.Skip(err)
	}
	renderMu.Lock()
	defer renderMu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Render(ctx, sampleCard()); err == nil {
		t.Fatal("cancelled render succeeded")
	}
}

func TestOptions(t *testing.T) {
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNAP_SOCIAL_CHROME_PATH", path)
	t.Setenv("SNAP_SOCIAL_CHROME_NO_SANDBOX", "")
	r, err := NewChromeRenderer(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.timeout != 30*time.Second || r.noSandbox != (os.Getuid() == 0) {
		t.Fatalf("unexpected defaults: %+v", r)
	}
	t.Setenv("SNAP_SOCIAL_CHROME_NO_SANDBOX", "true")
	r, err = NewChromeRenderer(Options{ChromePath: path, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !r.noSandbox || r.timeout != time.Second {
		t.Fatalf("unexpected explicit configuration: %+v", r)
	}
	t.Setenv("SNAP_SOCIAL_CHROME_NO_SANDBOX", "invalid")
	if _, err := NewChromeRenderer(Options{}); err == nil {
		t.Fatal("invalid boolean accepted")
	}
	t.Setenv("SNAP_SOCIAL_CHROME_NO_SANDBOX", "")
	if _, err := NewChromeRenderer(Options{Timeout: -time.Second}); err == nil {
		t.Fatal("negative timeout accepted")
	}
	if _, err := NewChromeRenderer(Options{ChromePath: "/missing/chrome"}); err == nil {
		t.Fatal("missing executable accepted")
	}
}
