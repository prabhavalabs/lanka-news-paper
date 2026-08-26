package httpapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseTrendDays(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "default", want: 90},
		{name: "week", value: "7", want: 7},
		{name: "month", value: "30", want: 30},
		{name: "quarter", value: "90", want: 90},
		{name: "unsupported", value: "14", wantErr: true},
		{name: "not a number", value: "many", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseTrendDays(test.value)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestNewAnalysisBackfillRequestUsesInclusiveEndDate(t *testing.T) {
	request, err := newAnalysisBackfillRequest(
		"date_range", "single_pass", "openrouter", "test-model", "2026-08-17", "2026-08-23", "", "",
	)

	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC), *request.From)
	require.Equal(t, time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), *request.To)
}

func TestNewAnalysisBackfillRequestRejectsInvalidDate(t *testing.T) {
	_, err := newAnalysisBackfillRequest(
		"date_range", "single_pass", "openrouter", "test-model", "last-week", "2026-08-23", "", "",
	)

	require.EqualError(t, err, "from must use YYYY-MM-DD")
}
