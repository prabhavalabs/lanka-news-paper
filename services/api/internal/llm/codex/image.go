package codex

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const imageFile = "illustration.png"

// GenerateImage draws one image with the Codex CLI's built-in image tool, which works with the
// ChatGPT login and needs no API key. It returns the raw image bytes Codex produced.
func (client *Client) GenerateImage(ctx context.Context, model, prompt string, timeout time.Duration) ([]byte, error) {
	if !ValidModel(model) {
		return nil, fmt.Errorf("model %q is not in the configured Codex allowlist", model)
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("an image prompt is required")
	}
	if err := acquire(ctx, client.slots); err != nil {
		return nil, err
	}
	defer func() { <-client.slots }()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	path, err := client.runner.LookPath("codex")
	if err != nil {
		return nil, fmt.Errorf("%w: locate executable: %v", ErrUnavailable, err)
	}
	if err := client.confirmLogin(ctx, path); err != nil {
		return nil, err
	}
	directory, err := os.MkdirTemp("", "snap-codex-image-")
	if err != nil {
		return nil, fmt.Errorf("create isolated Codex workspace: %w", err)
	}
	defer os.RemoveAll(directory)
	started := time.Now()
	arguments := []string{
		"exec", "--ephemeral", "--ignore-user-config", "--ignore-rules",
		"--skip-git-repo-check", "--sandbox", "workspace-write", "--color", "never", "--json",
		"--config", `web_search="disabled"`,
		"--config", `forced_login_method="chatgpt"`,
		"--disable", "browser_use", "--disable", "computer_use", "--disable", "apps", "--disable", "multi_agent",
		"--model", model, "-",
	}
	instructions := "Use the built-in image_gen tool exactly once to create the illustration described below. " +
		"Then copy the generated image file into the current working directory as " + imageFile + ". Do nothing else.\n\n" + prompt
	_, stderr, runErr := client.runner.Run(ctx, path, arguments, instructions, directory)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("Codex image generation: %w", ctx.Err())
	}
	if data, err := os.ReadFile(filepath.Join(directory, imageFile)); err == nil && len(data) > 0 {
		removeGenerated(started)
		return data, nil
	}
	// Codex keeps its own copy under CODEX_HOME; use it when the copy step did not happen.
	if data := newestGenerated(started); len(data) > 0 {
		removeGenerated(started)
		return data, nil
	}
	if runErr != nil {
		if classified := classifyError(stderr); classified != nil {
			return nil, classified
		}
		return nil, fmt.Errorf("Codex image generation: %w: %s", runErr, strings.TrimSpace(stderr))
	}
	return nil, fmt.Errorf("Codex image generation produced no image")
}

func generatedDirectory() string {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		user, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		home = filepath.Join(user, ".codex")
	}
	return filepath.Join(home, "generated_images")
}

func generatedSince(since time.Time) []string {
	root := generatedDirectory()
	if root == "" {
		return nil
	}
	var files []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.Type().IsRegular() {
			return nil
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".png" && extension != ".jpg" && extension != ".jpeg" && extension != ".webp" {
			return nil
		}
		if info, err := entry.Info(); err == nil && !info.ModTime().Before(since) {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func newestGenerated(since time.Time) []byte {
	var newest string
	var newestTime time.Time
	for _, path := range generatedSince(since) {
		if info, err := os.Stat(path); err == nil && info.ModTime().After(newestTime) {
			newest, newestTime = path, info.ModTime()
		}
	}
	if newest == "" {
		return nil
	}
	data, _ := os.ReadFile(newest)
	return data
}

// removeGenerated deletes Codex's own copies so generated images do not accumulate on disk.
func removeGenerated(since time.Time) {
	for _, path := range generatedSince(since) {
		_ = os.Remove(path)
	}
}
