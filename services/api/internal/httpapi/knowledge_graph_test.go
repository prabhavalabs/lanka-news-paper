package httpapi

import (
	"testing"

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
