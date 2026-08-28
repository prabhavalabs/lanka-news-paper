package newsletter

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
)

type RenderedEdition struct {
	Subject   string
	Preheader string
	HTML      string
	Text      string
}

type editionView struct {
	Digest         Digest
	Greeting       string
	LeadStories    []Story
	Sections       []storySection
	Subject        string
	Preheader      string
	IntroText      string
	FooterText     string
	UnsubscribeURL string
}

type storySection struct {
	Name    string
	Stories []Story
}

func RenderEdition(digest Digest, recipientName, unsubscribeURL string) (RenderedEdition, error) {
	return RenderEditionWithSettings(digest, recipientName, unsubscribeURL, defaultSettings())
}

func RenderEditionWithSettings(digest Digest, recipientName, unsubscribeURL string, settings Settings) (RenderedEdition, error) {
	if settings.SubjectTemplate == "" {
		settings = defaultSettings()
	}
	replacements := strings.NewReplacer(
		"{{date}}", digest.EditionDate,
		"{{articles}}", fmt.Sprint(digest.ArticleCount),
		"{{events}}", fmt.Sprint(digest.EventCount),
		"{{sources}}", fmt.Sprint(digest.SourceCount),
	)
	subject := replacements.Replace(settings.SubjectTemplate)
	preheader := replacements.Replace(settings.PreheaderTemplate)
	if strings.TrimSpace(digest.Intro) != "" {
		settings.IntroText = digest.Intro
	}
	greeting := "සුබ උදෑසනක්"
	if name := strings.TrimSpace(recipientName); name != "" {
		greeting += ", " + name
	}
	storyCount := min(settings.MaxStories, len(digest.Stories))
	digest.Stories = digest.Stories[:storyCount]
	leadCount := min(settings.LeadStoryCount, len(digest.Stories))
	view := editionView{
		Digest: digest, Greeting: greeting, LeadStories: digest.Stories[:leadCount],
		Sections: groupStories(digest.Stories[leadCount:]), Subject: subject,
		Preheader: preheader, IntroText: settings.IntroText, FooterText: settings.FooterText,
		UnsubscribeURL: unsubscribeURL,
	}
	var htmlOutput bytes.Buffer
	if err := newsletterHTML.Execute(&htmlOutput, view); err != nil {
		return RenderedEdition{}, fmt.Errorf("render newsletter HTML: %w", err)
	}
	var textOutput bytes.Buffer
	if err := newsletterText.Execute(&textOutput, view); err != nil {
		return RenderedEdition{}, fmt.Errorf("render newsletter text: %w", err)
	}
	return RenderedEdition{Subject: subject, Preheader: preheader, HTML: htmlOutput.String(), Text: textOutput.String()}, nil
}

func groupStories(stories []Story) []storySection {
	sections := make([]storySection, 0)
	positions := make(map[string]int)
	for _, story := range stories {
		position, exists := positions[story.Category]
		if !exists {
			position = len(sections)
			positions[story.Category] = position
			sections = append(sections, storySection{Name: story.Category, Stories: make([]Story, 0)})
		}
		sections[position].Stories = append(sections[position].Stories, story)
	}
	return sections
}

func coverageLabel(story Story) string {
	if story.SourceCount > 1 {
		return fmt.Sprintf("මූලාශ්‍ර %d · වාර්තා %d", story.SourceCount, story.ArticleCount)
	}
	return "තනි මූලාශ්‍ර වාර්තාව"
}

var newsletterHTML = htmltemplate.Must(htmltemplate.New("newsletter").Funcs(htmltemplate.FuncMap{
	"coverage": coverageLabel,
}).Parse(`<!doctype html>
<html lang="si"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Subject}}</title><style>
@media (max-width:680px){.shell{width:100%!important}.pad{padding-left:20px!important;padding-right:20px!important}.headline{font-size:24px!important}.story-title{font-size:19px!important}}
</style></head>
<body style="margin:0;background:#f3f1ec;color:#1c1b19;font-family:'Noto Sans Sinhala','Nirmala UI','Iskoola Pota',Arial,sans-serif">
<div style="display:none;max-height:0;overflow:hidden;opacity:0">{{.Preheader}}</div>
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#f3f1ec"><tr><td align="center" style="padding:24px 12px">
<table role="presentation" width="680" cellspacing="0" cellpadding="0" class="shell" style="width:680px;max-width:100%;background:#fff;border:1px solid #dedbd3">
<tr><td class="pad" style="padding:34px 42px 28px;border-bottom:3px solid #191816">
<div style="font-size:13px;letter-spacing:.12em;text-transform:uppercase;color:#6d6a63">Lanka News Paper</div>
<h1 class="headline" style="margin:12px 0 6px;font-size:30px;line-height:1.35">උදෑසන පුවත් සංග්‍රහය</h1>
<p style="margin:0;color:#6d6a63;font-size:14px">{{.Digest.EditionDate}} · පසුගිය පැය 24</p>
</td></tr>
<tr><td class="pad" style="padding:28px 42px 8px">
<p style="margin:0 0 12px;font-size:17px;line-height:1.75">{{.Greeting}}</p>
{{if .IntroText}}<p style="margin:0 0 12px;font-size:16px;line-height:1.8;color:#3d3a35">{{.IntroText}}</p>{{end}}
<p style="margin:0;font-size:15px;line-height:1.7;color:#55524c">පුවත් {{.Digest.ArticleCount}} · සිදුවීම් {{.Digest.EventCount}} · මූලාශ්‍ර {{.Digest.SourceCount}}</p>
</td></tr>
{{if .LeadStories}}
<tr><td class="pad" style="padding:24px 42px 8px"><h2 style="margin:0;font-size:21px">අද දැනගත යුතු ප්‍රධාන කරුණු</h2></td></tr>
{{range .LeadStories}}
<tr><td class="pad" style="padding:18px 42px;border-bottom:1px solid #ebe8e1">
<div style="font-size:12px;color:#7a5646">{{.Category}}{{if .Breaking}} · විශේෂ පුවත{{end}}</div>
<h3 class="story-title" style="margin:7px 0 8px;font-size:21px;line-height:1.5"><a href="{{.URL}}" style="color:#1c1b19;text-decoration:none">{{.Title}}</a></h3>
<p style="margin:0;font-size:16px;line-height:1.8;color:#3d3a35">{{.Summary}}</p>
<p style="margin:10px 0 0;font-size:12px;color:#767169">{{coverage .}}</p>
</td></tr>
{{end}}
{{else}}
<tr><td class="pad" style="padding:36px 42px"><p style="margin:0;font-size:17px;line-height:1.8">පසුගිය පැය 24 තුළ ප්‍රකාශිත වැදගත් පුවත් හමු නොවීය.</p></td></tr>
{{end}}
{{range .Sections}}
<tr><td class="pad" style="padding:28px 42px 4px"><h2 style="margin:0;font-size:20px">{{.Name}}</h2></td></tr>
{{range .Stories}}
<tr><td class="pad" style="padding:14px 42px;border-bottom:1px solid #ebe8e1">
<h3 style="margin:0 0 6px;font-size:17px;line-height:1.6"><a href="{{.URL}}" style="color:#1c1b19">{{.Title}}</a></h3>
<p style="margin:0;font-size:14px;line-height:1.75;color:#55524c">{{.Summary}}</p>
<p style="margin:8px 0 0;font-size:12px;color:#767169">{{coverage .}}</p>
</td></tr>
{{end}}{{end}}
<tr><td class="pad" style="padding:28px 42px;background:#f8f7f4;color:#68645d;font-size:12px;line-height:1.7">
<p style="margin:0 0 8px">{{.FooterText}}</p>
{{if .UnsubscribeURL}}<p style="margin:0"><a href="{{.UnsubscribeURL}}" style="color:#52504b">පුවත් සංග්‍රහයෙන් ඉවත් වන්න</a></p>{{end}}
</td></tr></table></td></tr></table></body></html>`))

var newsletterText = texttemplate.Must(texttemplate.New("newsletter-text").Funcs(texttemplate.FuncMap{
	"coverage": coverageLabel,
}).Parse(`{{.Subject}}
{{.Preheader}}

{{.Greeting}}
{{if .IntroText}}
{{.IntroText}}
{{end}}

{{if .LeadStories}}අද දැනගත යුතු ප්‍රධාන කරුණු
{{range .LeadStories}}
{{.Category}} — {{.Title}}
{{.Summary}}
{{coverage .}}
{{.URL}}
{{end}}{{else}}පසුගිය පැය 24 තුළ ප්‍රකාශිත වැදගත් පුවත් හමු නොවීය.
{{end}}{{range .Sections}}
{{.Name}}
{{range .Stories}}
{{.Title}}
{{.Summary}}
{{coverage .}}
{{.URL}}
{{end}}{{end}}
{{if .UnsubscribeURL}}පුවත් සංග්‍රහයෙන් ඉවත් වන්න: {{.UnsubscribeURL}}{{end}}
`))
