// social-card-preview renders sample editorial cards without backend services.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/card"
)

func main() {
	output := flag.String("output", "/tmp/lnp-social/preview", "output directory")
	flag.Parse()
	if flag.NArg() > 0 {
		*output = flag.Arg(0)
	}
	if err := run(*output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(output string) error {
	renderer, err := card.NewChromeRenderer(card.Options{})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	at := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	sources := []string{"අද දෙරණ", "හිරු පුවත්", "ලංකාදීප", "News First", "BBC සිංහල", "Daily Mirror", "Ada"}
	samples := []struct {
		name string
		c    card.Card
	}{
		{"breaking", card.Card{Kind: card.KindBreaking, Kicker: "දේශපාලන", Headline: "පාර්ලිමේන්තුවේ විශේෂ විවාදයක්: ආර්ථික ප්‍රතිසංස්කරණ ගැන අද තීරණයක්", Summary: "නව ආර්ථික යෝජනා පිළිබඳ ආණ්ඩුව සහ විපක්ෂය අද අදහස් පළ කිරීමට නියමිතය. ජනතාවගේ ජීවන වියදම, බදු සහ රාජ්‍ය සේවා ගැන කරුණු රැසක් විවාදයට එක් වෙයි.", Sources: sources, SourceCount: 7, Spectrum: &card.Spectrum{Left: 24, Center: 51, Right: 25}, PublishedAt: at}},
		{"top_story", card.Card{Kind: card.KindTopStory, Kicker: "ආර්ථික", Headline: "ශ්‍රී ලංකාවේ ආර්ථිකය යළි වර්ධනයට: ජනතාවගේ ජීවන වියදමට සහනයක් ලැබේවිද?", Summary: "ආර්ථික දර්ශකවල වර්ධනයක් පෙනුණත් අත්‍යවශ්‍ය භාණ්ඩ මිල සහ පවුල්වල ආදායම පිළිබඳ ගැටලු තවමත් පවතී. මූලාශ්‍ර කිහිපයක වාර්තා ඇසුරින් ප්‍රධාන කරුණු විමසා බලමු.", Sources: sources, SourceCount: 7, Spectrum: &card.Spectrum{Left: 18, Center: 62, Right: 20}, PublishedAt: at}},
	}
	// Exact rune limits; repeated wide glyphs also exercise unbroken-word fitting.
	extreme := card.Card{Kind: card.KindTopStory, Headline: strings.Repeat("ශ්‍රී ලංකාවේ ප්‍රධාන පුවත් ", 6), Summary: strings.Repeat("රාජ්‍ය ආර්ථික ප්‍රතිපත්ති සහ ජනතාවගේ ජීවන වියදම පිළිබඳ විශේෂ වාර්තාව. ", 6), Sources: append(append([]string{}, sources...), "The Island", "Sunday Times", "මව්බිම", "දිවයින", "සිළුමිණ"), SourceCount: 12, PublishedAt: at}
	extreme.Headline = string([]rune(extreme.Headline)[:90])
	extreme.Summary = string([]rune(extreme.Summary)[:280])
	samples = append(samples, struct {
		name string
		c    card.Card
	}{"extreme", extreme})
	for _, sample := range samples {
		png, err := renderer.Render(context.Background(), sample.c)
		if err != nil {
			return fmt.Errorf("%s: %w", sample.name, err)
		}
		path := filepath.Join(output, sample.name+".png")
		if err := os.WriteFile(path, png, 0644); err != nil {
			return err
		}
		fmt.Println(path)
	}
	return nil
}
