package social

import (
	"sort"
	"time"
)

// NextEligible applies rolling hourly limits, local calendar days, the gap and quiet hours.
// Equal quiet-hour endpoints disable the quiet window.
func NextEligible(now time.Time, settings Settings, published []time.Time) (time.Time, error) {
	if err := validateSettings(settings); err != nil {
		return time.Time{}, err
	}
	location, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	history := append([]time.Time(nil), published...)
	sort.Slice(history, func(i, j int) bool { return history[i].Before(history[j]) })
	next := now
	if len(history) > 0 {
		gap := history[len(history)-1].Add(time.Duration(settings.MinGapMinutes) * time.Minute)
		if gap.After(next) {
			next = gap
		}
	}
	for range 32 {
		local := next.In(location)
		hour := local.Hour()
		quiet := settings.QuietHoursStart != settings.QuietHoursEnd && ((settings.QuietHoursStart < settings.QuietHoursEnd && hour >= settings.QuietHoursStart && hour < settings.QuietHoursEnd) || (settings.QuietHoursStart > settings.QuietHoursEnd && (hour >= settings.QuietHoursStart || hour < settings.QuietHoursEnd)))
		if quiet {
			end := time.Date(local.Year(), local.Month(), local.Day(), settings.QuietHoursEnd, 0, 0, 0, location)
			if !end.After(next) {
				end = end.AddDate(0, 0, 1)
			}
			next = end
			continue
		}
		dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
		dayEnd := dayStart.AddDate(0, 0, 1)
		today := 0
		hourly := make([]time.Time, 0)
		for _, publishedAt := range history {
			if !publishedAt.Before(dayStart) && publishedAt.Before(dayEnd) {
				today++
			}
			if publishedAt.After(next.Add(-time.Hour)) && !publishedAt.After(next) {
				hourly = append(hourly, publishedAt)
			}
		}
		if today >= settings.MaxPostsPerDay {
			next = dayEnd
			continue
		}
		if len(hourly) >= settings.MaxPostsPerHour {
			next = hourly[len(hourly)-settings.MaxPostsPerHour].Add(time.Hour)
			continue
		}
		return next, nil
	}
	return time.Time{}, ErrInvalid
}
