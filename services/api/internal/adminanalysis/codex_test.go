package adminanalysis

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexEnvironmentDoesNotForwardApplicationSecrets(t *testing.T) {
	t.Setenv("SNAP_DATABASE_URL", "postgres://secret")
	t.Setenv("OPENROUTER_API_KEY", "secret-key")
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("HOME", "/app")
	t.Setenv("CODEX_HOME", "/data/codex")

	environment := codexProcessEnvironment()

	require.Contains(t, environment, "PATH=/usr/bin")
	require.Contains(t, environment, "CODEX_HOME=/data/codex")
	require.NotContains(t, environment, "SNAP_DATABASE_URL=postgres://secret")
	require.NotContains(t, environment, "OPENROUTER_API_KEY=secret-key")
}

type fakeProcessRunner struct {
	path    string
	outputs map[string]processResult
}

type processResult struct {
	stdout string
	stderr string
	err    error
}

func (runner fakeProcessRunner) LookPath(string) (string, error) {
	if runner.path == "" {
		return "", errors.New("not found")
	}
	return runner.path, nil
}

func (runner fakeProcessRunner) Run(_ context.Context, _ string, arguments []string, _ string, _ string) (string, string, error) {
	result := runner.outputs[arguments[0]]
	return result.stdout, result.stderr, result.err
}

func TestCodexProbeReportsReadyChatGPTAuthentication(t *testing.T) {
	client := NewCodexClient(fakeProcessRunner{
		path: "/usr/local/bin/codex",
		outputs: map[string]processResult{
			"--version": {stdout: "codex-cli 0.147.0\n"},
			"login":     {stdout: "Logged in using ChatGPT\n"},
		},
	})

	status := client.Probe(context.Background())

	require.True(t, status.Installed)
	require.True(t, status.Authenticated)
	require.True(t, status.Ready)
	require.Equal(t, "chatgpt", status.AuthMethod)
	require.Equal(t, "0.147.0", status.Version)
}

func TestCodexProbeReportsMissingBinary(t *testing.T) {
	client := NewCodexClient(fakeProcessRunner{})

	status := client.Probe(context.Background())

	require.False(t, status.Installed)
	require.False(t, status.Ready)
	require.Contains(t, status.Detail, "not installed")
}

type recordingProcessRunner struct {
	arguments []string
}

func (*recordingProcessRunner) LookPath(string) (string, error) {
	return "/usr/local/bin/codex", nil
}

func (runner *recordingProcessRunner) Run(_ context.Context, _ string, arguments []string, _ string, _ string) (string, string, error) {
	runner.arguments = append([]string(nil), arguments...)
	return "politics", "", nil
}

func TestCodexCompletionAllowsTasksWithoutStructuredSchema(t *testing.T) {
	runner := &recordingProcessRunner{}
	client := NewCodexClient(runner)

	output, err := client.Complete(context.Background(), "gpt-test", "Classify this article", nil)

	require.NoError(t, err)
	require.Equal(t, "politics", output)
	require.False(t, slices.Contains(runner.arguments, "--output-schema"))
}
