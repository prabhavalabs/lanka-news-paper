package reels

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/stretchr/testify/require"
)

func TestParseGoogleTrends(t *testing.T) {
	feed := `<?xml version="1.0"?><rss xmlns:ht="https://trends.google.com/trending/rss"><channel>
	<item><title>namal rajapaksa</title><ht:approx_traffic>2000+</ht:approx_traffic>
	<ht:news_item><ht:news_item_title>Namal &amp; the Airbus case</ht:news_item_title></ht:news_item></item>
	<item><title> </title></item></channel></rss>`
	trends, err := parseGoogleTrends([]byte(feed))
	require.NoError(t, err)
	require.Equal(t, []googleTrend{{Query: "namal rajapaksa", Traffic: "2000+", News: []string{"Namal & the Airbus case"}}}, trends)
}

func TestReusableLicence(t *testing.T) {
	for _, licence := range []string{"CC0", "Public domain", "CC BY 2.0", "CC BY 4.0", "cc by 3.0"} {
		require.True(t, reusableLicense.MatchString(licence), licence)
	}
	for _, licence := range []string{"CC BY-SA 4.0", "CC BY-NC 2.0", "CC BY-ND 3.0", "GODL-India", "Fair use", ""} {
		require.False(t, reusableLicense.MatchString(licence), licence)
	}
}

func TestChooseQuestionPrefersEvenSplit(t *testing.T) {
	script := Script{Questions: []Question{
		{Question: "Should the case continue?", YesLabel: "ඔව් ❤️", NoLabel: "නැහැ", ExpectedYes: 92},
		{Question: "Are you satisfied?", YesLabel: "ඔව් ❤️", NoLabel: "නැහැ 👍", ExpectedYes: 47},
		{Question: "Do you trust it?", YesLabel: "ඔව්", NoLabel: "නැහැ", ExpectedYes: 40},
	}}
	require.NoError(t, script.chooseQuestion())
	require.Equal(t, "Are you satisfied?", script.Question)
	require.Equal(t, "ඔව්", script.YesLabel)
	require.Equal(t, "නැහැ", script.NoLabel)
}

func TestBuildCaptionCreditsPhotosAndAI(t *testing.T) {
	caption := buildCaption(Script{Caption: "Facts.", Question: "Q?", YesLabel: "ඔව්", NoLabel: "නැහැ", Hashtags: []string{"#Courts", "ශ්‍රී ලංකා"}},
		[]Photo{{Kind: "photo", Credit: "Jane / Wikimedia Commons (CC0)"}, {Kind: "ai", Credit: aiCredit}})
	require.Contains(t, caption, "🗳️ Q?")
	require.Contains(t, caption, "#Courts #ශ්‍රීලංකා #තුලාව #ThulawaNews")
	again := buildCaption(Script{Caption: "x", Hashtags: []string{"තුලාව", "#thulawanews", "Courts"}}, nil)
	require.Equal(t, 1, strings.Count(again, "#තුලාව"))
	require.True(t, strings.HasSuffix(again, "#තුලාව #thulawanews #Courts"), again)
	require.Contains(t, caption, "ඡායාරූප: Jane / Wikimedia Commons (CC0) · සමහර රූප AI මගින් නිර්මිත නිදර්ශන වේ")
}

func TestLibraryRotatesAndHonoursTrackList(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.mp3", "b.mp3", "c.mp3"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644))
	}
	library := Library{Dir: dir}
	first, _, ok := library.choose(nil)
	require.True(t, ok)
	require.Equal(t, "a.mp3", first.File)
	next, _, _ := library.choose([]string{"a.mp3"})
	require.Equal(t, "b.mp3", next.File)
	next, _, _ = library.choose([]string{"b.mp3", "a.mp3"})
	require.Equal(t, "c.mp3", next.File)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracks.json"), []byte(`[{"file":"c.mp3","title":"Epic","start":28.2},{"file":"missing.mp3"}]`), 0o644))
	listed, path, _ := library.choose(nil)
	require.Equal(t, track{File: "c.mp3", Title: "Epic", Start: 28.2}, listed)
	require.True(t, strings.HasSuffix(path, "c.mp3"))

	_, _, ok = Library{Dir: filepath.Join(dir, "none")}.choose(nil)
	require.False(t, ok)
}

func TestValidScriptLimits(t *testing.T) {
	valid := Script{Tag: "අධිකරණ පුවත", Headline: []string{"එයාර්බස් නඩුව"}, Facts: []string{"Fact"}, Question: "ඔබ සෑහීමකට පත්වෙනවාද?", YesLabel: "ඔව්", NoLabel: "නැහැ"}
	require.NoError(t, validScript(valid))
	long := valid
	long.Headline = []string{strings.Repeat("අ", 31)}
	require.ErrorIs(t, validScript(long), ErrInvalid)
	empty := valid
	empty.YesLabel = ""
	require.ErrorIs(t, validScript(empty), ErrInvalid)
}

func TestPlainLabelKeepsSinhalaJoiners(t *testing.T) {
	require.Equal(t, "ශ්‍රී", plainLabel("ශ්‍රී ❤️"))
}

func TestCommonsImagesAcceptNumericMetadataAndFilterLicences(t *testing.T) {
	data := `{"query":{"pages":{
	 "1":{"title":"File:Free.jpg","imageinfo":[{"thumburl":"https://x/free.jpg","width":1800,"height":1200,"mime":"image/jpeg","extmetadata":{"LicenseShortName":{"value":"CC BY 2.0"},"Artist":{"value":"<a href=\"u\">Jane</a> Doe"},"Copyrighted":{"value":true},"ImageWidth":{"value":1800},"DateTimeOriginal":{"value":"2019-03-02"}}}]},
	 "2":{"title":"File:ShareAlike.jpg","imageinfo":[{"thumburl":"https://x/sa.jpg","width":1800,"height":1200,"mime":"image/jpeg","extmetadata":{"LicenseShortName":{"value":"CC BY-SA 4.0"}}}]},
	 "4":{"title":"File:Old.jpg","imageinfo":[{"thumburl":"https://x/old.jpg","width":1800,"height":1200,"mime":"image/jpeg","extmetadata":{"LicenseShortName":{"value":"Public domain"},"DateTimeOriginal":{"value":"circa 1880"}}}]},
	 "5":{"title":"File:Panorama.jpg","imageinfo":[{"thumburl":"https://x/pano.jpg","width":4000,"height":1000,"mime":"image/jpeg","extmetadata":{"LicenseShortName":{"value":"CC0"}}}]},
	 "3":{"title":"File:Tiny.png","imageinfo":[{"thumburl":"https://x/tiny.png","width":300,"mime":"image/png","extmetadata":{"LicenseShortName":{"value":"CC0"}}}]}}}}`
	var response imageInfoResponse
	require.NoError(t, json.Unmarshal([]byte(data), &response))
	images := response.images()
	require.Len(t, images, 1)
	require.Equal(t, "Jane Doe / Wikimedia Commons (CC BY 2.0)", images[0].Credit())
}

type fakeModel struct{ text string }

func (model fakeModel) Complete(context.Context, llm.Request) (llm.Response, error) {
	return llm.Response{Text: model.text}, nil
}

func TestRankStoriesMapsShortKeysToStorylines(t *testing.T) {
	candidates := []candidate{{ID: "uuid-fire"}, {ID: "uuid-court"}}
	ranking, err := rankStories(context.Background(), fakeModel{`{"ranking":[{"key":"c2","google_trends":["namal"],"reason":"court"},{"key":"c9","google_trends":[],"reason":"unknown"},{"key":"c2","google_trends":[],"reason":"duplicate"}]}`}, candidates, nil)
	require.NoError(t, err)
	require.Len(t, ranking, 1)
	require.Equal(t, "uuid-court", ranking[0].StorylineID)
	require.Equal(t, "court", ranking[0].Reason)
}

func TestCommonsLookupLive(t *testing.T) {
	if os.Getenv("SNAP_REEL_NETWORK_TEST") != "1" {
		t.Skip("SNAP_REEL_NETWORK_TEST is not set")
	}
	client := &http.Client{Timeout: 20 * time.Second}
	person, err := personPhoto(context.Background(), client, "Namal Rajapaksa")
	require.NoError(t, err)
	require.NotNil(t, person)
	t.Logf("person: %s %s", person.Title, person.Credit())
	object, err := searchPhoto(context.Background(), client, "SriLankan Airlines aircraft")
	require.NoError(t, err)
	require.NotNil(t, object)
	t.Logf("object: %s %s", object.Title, object.Credit())
	trends, err := googleTrends(context.Background(), client)
	require.NoError(t, err)
	require.NotEmpty(t, trends)
}
