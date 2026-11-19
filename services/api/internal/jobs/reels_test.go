package jobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReelScheduleRunsEverySixHoursColomboTime(t *testing.T) {
	schedule := reelSchedule()
	colombo := time.FixedZone("Colombo", 5*3600+1800)
	next := schedule.Next(time.Date(2026, 11, 19, 13, 5, 0, 0, colombo))
	require.True(t, next.Equal(time.Date(2026, 11, 19, 18, 0, 0, 0, colombo)), next)
	next = schedule.Next(time.Date(2026, 11, 19, 18, 0, 0, 0, colombo))
	require.True(t, next.Equal(time.Date(2026, 11, 20, 0, 0, 0, 0, colombo)), next)
}
