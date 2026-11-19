package reels

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/card"
)

//go:embed page.html
var pageSource string

var pageTemplate = template.Must(template.New("reel").Funcs(template.FuncMap{"inc": func(i int) int { return i + 1 }}).Parse(pageSource))

const (
	fps          = 30
	reelSeconds  = 14.0
	coverSeconds = 4.5
	kicker       = "අද දවසේ කතාබහ"
	aiCredit     = "AI මගින් නිර්මිත නිදර්ශන රූපයකි"
)

// shot is one image in the reel with the credit line shown over it.
type shot struct {
	Data   []byte
	MIME   string
	Credit string
}

type pageShot struct {
	URL    template.URL
	Credit string
}

// Renderer draws reels in headless Chrome and encodes them with ffmpeg.
type Renderer struct {
	ChromePath string
	NoSandbox  bool
}

func (renderer Renderer) chrome() (string, error) {
	for _, candidate := range []string{renderer.ChromePath, "/usr/bin/chromium", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "chromium", "chromium-browser", "google-chrome"} {
		if candidate == "" {
			continue
		}
		if found, err := exec.LookPath(candidate); err == nil {
			return found, nil
		}
	}
	return "", fmt.Errorf("Chrome not found; set SNAP_SOCIAL_CHROME_PATH")
}

// Ready reports what is missing for rendering on this machine.
func (renderer Renderer) Ready() error {
	if _, err := renderer.chrome(); err != nil {
		return err
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg is not installed")
	}
	return nil
}

// render draws every frame and pipes them to ffmpeg with the soundtrack, returning the MP4 and a cover JPEG.
func (renderer Renderer) render(ctx context.Context, script Script, shots []shot, audioPath string, audioStart float64) ([]byte, []byte, error) {
	views := make([]pageShot, len(shots))
	for i, s := range shots {
		views[i] = pageShot{template.URL("data:" + s.MIME + ";base64," + base64.StdEncoding.EncodeToString(s.Data)), s.Credit}
	}
	var document bytes.Buffer
	if err := pageTemplate.Execute(&document, map[string]any{
		"Fonts": card.FontCSS(), "Masthead": card.Masthead(), "Shots": views, "Script": script,
		"Kicker": kicker, "CTA": "poll", "Duration": reelSeconds,
	}); err != nil {
		return nil, nil, err
	}
	path, err := renderer.chrome()
	if err != nil {
		return nil, nil, err
	}
	workDir, err := os.MkdirTemp("", "thulawa-reel-")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(workDir)

	flags := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	flags = append(flags, chromedp.ExecPath(path), chromedp.Flag("headless", true), chromedp.Flag("no-sandbox", renderer.NoSandbox),
		chromedp.Flag("disable-gpu", true), chromedp.Flag("disable-dev-shm-usage", true), chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("font-render-hinting", "none"), chromedp.WindowSize(1080, 1920), chromedp.UserDataDir(filepath.Join(workDir, "chrome")))
	alloc, stopAlloc := chromedp.NewExecAllocator(ctx, flags...)
	defer stopAlloc()
	browser, closeBrowser := chromedp.NewContext(alloc)
	defer closeBrowser()

	if err := chromedp.Do(browser, chromedp.Navigate("about:blank"), chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		if _, err := cdp.Call(ctx, t, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{Width: 1080, Height: 1920, DeviceScaleFactor: 1}); err != nil {
			return err
		}
		tree, err := cdp.Call(ctx, t, page.GetFrameTree, cdp.Empty{})
		if err != nil {
			return err
		}
		_, err = cdp.Call(ctx, t, page.SetDocumentContent, page.SetDocumentContentParams{FrameID: tree.FrameTree.Frame.ID, HTML: document.String()})
		return err
	}), chromedp.Evaluate[chromedp.Void]("prepare()", chromedp.EvalAwaitPromise)); err != nil {
		return nil, nil, fmt.Errorf("load reel page: %w", err)
	}

	output := filepath.Join(workDir, "reel.mp4")
	var ffmpegLog bytes.Buffer
	ffmpeg := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-y",
		"-f", "image2pipe", "-framerate", fmt.Sprint(fps), "-c:v", "mjpeg", "-i", "-",
		"-ss", fmt.Sprintf("%.3f", audioStart), "-i", audioPath, "-map", "0:v", "-map", "1:a",
		"-c:v", "libx264", "-preset", "medium", "-crf", "19", "-pix_fmt", "yuv420p", "-r", fmt.Sprint(fps),
		"-af", fmt.Sprintf("afade=t=in:d=0.03,afade=t=out:st=%.2f:d=1.2,loudnorm=I=-14:TP=-1.5:LRA=11", reelSeconds-1.2),
		"-c:a", "aac", "-b:a", "192k", "-ar", "44100", "-shortest", "-movflags", "+faststart", output)
	ffmpeg.Stderr = &ffmpegLog
	stdin, err := ffmpeg.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := ffmpeg.Start(); err != nil {
		return nil, nil, fmt.Errorf("start ffmpeg: %w", err)
	}
	abort := func(err error) ([]byte, []byte, error) {
		stdin.Close()
		ffmpeg.Wait()
		return nil, nil, err
	}
	var cover []byte
	frames := int(reelSeconds * fps)
	for f := 0; f < frames; f++ {
		at := float64(f) * 1000 / fps
		if err := chromedp.Do(browser, chromedp.Evaluate[chromedp.Void](fmt.Sprintf("seek(%f)", at), chromedp.EvalAwaitPromise)); err != nil {
			return abort(fmt.Errorf("frame %d: %w", f, err))
		}
		frame, err := chromedp.Run(browser, chromedp.Action[[]byte](func(ctx context.Context, t *chromedp.Target) ([]byte, error) {
			quality := int64(92)
			r, err := cdp.Call(ctx, t, page.CaptureScreenshot, page.CaptureScreenshotParams{Format: page.CaptureScreenshotFormatJpeg, Quality: &quality})
			if err != nil {
				return nil, err
			}
			return r.Data, nil
		}))
		if err != nil {
			return abort(fmt.Errorf("frame %d: %w", f, err))
		}
		if f == int(coverSeconds*fps) {
			cover = frame
		}
		if _, err := io.Copy(stdin, bytes.NewReader(frame)); err != nil {
			return abort(fmt.Errorf("write frame %d: %w (%s)", f, err, strings.TrimSpace(ffmpegLog.String())))
		}
	}
	stdin.Close()
	if err := ffmpeg.Wait(); err != nil {
		return nil, nil, fmt.Errorf("ffmpeg: %w (%s)", err, strings.TrimSpace(ffmpegLog.String()))
	}
	video, err := os.ReadFile(output)
	return video, cover, err
}
