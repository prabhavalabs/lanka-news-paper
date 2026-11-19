package reels

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/publish"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/storyline"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/trends"
)

// ErrNothingToReel means no trending storyline is suitable or new enough for a reel right now.
var ErrNothingToReel = errors.New("no trending story needs a reel right now")

type completer interface {
	Complete(context.Context, llm.Request) (llm.Response, error)
}

type illustrator interface {
	GenerateImage(ctx context.Context, model, prompt string, timeout time.Duration) ([]byte, error)
}

// Config wires the reel builder to the newsroom's data, models, storage and renderer.
type Config struct {
	Store       *Store
	Storylines  *storyline.Store
	News        *publish.Store
	Model       completer
	Illustrator illustrator
	ImageModel  string
	Media       *media.Store
	Library     Library
	Renderer    Renderer
	HTTP        *http.Client
	Logger      *slog.Logger
	// Radar, when set, chooses the story from YouTube, Google, web research and coverage signals.
	Radar *trends.Radar
}

type Service struct{ config Config }

func NewService(config Config) *Service {
	if config.HTTP == nil {
		config.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	return &Service{config: config}
}

func (service *Service) Store() *Store { return service.config.Store }

// Ready reports what this worker is missing to render reels (Chrome, ffmpeg).
func (service *Service) Ready() error { return service.config.Renderer.Ready() }

// A storyline gets a new reel only when its latest event changed and its last reel is this old.
const (
	repeatWindow = 7 * 24 * time.Hour
	repeatGap    = 12 * time.Hour
)

const imageStyle = "Photorealistic cinematic news photograph in a vertical 9:16 portrait composition. Dramatic, moody lighting with deep navy shadows and warm golden highlights, shallow depth of field, rich detail. Present-day Sri Lanka where a setting is shown. No text, letters, numbers, signs, logos or watermarks. No real or identifiable person and no likeness of any public figure; any people are anonymous, seen from behind or in silhouette. Scene: "

// Run builds one reel. With an empty id it is the scheduled run: it picks a storyline and only
// creates a draft when one is suitable. Failures are recorded on the draft, not retried.
func (service *Service) Run(ctx context.Context, id string, rerender bool) error {
	logger := service.config.Logger
	if err := service.config.Renderer.Ready(); err != nil {
		return fmt.Errorf("reels cannot render on this worker: %w", err)
	}
	if id == "" {
		storylineID, trend, err := service.pick(ctx)
		if errors.Is(err, ErrNothingToReel) {
			logger.InfoContext(ctx, "scheduled reel skipped", "reason", err)
			return nil
		}
		if err != nil {
			return err
		}
		if id, err = service.config.Store.Create(ctx, "schedule", storylineID, nil, trend); err != nil {
			return err
		}
	}
	if err := service.config.Store.Start(ctx, id); err != nil {
		if errors.Is(err, ErrConflict) {
			return nil
		}
		return err
	}
	started := time.Now()
	if err := service.build(ctx, id, rerender); err != nil {
		logger.WarnContext(ctx, "reel build failed", "reel_id", id, "error", err)
		return service.config.Store.Fail(context.WithoutCancel(ctx), id, err)
	}
	logger.InfoContext(ctx, "reel ready", "reel_id", id, "seconds", int(time.Since(started).Seconds()))
	return nil
}

func (service *Service) build(ctx context.Context, id string, rerender bool) error {
	store := service.config.Store
	reel, err := store.Get(ctx, id)
	if err != nil {
		return err
	}
	if rerender && len(reel.Assets.Photos) > 0 && reel.Script.Question != "" {
		return service.renderAndSave(ctx, reel)
	}

	storylineID := ""
	if reel.StorylineID != nil {
		storylineID = *reel.StorylineID
	}
	if storylineID == "" {
		picked, trend, err := service.pick(ctx)
		if err != nil {
			return err
		}
		if err := store.SetStory(ctx, id, picked, trend); err != nil {
			return err
		}
		storylineID = picked
	}
	story, err := service.config.Storylines.Get(ctx, storylineID, false)
	if err != nil {
		return fmt.Errorf("load storyline: %w", err)
	}
	if story.LatestEvent == nil {
		return fmt.Errorf("the storyline has no public event yet")
	}
	event, err := service.config.News.GetEvent(ctx, story.LatestEvent.ID)
	if err != nil {
		return fmt.Errorf("load latest event: %w", err)
	}
	input := storyInput{Title: story.Title, Summary: story.Summary, LatestTitle: story.LatestEvent.Title}
	if story.LatestEvent.Summary != nil {
		input.LatestSummary = *story.LatestEvent.Summary
	}
	if event.Analysis != nil {
		input.Analysis = event.Analysis.Summary
	}
	for _, article := range event.Articles {
		r := report{Source: article.Source.Name, Headline: article.Headline}
		if article.Excerpt != nil {
			r.Excerpt = *article.Excerpt
		}
		input.Reports = append(input.Reports, r)
	}
	for i, earlier := range story.Events {
		if i == 0 || i > 8 {
			continue
		}
		input.Earlier = append(input.Earlier, earlier.Time.Format("2006-01-02")+": "+earlier.Title)
	}
	script, err := writeScript(ctx, service.config.Model, input)
	if err != nil {
		return fmt.Errorf("write script: %w", err)
	}
	photos, err := service.gatherImages(ctx, id, script)
	if err != nil {
		return err
	}
	music := service.chooseMusic(ctx)
	caption := buildCaption(script, photos)
	if err := store.SaveDraft(ctx, id, story.LatestEvent.ID, story.Title, script, caption, Assets{Photos: photos, Music: &music}); err != nil {
		return err
	}
	if reel, err = store.Get(ctx, id); err != nil {
		return err
	}
	return service.renderAndSave(ctx, reel)
}

// pick chooses the storyline for the next reel from the public trending list, skipping stories
// reeled recently without a new development, and lets the model rank the rest against Google Trends.
func (service *Service) pick(ctx context.Context) (string, Trend, error) {
	if service.config.Radar != nil {
		storylineID, trend, err := service.pickFromRadar(ctx)
		if err == nil || errors.Is(err, ErrNothingToReel) {
			return storylineID, trend, err
		}
		service.config.Logger.WarnContext(ctx, "trend radar unavailable; ranking trending storylines instead", "error", err)
	}
	return service.pickFromStorylines(ctx)
}

// pickFromRadar takes the highest-ranked ready radar topic whose storyline is public and has no
// reel for its latest development. A radar run younger than 90 minutes is reused.
func (service *Service) pickFromRadar(ctx context.Context) (string, Trend, error) {
	run, err := service.config.Radar.Fresh(ctx, 90*time.Minute)
	if err != nil {
		return "", Trend{}, err
	}
	recent, err := service.config.Store.recentStories(ctx, repeatWindow)
	if err != nil {
		return "", Trend{}, err
	}
	now := time.Now()
	for _, topic := range run.Topics {
		if topic.Status != "ready" || topic.StorylineID == nil {
			continue
		}
		story, err := service.config.Storylines.Get(ctx, *topic.StorylineID, false)
		if err != nil || story.LatestEvent == nil {
			continue
		}
		if last, ok := recent[story.ID]; ok && (last.eventID == story.LatestEvent.ID || now.Sub(last.at) < repeatGap) {
			continue
		}
		return story.ID, radarTrend(topic), nil
	}
	return "", Trend{}, ErrNothingToReel
}

func radarTrend(topic trends.Topic) Trend {
	trend := Trend{Score: int(math.Round(topic.Score)), GoogleTrends: []string{}, Reason: topic.Reason, Topic: topic.TitleEN, Rank: topic.Rank}
	for _, g := range topic.Evidence.Google {
		trend.GoogleTrends = append(trend.GoogleTrends, g.Query)
	}
	for _, v := range topic.Evidence.YouTube {
		trend.YouTube = append(trend.YouTube, fmt.Sprintf("%s (%s, %s views, %.1f× usual) %s", v.Title, v.Channel, compact(v.Views), v.Outlier, v.URL))
	}
	for _, w := range topic.Evidence.Web {
		for _, link := range w.Evidence {
			trend.Web = append(trend.Web, link.URL)
		}
	}
	return trend
}

func compact(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.0fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

// pickFromStorylines is the fallback: public trending storylines ranked against Google Trends.
func (service *Service) pickFromStorylines(ctx context.Context) (string, Trend, error) {
	trending, err := service.config.Storylines.List(ctx, false)
	if err != nil {
		return "", Trend{}, fmt.Errorf("list trending storylines: %w", err)
	}
	recent, err := service.config.Store.recentStories(ctx, repeatWindow)
	if err != nil {
		return "", Trend{}, err
	}
	now := time.Now()
	var candidates []candidate
	scores := map[string]int{}
	for _, story := range trending {
		if story.LatestEvent == nil {
			continue
		}
		if last, ok := recent[story.ID]; ok && (last.eventID == story.LatestEvent.ID || now.Sub(last.at) < repeatGap) {
			continue
		}
		candidates = append(candidates, candidate{ID: story.ID, Title: story.Title, Summary: story.Summary,
			Latest: story.LatestEvent.Title, Score: story.Score, SourceCount: story.SourceCount})
		scores[story.ID] = story.Score
	}
	if len(candidates) == 0 {
		return "", Trend{}, ErrNothingToReel
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
	google, err := trends.FetchGoogleTrends(ctx, service.config.HTTP)
	if err != nil {
		service.config.Logger.WarnContext(ctx, "Google Trends unavailable for reel pick", "error", err)
	}
	ranking, err := rankStories(ctx, service.config.Model, candidates, google)
	if err != nil {
		service.config.Logger.WarnContext(ctx, "reel story ranking failed; using trend score", "error", err)
		return candidates[0].ID, Trend{Score: candidates[0].Score, GoogleTrends: []string{}, Reason: "Highest coverage trend score (AI ranking unavailable)."}, nil
	}
	if len(ranking) == 0 {
		return "", Trend{}, ErrNothingToReel
	}
	best := ranking[0]
	if best.GoogleTrends == nil {
		best.GoogleTrends = []string{}
	}
	return best.StorylineID, Trend{Score: scores[best.StorylineID], GoogleTrends: best.GoogleTrends, Reason: best.Reason}, nil
}

// gatherImages finds up to two freely licensed photos and draws two symbolic AI images, stores
// them under reels/<id>/ and returns them interleaved (photo, AI, photo, AI).
func (service *Service) gatherImages(ctx context.Context, id string, script Script) ([]Photo, error) {
	logger := service.config.Logger
	var found []*commonsImage
	seen := map[string]bool{}
	add := func(image *commonsImage, err error, what string) {
		if err != nil {
			logger.WarnContext(ctx, "reel photo lookup failed", "query", what, "error", err)
			return
		}
		if image != nil && !seen[image.Title] && len(found) < 2 {
			seen[image.Title] = true
			found = append(found, image)
		}
	}
	for _, person := range script.People {
		image, err := personPhoto(ctx, service.config.HTTP, person)
		add(image, err, person)
	}
	for _, query := range script.PhotoQueries {
		image, err := searchPhoto(ctx, service.config.HTTP, query)
		add(image, err, query)
	}
	var real []Photo
	for i, image := range found {
		data, contentType, err := fetch(ctx, service.config.HTTP, image.URL, 20<<20)
		if err != nil {
			logger.WarnContext(ctx, "reel photo download failed", "file", image.Title, "error", err)
			continue
		}
		mime := http.DetectContentType(data)
		if !strings.HasPrefix(mime, "image/") {
			mime = contentType
		}
		key := fmt.Sprintf("reels/%s/photo-%d%s", id, i+1, extension(mime))
		if err := service.config.Media.Put(ctx, key, mime, data); err != nil {
			return nil, err
		}
		real = append(real, Photo{Key: key, MIME: mime, Kind: "photo", Credit: image.Credit(), License: image.License, SourceURL: "https://commons.wikimedia.org/wiki/" + strings.ReplaceAll(image.Title, " ", "_")})
	}

	generated := make([]*Photo, len(script.ImagePrompts))
	var wait sync.WaitGroup
	for i, prompt := range script.ImagePrompts {
		if service.config.Illustrator == nil || service.config.ImageModel == "" {
			break
		}
		wait.Add(1)
		go func(i int, prompt string) {
			defer wait.Done()
			data, err := service.config.Illustrator.GenerateImage(ctx, service.config.ImageModel, imageStyle+prompt, 8*time.Minute)
			if err != nil {
				logger.WarnContext(ctx, "reel image generation failed", "image", i+1, "error", err)
				return
			}
			mime := http.DetectContentType(data)
			key := fmt.Sprintf("reels/%s/ai-%d%s", id, i+1, extension(mime))
			if err := service.config.Media.Put(ctx, key, mime, data); err != nil {
				logger.WarnContext(ctx, "store reel image", "error", err)
				return
			}
			generated[i] = &Photo{Key: key, MIME: mime, Kind: "ai", Credit: aiCredit}
		}(i, prompt)
	}
	wait.Wait()
	var photos []Photo
	for i := 0; i < len(real) || i < len(generated); i++ {
		if i < len(real) {
			photos = append(photos, real[i])
		}
		if i < len(generated) && generated[i] != nil {
			photos = append(photos, *generated[i])
		}
	}
	if len(photos) == 0 {
		return nil, fmt.Errorf("no images: no reusable photo was found and image generation failed")
	}
	return photos, nil
}

func extension(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}

func (service *Service) chooseMusic(ctx context.Context) Music {
	recent, err := service.config.Store.lastMusic(ctx, 6)
	if err != nil {
		service.config.Logger.WarnContext(ctx, "recent reel music unavailable", "error", err)
	}
	if t, _, ok := service.config.Library.choose(recent); ok {
		return Music{File: t.File, Title: t.Title, Start: t.Start}
	}
	return Music{File: "thulawa-epic-score", Title: "Thulawa epic score (generated)", Generated: true}
}

// renderAndSave draws the reel from the saved script and images and stores the video and cover.
func (service *Service) renderAndSave(ctx context.Context, reel Reel) error {
	var shots []shot
	for _, photo := range reel.Assets.Photos {
		body, _, err := service.config.Media.Open(ctx, photo.Key)
		if err != nil {
			return fmt.Errorf("open reel image %s: %w", photo.Key, err)
		}
		data, err := io.ReadAll(body)
		body.Close()
		if err != nil {
			return err
		}
		credit := photo.Credit
		if photo.Kind == "photo" {
			credit = "ඡායාරූපය: " + credit
		}
		shots = append(shots, shot{Data: data, MIME: photo.MIME, Credit: credit})
	}
	music := reel.Assets.Music
	if music == nil {
		chosen := service.chooseMusic(ctx)
		music = &chosen
	}
	audioPath, start := filepath.Join(service.config.Library.Dir, music.File), music.Start
	if music.Generated || !fileExists(audioPath) {
		score, err := os.CreateTemp("", "thulawa-score-*.wav")
		if err != nil {
			return err
		}
		defer os.Remove(score.Name())
		if _, err := score.Write(EpicScore(reelSeconds, 7)); err != nil {
			score.Close()
			return err
		}
		score.Close()
		audioPath, start = score.Name(), 0
	}
	video, cover, err := service.config.Renderer.render(ctx, reel.Script, shots, audioPath, start)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	stamp := time.Now().UTC().Format("20060102T150405")
	videoKey := fmt.Sprintf("reels/%s/reel-%s.mp4", reel.ID, stamp)
	coverKey := fmt.Sprintf("reels/%s/cover-%s.jpg", reel.ID, stamp)
	if err := service.config.Media.Put(ctx, videoKey, "video/mp4", video); err != nil {
		return err
	}
	if err := service.config.Media.Put(ctx, coverKey, "image/jpeg", cover); err != nil {
		return err
	}
	if err := service.config.Store.SaveRendered(ctx, reel.ID, videoKey, coverKey); err != nil {
		return err
	}
	for _, old := range []*string{reel.VideoKey, reel.CoverKey} {
		if old != nil && *old != "" && *old != videoKey && *old != coverKey {
			_ = service.config.Media.Delete(ctx, *old)
		}
	}
	return nil
}
