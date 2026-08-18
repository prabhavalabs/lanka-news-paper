package httpapi

import (
	"testing"
	"time"

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
