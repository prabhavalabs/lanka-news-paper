package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/publish"
	"github.com/stretchr/testify/require"
)

func TestParseKnowledgeDays(t *testing.T) {
	days, err := parseKnowledgeDays("")
	require.NoError(t, err)
	require.Equal(t, 1, days)

	days, err = parseKnowledgeDays("30")
	require.NoError(t, err)
	require.Equal(t, 30, days)

	_, err = parseKnowledgeDays("365")
	require.EqualError(t, err, "days must be 1, 7, or 30")
}

func TestParseKnowledgeWindow(t *testing.T) {
	now := time.Date(2026, time.August, 17, 15, 30, 0, 0, time.FixedZone("CEST", 2*60*60))
	start, end, err := parseKnowledgeWindow("1", "", "", now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, time.August, 16, 13, 30, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2026, time.August, 17, 13, 30, 0, 0, time.UTC), end)

	start, end, err = parseKnowledgeWindow("", "2026-08-01", "2026-08-07", now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2026, time.August, 8, 0, 0, 0, 0, time.UTC), end)

	_, _, err = parseKnowledgeWindow("", "2026-08-07", "2026-08-01", now)
	require.EqualError(t, err, "to must be on or after from")
}

func TestPublicKnowledgeGraphRejectsInvalidSource(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-graph?source=not-a-uuid", nil)
	recorder := httptest.NewRecorder()
	NewRouter(Dependencies{}).ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestPublicKnowledgeGraphDTOExcludesAdminFields(t *testing.T) {
	payload, err := json.Marshal(publish.KnowledgeGraph{Events: []publish.KnowledgeEvent{{
		ID: "event", Articles: []publish.KnowledgeArticle{{
			ID: "article", Narrative: &publish.KnowledgeNarrative{Label: "neutral"},
		}},
	}}})
	require.NoError(t, err)
	for _, field := range []string{"provider_id", "provider_model", "original_url", "algorithm_version", "locked", "evidence", "rationale"} {
		require.NotContains(t, string(payload), field)
	}
}
