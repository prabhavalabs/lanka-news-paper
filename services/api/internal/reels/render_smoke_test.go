package reels

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRenderSmoke renders a full reel with the generated score; it needs Chrome and ffmpeg, so it
// only runs when SNAP_REEL_RENDER_TEST=1 (for example inside the production image).
func TestRenderSmoke(t *testing.T) {
	if os.Getenv("SNAP_REEL_RENDER_TEST") != "1" {
		t.Skip("SNAP_REEL_RENDER_TEST is not set")
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 720, 1280))
	for y := 0; y < 1280; y++ {
		for x := 0; x < 720; x++ {
			canvas.Set(x, y, color.RGBA{uint8(x / 3), 40, uint8(y / 5), 255})
		}
	}
	var picture bytes.Buffer
	require.NoError(t, png.Encode(&picture, canvas))
	score, err := os.CreateTemp("", "score-*.wav")
	require.NoError(t, err)
	_, err = score.Write(EpicScore(reelSeconds, 7))
	require.NoError(t, err)
	score.Close()
	defer os.Remove(score.Name())

	script := Script{Tag: "අධිකරණ පුවත", Headline: []string{"එයාර්බස් නඩුව", "නාමල් අධිකරණයට"}, Facts: []string{"නාමල් රාජපක්ෂ මහාධිකරණයට ඉදිරිපත් කෙරුණා", "චෝදනා 24ක්", "කොටුවේ නඩුව අවසන්"},
		Question: "මේ නඩු කටයුතු ගැන ඔබ සෑහීමකට පත්වෙනවාද?", YesLabel: "ඔව්", NoLabel: "නැහැ"}
	renderer := Renderer{ChromePath: os.Getenv("SNAP_SOCIAL_CHROME_PATH"), NoSandbox: os.Getuid() == 0}
	require.NoError(t, renderer.Ready())
	started := time.Now()
	video, cover, err := renderer.render(context.Background(), script, []shot{{picture.Bytes(), "image/png", "ඡායාරූපය: Test"}, {picture.Bytes(), "image/png", aiCredit}}, score.Name(), 0)
	require.NoError(t, err)
	t.Logf("rendered %d KB video, %d KB cover in %s", len(video)/1024, len(cover)/1024, time.Since(started).Round(time.Second))
	require.Greater(t, len(video), 200_000)
	require.Greater(t, len(cover), 20_000)
	if out := os.Getenv("SNAP_REEL_RENDER_OUT"); out != "" {
		require.NoError(t, os.WriteFile(out+"/smoke.mp4", video, 0o644))
		require.NoError(t, os.WriteFile(out+"/smoke-cover.jpg", cover, 0o644))
	}
}
