// Package card renders self-contained, publisher-photo-free editorial cards.
package card

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"
	"unicode/utf8"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type Kind string

const (
	KindBreaking Kind = "breaking"
	KindTopStory Kind = "top_story"
)

type Spectrum struct{ Left, Center, Right int }

// SourceMark places one credited source on the card's coverage spectrum.
type SourceMark struct {
	Name     string
	Logo     []byte   // optional publisher logo; shown only for allowed image types
	LogoType string   // MIME type of Logo
	Lean     *float64 // -1 (left) .. 1 (right); nil when the source's coverage was not analysed
}

type Card struct {
	Kind        Kind
	Kicker      string
	Headline    string
	Summary     string
	Sources     []string
	Marks       []SourceMark // optional; replaces Sources in the coverage section when set
	SourceCount int
	Spectrum    *Spectrum
	PublishedAt time.Time
	SiteURL     string // optional footer link; empty shows the brand name instead
}

// ScaleFactor renders cards at twice their CSS size (2160×2700) so text stays sharp after Facebook recompresses them.
const ScaleFactor = 2

type Renderer interface {
	Render(context.Context, Card) ([]byte, error)
}
type Options struct {
	ChromePath string
	Timeout    time.Duration
}

type ChromeRenderer struct {
	path      string
	timeout   time.Duration
	noSandbox bool
}

var _ Renderer = (*ChromeRenderer)(nil)

// Serializes all renderer instances, not just calls on a single instance.
var renderMu sync.Mutex

func Validate(c Card) error {
	if c.Kind != KindBreaking && c.Kind != KindTopStory {
		return fmt.Errorf("invalid card kind %q", c.Kind)
	}
	if !utf8.ValidString(c.Headline) || strings.TrimSpace(c.Headline) == "" || utf8.RuneCountInString(c.Headline) > 90 {
		return fmt.Errorf("headline must contain 1..90 UTF-8 runes")
	}
	if !utf8.ValidString(c.Summary) || utf8.RuneCountInString(c.Summary) > 280 {
		return fmt.Errorf("summary must contain at most 280 UTF-8 runes")
	}
	if c.SourceCount < 1 || c.SourceCount > 10000 || len(c.Sources) > c.SourceCount {
		return fmt.Errorf("source count must be 1..10000 and cover the credited sources")
	}
	for _, s := range c.Sources {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("source names must not be empty")
		}
	}
	if len(c.Marks) > c.SourceCount {
		return fmt.Errorf("source count must cover the source marks")
	}
	for _, m := range c.Marks {
		if strings.TrimSpace(m.Name) == "" {
			return fmt.Errorf("source names must not be empty")
		}
		if m.Lean != nil && (*m.Lean < -1 || *m.Lean > 1) {
			return fmt.Errorf("source lean must be -1..1")
		}
	}
	if c.Spectrum != nil {
		for _, v := range []int{c.Spectrum.Left, c.Spectrum.Center, c.Spectrum.Right} {
			if v < 0 || v > 100 {
				return fmt.Errorf("spectrum percentages must be 0..100")
			}
		}
	}
	return nil
}

func NewChromeRenderer(opts Options) (*ChromeRenderer, error) {
	path := opts.ChromePath
	if path == "" {
		path = os.Getenv("SNAP_SOCIAL_CHROME_PATH")
	}
	if path == "" {
		for _, candidate := range []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "chromium", "chromium-browser", "google-chrome", "google-chrome-stable"} {
			if found, err := exec.LookPath(candidate); err == nil {
				path = found
				break
			}
		}
	}
	if path == "" {
		return nil, fmt.Errorf("Chrome not found; set SNAP_SOCIAL_CHROME_PATH")
	}
	resolved, err := exec.LookPath(path)
	if err != nil {
		return nil, fmt.Errorf("find Chrome: %w", err)
	}
	if opts.Timeout < 0 {
		return nil, fmt.Errorf("render timeout must not be negative")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	noSandbox := os.Getuid() == 0
	if value := os.Getenv("SNAP_SOCIAL_CHROME_NO_SANDBOX"); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("SNAP_SOCIAL_CHROME_NO_SANDBOX: %w", err)
		}
		noSandbox = noSandbox || enabled
	}
	return &ChromeRenderer{path: resolved, timeout: opts.Timeout, noSandbox: noSandbox}, nil
}

func (r *ChromeRenderer) Render(ctx context.Context, c Card) ([]byte, error) {
	document, err := renderHTML(c)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	// TryLock lets cancelled callers leave the queue promptly without leaking a
	// goroutine that could later acquire the mutex. No browser exists while queued.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for !renderMu.TryLock() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
	defer renderMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	flags := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	flags = append(flags, chromedp.ExecPath(r.path), chromedp.Flag("headless", true), chromedp.Flag("no-sandbox", r.noSandbox), chromedp.Flag("disable-gpu", true), chromedp.Flag("disable-dev-shm-usage", true), chromedp.Flag("hide-scrollbars", true), chromedp.Flag("font-render-hinting", "none"))
	alloc, stop := chromedp.NewExecAllocator(ctx, flags...)
	defer stop()
	browser, closeBrowser := chromedp.NewContext(alloc)
	defer closeBrowser()
	if err := chromedp.Do(browser, chromedp.Navigate("about:blank")); err != nil {
		return nil, fmt.Errorf("initialize card document: %w", err)
	}
	// Subscribe before enabling interception. Only data resources may proceed;
	// the inline document is supplied through CDP rather than a network URL.
	events := chromedp.Events(browser, fetch.RequestPaused)
	go func() {
		for ev, err := range events {
			if err != nil {
				return
			}
			target := chromedp.FromContext(browser).Target
			if strings.HasPrefix(ev.Request.URL, "data:") {
				_, _ = cdp.Call(browser, target, fetch.ContinueRequest, fetch.ContinueRequestParams{RequestID: ev.RequestID})
			} else {
				_, _ = cdp.Call(browser, target, fetch.FailRequest, fetch.FailRequestParams{RequestID: ev.RequestID, ErrorReason: network.ErrorReasonBlockedByClient})
			}
		}
	}()
	err = chromedp.Do(browser, chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		if _, err := cdp.Call(ctx, t, fetch.Enable, fetch.EnableParams{}); err != nil {
			return err
		}
		if _, err := cdp.Call(ctx, t, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{Width: 1080, Height: 1350, DeviceScaleFactor: ScaleFactor, Mobile: false}); err != nil {
			return err
		}
		tree, err := cdp.Call(ctx, t, page.GetFrameTree, cdp.Empty{})
		if err != nil {
			return err
		}
		_, err = cdp.Call(ctx, t, page.SetDocumentContent, page.SetDocumentContentParams{FrameID: tree.FrameTree.Frame.ID, HTML: document})
		return err
	}), chromedp.Evaluate[chromedp.Void](fitScript, chromedp.EvalAwaitPromise))
	if err != nil {
		return nil, fmt.Errorf("render card: %w", err)
	}
	png, err := chromedp.Run(browser, chromedp.Screenshot(chromedp.CSS("#card")))
	if err != nil {
		return nil, fmt.Errorf("capture card: %w", err)
	}
	return png, nil
}
