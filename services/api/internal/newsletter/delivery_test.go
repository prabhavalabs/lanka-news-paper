package newsletter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDeliveryWindowUsesPreviousColomboMorning(t *testing.T) {
	location, err := time.LoadLocation("Asia/Colombo")
	require.NoError(t, err)
	now := time.Date(2026, 8, 27, 3, 0, 0, 0, time.UTC)

	editionDate, start, end, due := deliveryWindow(now, location, 8)

	require.True(t, due)
	require.Equal(t, "2026-08-27", editionDate)
	require.Equal(t, time.Date(2026, 8, 26, 8, 0, 0, 0, location), start)
	require.Equal(t, time.Date(2026, 8, 27, 8, 0, 0, 0, location), end)
}

func TestDeliveryWindowIsNotDueBeforeMorning(t *testing.T) {
	location, err := time.LoadLocation("Asia/Colombo")
	require.NoError(t, err)
	now := time.Date(2026, 8, 27, 2, 15, 0, 0, time.UTC)

	_, _, _, due := deliveryWindow(now, location, 8)

	require.False(t, due)
}
