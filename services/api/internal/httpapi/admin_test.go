package httpapi

import (
	"testing"

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
