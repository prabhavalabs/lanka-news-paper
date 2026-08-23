package desk

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestParseQueueEntryRef(t *testing.T) {
	runID := uuid.New()
	tests := []struct {
		name string
		id   string
		kind string
		ok   bool
	}{
		{name: "river", id: "river:93291", kind: "river", ok: true},
		{name: "pipeline", id: "pipeline:" + runID.String(), kind: "pipeline", ok: true},
		{name: "missing prefix", id: "93291", ok: false},
		{name: "unsupported prefix", id: "cron:12", ok: false},
		{name: "invalid river id", id: "river:-1", ok: false},
		{name: "invalid run id", id: "pipeline:not-a-uuid", ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := parseQueueEntryRef(test.id)
			if !test.ok {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.kind, result.kind)
		})
	}
}

func TestNewQueueJobArtifactPreservesStructuredData(t *testing.T) {
	artifact, err := newQueueJobArtifact("input", "job_request", "Request", "Description", map[string]any{
		"article_id": "article-1",
		"periodic":   true,
	})
	require.NoError(t, err)
	require.Equal(t, "input:job_request", artifact.ID)
	require.JSONEq(t, `{"article_id":"article-1","periodic":true}`, string(artifact.Data))
}

func TestDurationBetween(t *testing.T) {
	start := time.Date(2026, time.August, 23, 1, 0, 0, 0, time.UTC)
	end := start.Add(1250 * time.Millisecond)
	require.Equal(t, int64(1250), *durationBetween(&start, &end))
	require.Nil(t, durationBetween(nil, &end))
}

func TestJSONString(t *testing.T) {
	require.Equal(t, "article-1", jsonString(json.RawMessage(`{"article_id":"article-1"}`), "article_id"))
	require.Empty(t, jsonString(json.RawMessage(`{"article_id":12}`), "article_id"))
	require.Empty(t, jsonString(json.RawMessage(`not-json`), "article_id"))
}
