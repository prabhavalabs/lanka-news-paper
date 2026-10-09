package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const successfulEvents = `{"type":"item.completed","item":{"type":"agent_message","text":"first"}}
{"type":"item.completed","item":{"type":"agent_message","text":"politics"}}
{"type":"turn.completed","usage":{"input_tokens":12,"cached_input_tokens":7,"output_tokens":4,"reasoning_output_tokens":2}}
`

type fakeRunner struct {
	missing bool
	run     func(context.Context, []string, string, string) (string, string, error)
}

func (runner fakeRunner) LookPath(string) (string, error) {
	if runner.missing {
		return "", errors.New("not found")
	}
	return "/fake/codex", nil
}
func (runner fakeRunner) Run(ctx context.Context, _ string, args []string, stdin, directory string) (string, string, error) {
	if args[0] == "login" && runner.run == nil {
		return "Logged in using ChatGPT", "", nil
	}
	if runner.run != nil {
		return runner.run(ctx, args, stdin, directory)
	}
	if args[0] == "--version" {
		return "codex-cli 0.162.0", "", nil
	}
	if args[0] == "login" {
		return "", "Logged in using ChatGPT", nil
	}
	return successfulEvents, "", nil
}

func TestProbeRequiresChatGPTLogin(t *testing.T) {
	status := NewClient(fakeRunner{}).Probe(context.Background())
	require.True(t, status.Ready)
	require.Equal(t, "chatgpt", status.AuthMethod)
	require.Equal(t, "0.162.0", status.Version)
	require.False(t, NewClient(fakeRunner{missing: true}).Probe(context.Background()).Installed)
	status = NewClient(fakeRunner{run: func(_ context.Context, args []string, _, _ string) (string, string, error) {
		if args[0] == "login" {
			return "Logged in using an API key", "", nil
		}
		return "codex-cli 0.162.0", "", nil
	}}).Probe(context.Background())
	require.True(t, status.Authenticated)
	require.False(t, status.Ready)
}

func TestCompletionArgumentsAndIsolatedFiles(t *testing.T) {
	var workspace string
	runner := fakeRunner{run: func(_ context.Context, args []string, stdin, directory string) (string, string, error) {
		if args[0] == "login" {
			return "Logged in using ChatGPT", "", nil
		}
		workspace = directory
		require.Equal(t, "headline", stdin)
		require.Equal(t, "exec", args[0])
		require.Equal(t, "-", args[len(args)-1])
		for _, flag := range []string{"--ephemeral", "--ignore-user-config", "--ignore-rules", "--skip-git-repo-check", "--json"} {
			require.Contains(t, args, flag)
		}
		for _, pair := range [][2]string{{"--sandbox", "read-only"}, {"--color", "never"}, {"--model", "gpt-6-luna"}, {"--disable", "shell_tool"}, {"--disable", "unified_exec"}, {"--disable", "browser_use"}, {"--disable", "computer_use"}, {"--disable", "apps"}, {"--disable", "multi_agent"}} {
			require.Contains(t, strings.Join(args, "\n"), strings.Join(pair[:], "\n"))
		}
		require.Contains(t, args, `web_search="disabled"`)
		require.Contains(t, args, `model_reasoning_effort="medium"`)
		require.Contains(t, args, "model_instructions_file="+strconv.Quote(filepath.Join(directory, "instructions.txt")))
		data, err := os.ReadFile(filepath.Join(directory, "instructions.txt"))
		require.NoError(t, err)
		require.Contains(t, string(data), "system prompt")
		require.NotContains(t, string(data), "headline")
		data, err = os.ReadFile(filepath.Join(directory, "output-schema.json"))
		require.NoError(t, err)
		require.JSONEq(t, `{"type":"object"}`, string(data))
		entries, err := os.ReadDir(directory)
		require.NoError(t, err)
		require.Len(t, entries, 2)
		return successfulEvents, "", nil
	}}
	result, err := NewClient(runner).Complete(context.Background(), CallOptions{Model: "gpt-6-luna", ReasoningEffort: "medium", System: "system prompt", Input: "headline", JSONSchema: map[string]any{"type": "object"}})
	require.NoError(t, err)
	require.Equal(t, "politics", result.Text)
	require.Equal(t, 12, result.InputTokens)
	require.Equal(t, 7, result.CachedInputTokens)
	require.Equal(t, 4, result.OutputTokens)
	require.Equal(t, 2, result.ReasoningTokens)
	_, err = os.Stat(workspace)
	require.True(t, os.IsNotExist(err))
}

func TestJSONLFailuresAndWarnings(t *testing.T) {
	_, err := parseJSONL(`{"type":"item.completed","item":{"type":"error","message":"Unrecognized config key: obsolete"}}` + "\n" + successfulEvents)
	require.NoError(t, err)
	for _, test := range []struct {
		output string
		kind   error
	}{
		{`{"type":"item.completed","item":{"type":"error","message":"You've hit your usage limit"}}`, ErrQuota},
		{`{"type":"turn.failed","error":{"message":"401 Unauthorized"}}`, ErrAuth},
		{`{"type":"error","message":"network disconnected"}`, nil},
		{`{"type":"item.completed","item":{"type":"agent_message","text":"partial"}}`, nil},
		{`{"type":"turn.completed","usage":{}}`, nil},
		{`invalid`, nil},
	} {
		_, err := parseJSONL(test.output)
		require.Error(t, err)
		if test.kind != nil {
			require.ErrorIs(t, err, test.kind)
		}
	}
}

func TestClassifyError(t *testing.T) {
	for _, test := range []struct {
		message string
		kind    error
	}{
		{"401 Unauthorized", ErrAuth}, {"HTTP status: 401", ErrAuth},
		{"Not logged in", ErrAuth}, {"refresh_token_reused", ErrAuth}, {"token_expired", ErrAuth},
		{"Your access token could not be refreshed", ErrAuth}, {"Please log in again", ErrAuth},
		{"You've hit your usage limit", ErrQuota}, {"Rate limit reached", ErrQuota},
		{"rate_limit_exceeded", ErrQuota}, {"exceeded your current quota", ErrQuota},
		{"429 Too Many Requests", ErrQuota}, {"status 429", ErrQuota},
		{"line 401 of schema", nil}, {"article mentions a fishing quota", nil},
		{"authentication field missing from schema", nil}, {"rate limit configuration invalid", nil},
		{"429 bytes received", nil}, {"1401 Unauthorized", nil}, {"refresh_token field invalid", nil},
		{"connection refused", nil},
	} {
		t.Run(test.message, func(t *testing.T) {
			err := classifyError(test.message)
			if test.kind != nil {
				require.ErrorIs(t, err, test.kind)
			} else {
				require.NotErrorIs(t, err, ErrAuth)
				require.NotErrorIs(t, err, ErrQuota)
			}
		})
	}
}

func TestCompletionDoesNotClassifyModelOrRunnerText(t *testing.T) {
	for _, failed := range []bool{false, true} {
		client := NewClient(fakeRunner{run: func(_ context.Context, args []string, _, _ string) (string, string, error) {
			if args[0] == "login" {
				return "Logged in using ChatGPT", "", nil
			}
			output := `{"type":"item.completed","item":{"type":"agent_message","text":"401 Unauthorized; You've hit your usage limit"}}`
			if failed {
				return output, "generic failure", errors.New("runner reports status 429")
			}
			return output + "\n" + `{"type":"turn.completed","usage":{}}`, "", nil
		}})
		_, err := client.Complete(context.Background(), CallOptions{Model: "gpt-6-luna"})
		if failed {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		require.NotErrorIs(t, err, ErrAuth)
		require.NotErrorIs(t, err, ErrQuota)
	}
}

func TestCompletionClassifiesStderrAndUnavailable(t *testing.T) {
	_, err := NewClient(fakeRunner{missing: true}).Complete(context.Background(), CallOptions{Model: "model"})
	require.ErrorIs(t, err, ErrUnavailable)
	_, err = NewClient(fakeRunner{run: func(context.Context, []string, string, string) (string, string, error) {
		return "", "401 Unauthorized", errors.New("exit status 1")
	}}).Complete(context.Background(), CallOptions{Model: "model"})
	require.ErrorIs(t, err, ErrAuth)
}

func TestProcessEnvironmentExcludesSecrets(t *testing.T) {
	t.Setenv("SNAP_DATABASE_URL", "postgres://secret")
	t.Setenv("OPENAI_API_KEY", "secret")
	t.Setenv("CODEX_HOME", "/account")
	environment := processEnvironment()
	require.Contains(t, environment, "CODEX_HOME=/account")
	require.NotContains(t, environment, "SNAP_DATABASE_URL=postgres://secret")
	require.NotContains(t, environment, "OPENAI_API_KEY=secret")
}

func TestConcurrencyWaitRespectsContext(t *testing.T) {
	firstStarted, release := make(chan struct{}), make(chan struct{})
	calls := make(chan struct{}, 2)
	client := NewClient(fakeRunner{run: func(ctx context.Context, args []string, _, _ string) (string, string, error) {
		if args[0] == "login" {
			return "Logged in using ChatGPT", "", nil
		}
		calls <- struct{}{}
		close(firstStarted)
		select {
		case <-release:
			return successfulEvents, "", nil
		case <-ctx.Done():
			return "", "", ctx.Err()
		}
	}})
	client.slots = make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() { _, err := client.Complete(context.Background(), CallOptions{Model: "model"}); done <- err }()
	<-firstStarted
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Complete(ctx, CallOptions{Model: "model"})
	require.ErrorIs(t, err, context.Canceled)
	timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelTimeout()
	_, err = client.Complete(timeoutCtx, CallOptions{Model: "model"})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Len(t, calls, 1)
	close(release)
	require.NoError(t, <-done)
	require.Empty(t, client.slots)
	require.Equal(t, NewClient(fakeRunner{}).slots, NewClient(fakeRunner{}).slots)
}

func TestRecheckAfterConcurrencyWaitDoesNotSpawn(t *testing.T) {
	client := NewClient(fakeRunner{run: func(context.Context, []string, string, string) (string, string, error) {
		t.Fatal("must not spawn")
		return "", "", nil
	}})
	_, err := client.Complete(context.Background(), CallOptions{Model: "model", BeforeRun: func(context.Context) error { return ErrQuota }})
	require.ErrorIs(t, err, ErrQuota)
}

func TestCodexSmoke(t *testing.T) {
	if os.Getenv("SNAP_CODEX_SMOKE") != "1" {
		t.Skip("SNAP_CODEX_SMOKE is not enabled")
	}
	client := NewClient(nil)
	require.True(t, client.Probe(context.Background()).Ready, "ChatGPT login is required")
	started := time.Now()
	result, err := client.Complete(context.Background(), CallOptions{
		Model: "gpt-6-luna", ReasoningEffort: "low", Timeout: 90 * time.Second,
		System:     "Classify the headline. Return the category politics or sport in the required JSON object.",
		Input:      "Sri Lankan parliament debates electoral reforms.",
		JSONSchema: map[string]any{"type": "object", "properties": map[string]any{"category": map[string]any{"type": "string", "enum": []string{"politics", "sport"}}}, "required": []string{"category"}, "additionalProperties": false},
	})
	require.NoError(t, err)
	var answer map[string]string
	require.NoError(t, json.Unmarshal([]byte(result.Text), &answer))
	require.Equal(t, "politics", answer["category"])
	t.Logf("latency_ms=%d input_tokens=%d cached_input_tokens=%d output_tokens=%d reasoning_tokens=%d answer=%s", time.Since(started).Milliseconds(), result.InputTokens, result.CachedInputTokens, result.OutputTokens, result.ReasoningTokens, result.Text)
}

func TestCompletionWithoutSchemaAndEffort(t *testing.T) {
	runner := fakeRunner{run: func(_ context.Context, args []string, _, _ string) (string, string, error) {
		if args[0] == "login" {
			return "Logged in using ChatGPT", "", nil
		}
		require.NotContains(t, args, "--output-schema")
		require.NotContains(t, strings.Join(args, "\n"), "model_reasoning_effort")
		return successfulEvents, "", nil
	}}
	_, err := NewClient(runner).Complete(context.Background(), CallOptions{Model: "gpt-6-luna"})
	require.NoError(t, err)
}

func TestLoginCancellationDoesNotBecomeAnAuthenticationFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := fakeRunner{run: func(context.Context, []string, string, string) (string, string, error) {
		cancel()
		return "", "", context.Canceled
	}}
	_, err := NewClient(runner).Complete(ctx, CallOptions{Model: "gpt-6-luna"})
	require.ErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, ErrAuth)
}

func TestCompletionRejectsStoredAPIKeyLogin(t *testing.T) {
	runner := fakeRunner{run: func(_ context.Context, args []string, _, _ string) (string, string, error) {
		require.Equal(t, "login", args[0])
		return "Logged in using an API key", "", nil
	}}
	_, err := NewClient(runner).Complete(context.Background(), CallOptions{Model: "gpt-6-luna"})
	require.ErrorIs(t, err, ErrAuth)
}

func TestLoginConfirmationCache(t *testing.T) {
	loginCalls, execCalls := 0, 0
	apiKey := false
	client := NewClient(fakeRunner{run: func(_ context.Context, args []string, _, _ string) (string, string, error) {
		if args[0] == "login" {
			loginCalls++
			if apiKey {
				return "Logged in using an API key", "", nil
			}
			return "Logged in using ChatGPT", "", nil
		}
		execCalls++
		if execCalls == 3 {
			return "", "401 Unauthorized", errors.New("exit status 1")
		}
		return successfulEvents, "", nil
	}})
	for range 2 {
		_, err := client.Complete(context.Background(), CallOptions{Model: "gpt-6-luna"})
		require.NoError(t, err)
	}
	require.Equal(t, 1, loginCalls)
	_, err := client.Complete(context.Background(), CallOptions{Model: "gpt-6-luna"})
	require.ErrorIs(t, err, ErrAuth)
	apiKey = true
	_, err = client.Complete(context.Background(), CallOptions{Model: "gpt-6-luna"})
	require.ErrorIs(t, err, ErrAuth)
	require.Equal(t, 2, loginCalls)
	require.Equal(t, 3, execCalls, "API-key login must not execute")
	apiKey = false
	_, err = client.Complete(context.Background(), CallOptions{Model: "gpt-6-luna"})
	require.NoError(t, err)
	client.authMu.Lock()
	client.authConfirmedUntil = time.Now().Add(-time.Second)
	client.authMu.Unlock()
	_, err = client.Complete(context.Background(), CallOptions{Model: "gpt-6-luna"})
	require.NoError(t, err)
	require.Equal(t, 4, loginCalls)
}

func TestCompletionTimeoutStartsAfterSlotAcquisition(t *testing.T) {
	client := NewClient(fakeRunner{run: func(ctx context.Context, args []string, _, _ string) (string, string, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.Positive(t, time.Until(deadline))
		if args[0] == "login" {
			return "Logged in using ChatGPT", "", nil
		}
		return successfulEvents, "", nil
	}})
	client.slots = make(chan struct{}, 1)
	client.slots <- struct{}{}
	done := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		_, err := client.Complete(ctx, CallOptions{Model: "gpt-6-luna", Timeout: 20 * time.Millisecond})
		done <- err
	}()
	timer := time.NewTimer(40 * time.Millisecond)
	defer timer.Stop()
	<-timer.C
	<-client.slots
	require.NoError(t, <-done)
}
