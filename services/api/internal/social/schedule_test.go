package social

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func testSettings() Settings {
	return Settings{Enabled: true, MaxPostsPerHour: 2, MaxPostsPerDay: 12, MinGapMinutes: 20, QuietHoursStart: 0, QuietHoursEnd: 6, MinScore: 70, LookbackHours: 6, CandidatesPerRun: 8, Timezone: "Asia/Colombo", RealtimeEnabled: true, MinReviewScore: 80}
}

func TestNextEligible(t *testing.T) {
	location, err := time.LoadLocation("Asia/Colombo")
	require.NoError(t, err)
	local := func(day, hour, minute int) time.Time {
		return time.Date(2026, 10, 10+day, hour, minute, 0, 0, location)
	}
	tests := []struct {
		name     string
		now      time.Time
		settings Settings
		history  []time.Time
		expected time.Time
	}{
		{"quiet morning", local(0, 3, 0), testSettings(), nil, local(0, 6, 0)},
		{"end boundary", local(0, 6, 0), testSettings(), nil, local(0, 6, 0)},
		{"minimum gap", local(0, 10, 0), testSettings(), []time.Time{local(0, 9, 50)}, local(0, 10, 10)},
		{"rolling hour", local(0, 10, 0), testSettings(), []time.Time{local(0, 9, 10), local(0, 9, 35)}, local(0, 10, 10)},
		{"hour boundary expires", local(0, 10, 10), testSettings(), []time.Time{local(0, 9, 10), local(0, 9, 35)}, local(0, 10, 10)},
	}
	daily := testSettings()
	daily.MaxPostsPerDay = 1
	tests = append(tests, struct {
		name     string
		now      time.Time
		settings Settings
		history  []time.Time
		expected time.Time
	}{"calendar day then quiet", local(0, 23, 0), daily, []time.Time{local(0, 8, 0)}, local(1, 6, 0)})
	overnight := testSettings()
	overnight.QuietHoursStart = 22
	overnight.QuietHoursEnd = 7
	tests = append(tests, struct {
		name     string
		now      time.Time
		settings Settings
		history  []time.Time
		expected time.Time
	}{"overnight quiet", local(0, 23, 0), overnight, nil, local(1, 7, 0)})
	disabled := testSettings()
	disabled.QuietHoursStart = 0
	disabled.QuietHoursEnd = 0
	tests = append(tests, struct {
		name     string
		now      time.Time
		settings Settings
		history  []time.Time
		expected time.Time
	}{"quiet disabled", local(0, 1, 0), disabled, nil, local(0, 1, 0)})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			next, err := NextEligible(test.now, test.settings, test.history)
			require.NoError(t, err)
			require.True(t, next.Equal(test.expected), "got %v, expected %v", next, test.expected)
		})
	}
}

func TestNextEligibleUsesCalendarTimezone(t *testing.T) {
	settings := testSettings()
	settings.MaxPostsPerDay = 1
	settings.QuietHoursStart = 0
	settings.QuietHoursEnd = 0
	now := time.Date(2026, 10, 10, 19, 0, 0, 0, time.UTC) // already the next day in Colombo
	published := time.Date(2026, 10, 10, 16, 0, 0, 0, time.UTC)
	next, err := NextEligible(now, settings, []time.Time{published})
	require.NoError(t, err)
	require.Equal(t, now, next)
	settings.Timezone = "Invalid/Zone"
	_, err = NextEligible(now, settings, nil)
	require.Error(t, err)
}
