package adminanalysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var ErrCodexUnavailable = errors.New("Codex CLI is unavailable")

type ProcessRunner interface {
	LookPath(string) (string, error)
	Run(context.Context, string, []string, string, string) (string, string, error)
}

type osProcessRunner struct{}

func (osProcessRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }

func (osProcessRunner) Run(ctx context.Context, name string, arguments []string, stdin, directory string) (string, string, error) {
	command := exec.CommandContext(ctx, name, arguments...)
	command.Dir = directory
	command.Stdin = strings.NewReader(stdin)
	command.Env = codexProcessEnvironment()
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), err
}

func codexProcessEnvironment() []string {
	keys := []string{"PATH", "HOME", "CODEX_HOME", "TMPDIR", "LANG", "LC_ALL", "SSL_CERT_FILE", "SSL_CERT_DIR", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"}
	environment := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			environment = append(environment, key+"="+value)
		}
	}
	return environment
}

type CodexStatus struct {
	Installed     bool      `json:"installed"`
	Authenticated bool      `json:"authenticated"`
	Ready         bool      `json:"ready"`
	Path          string    `json:"path"`
	Version       string    `json:"version"`
	AuthMethod    string    `json:"auth_method"`
	Detail        string    `json:"detail"`
	CheckedAt     time.Time `json:"checked_at"`
	Models        []string  `json:"models"`
}

type CodexClient struct {
	runner ProcessRunner
}

func NewCodexClient(runner ProcessRunner) *CodexClient {
	if runner == nil {
		runner = osProcessRunner{}
	}
	return &CodexClient{runner: runner}
}

func (client *CodexClient) Probe(ctx context.Context) CodexStatus {
	status := CodexStatus{
		CheckedAt: time.Now().UTC(),
		Models:    codexModels(),
	}
	path, err := client.runner.LookPath("codex")
	if err != nil {
		status.Detail = "Codex CLI is not installed or is not available on PATH."
		return status
	}
	status.Installed, status.Path = true, path

	probeContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	version, _, versionErr := client.runner.Run(probeContext, path, []string{"--version"}, "", os.TempDir())
	cancel()
	if versionErr == nil {
		status.Version = parseCodexVersion(version)
	}

	loginContext, cancelLogin := context.WithTimeout(ctx, 8*time.Second)
	stdout, stderr, loginErr := client.runner.Run(loginContext, path, []string{"login", "status"}, "", os.TempDir())
	cancelLogin()
	loginOutput := strings.TrimSpace(stdout + "\n" + stderr)
	if loginErr != nil {
		status.Detail = "Codex CLI is installed but is not authenticated."
		return status
	}
	lower := strings.ToLower(loginOutput)
	if strings.Contains(lower, "logged in using chatgpt") {
		status.Authenticated, status.AuthMethod = true, "chatgpt"
	} else if strings.Contains(lower, "api key") || strings.Contains(lower, "apikey") {
		status.Authenticated, status.AuthMethod = true, "api_key"
	} else if strings.Contains(lower, "logged in") {
		status.Authenticated, status.AuthMethod = true, "authenticated"
	}
	status.Ready = status.Installed && status.Authenticated
	if status.Ready {
		status.Detail = "Codex CLI is installed and authenticated for administrative backfills."
	} else {
		status.Detail = "Codex CLI is installed but authentication could not be confirmed."
	}
	return status
}

func (client *CodexClient) Complete(ctx context.Context, model, prompt string, schema map[string]any) (string, error) {
	path, err := client.runner.LookPath("codex")
	if err != nil {
		return "", fmt.Errorf("%w: locate executable: %v", ErrCodexUnavailable, err)
	}
	directory, err := os.MkdirTemp("", "snap-codex-analysis-")
	if err != nil {
		return "", fmt.Errorf("create isolated Codex workspace: %w", err)
	}
	defer os.RemoveAll(directory)
	arguments := []string{
		"exec", "--ephemeral", "--ignore-user-config", "--ignore-rules",
		"--skip-git-repo-check", "--sandbox", "read-only", "--color", "never",
		"--config", `model_reasoning_effort="low"`,
		"--disable", "shell_tool", "--disable", "unified_exec", "--disable", "browser_use",
		"--disable", "computer_use", "--disable", "apps", "--disable", "multi_agent",
		"--model", model,
	}
	if schema != nil {
		schemaData, err := json.Marshal(schema)
		if err != nil {
			return "", fmt.Errorf("encode Codex output schema: %w", err)
		}
		schemaPath := filepath.Join(directory, "output-schema.json")
		if err := os.WriteFile(schemaPath, schemaData, 0o600); err != nil {
			return "", fmt.Errorf("write Codex output schema: %w", err)
		}
		arguments = append(arguments, "--output-schema", schemaPath)
	}
	arguments = append(arguments, "-")
	stdout, stderr, err := client.runner.Run(ctx, path, arguments, prompt, directory)
	if err != nil {
		detail := truncateRunes(strings.TrimSpace(stderr), 1000)
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("Codex CLI execution failed: %s", detail)
	}
	output := strings.TrimSpace(stdout)
	if output == "" {
		return "", fmt.Errorf("Codex CLI returned an empty response")
	}
	return output, nil
}

var versionPattern = regexp.MustCompile(`(?i)codex(?:-cli)?\s+v?([^\s]+)`)

func parseCodexVersion(value string) string {
	match := versionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) == 2 {
		return match[1]
	}
	return strings.TrimSpace(value)
}

func codexModels() []string {
	value := strings.TrimSpace(os.Getenv("CODEX_BACKFILL_MODELS"))
	if value == "" {
		return []string{"gpt-5.6-luna", "gpt-5.6-terra", "gpt-5.6-sol"}
	}
	items := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}
