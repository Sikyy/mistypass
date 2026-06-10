package httpx

import (
	"sort"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/event"
)

type occupancyDay struct {
	Date         string `json:"date"`
	UniqueUsers  int    `json:"unique_users"`
	TotalEntries int    `json:"total_entries"`
}

type retentionBucket struct {
	Start          string  `json:"start"`
	ActiveUsers    int     `json:"active_users"`
	NewUsers       int     `json:"new_users"`
	ReturningUsers int     `json:"returning_users"`
	RetentionRate  float64 `json:"retention_rate"`
}

// isActiveAccessEvent reports whether an event represents a successful presence
// signal by a known actor — the same "granted" rule used by getAccessSummary.
func isActiveAccessEvent(ev event.AccessEvent) bool {
	if strings.TrimSpace(ev.Actor) == "" {
		return false
	}
	return strings.EqualFold(ev.Result, "success") || strings.EqualFold(ev.Result, "accepted")
}

func eventInWindow(ev event.AccessEvent, start, end time.Time, buildingID string) bool {
	if ev.At.Before(start) || !ev.At.Before(end) {
		return false
	}
	if buildingID != "" && ev.BuildingID != buildingID {
		return false
	}
	return true
}

// weekBucketKey returns the UTC date of the Monday on/before t (RFC date string).
func weekBucketKey(t time.Time) string {
	u := t.UTC()
	offset := (int(u.Weekday()) + 6) % 7 // days since Monday (Mon=0 … Sun=6)
	monday := time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -offset)
	return monday.Format("2006-01-02")
}

// computeOccupancy aggregates daily unique active users (UTC days) in [start,end).
func computeOccupancy(events []event.AccessEvent, start, end time.Time, buildingID string) (days []occupancyDay, peakDate string, peakUnique int, totalUnique int) {
	perDayUsers := map[string]map[string]struct{}{}
	perDayEntries := map[string]int{}
	allUsers := map[string]struct{}{}

	for _, ev := range events {
		if !eventInWindow(ev, start, end, buildingID) || !isActiveAccessEvent(ev) {
			continue
		}
		day := ev.At.UTC().Format("2006-01-02")
		if perDayUsers[day] == nil {
			perDayUsers[day] = map[string]struct{}{}
		}
		perDayUsers[day][ev.Actor] = struct{}{}
		perDayEntries[day]++
		allUsers[ev.Actor] = struct{}{}
	}

	dayKeys := make([]string, 0, len(perDayUsers))
	for day := range perDayUsers {
		dayKeys = append(dayKeys, day)
	}
	sort.Strings(dayKeys)

	days = make([]occupancyDay, 0, len(dayKeys))
	for _, day := range dayKeys {
		unique := len(perDayUsers[day])
		days = append(days, occupancyDay{Date: day, UniqueUsers: unique, TotalEntries: perDayEntries[day]})
		if unique > peakUnique {
			peakUnique = unique
			peakDate = day
		}
	}
	return days, peakDate, peakUnique, len(allUsers)
}

// computeRetention aggregates active/new/returning users per bucket (day|week).
func computeRetention(events []event.AccessEvent, start, end time.Time, buildingID, bucket string) []retentionBucket {
	bucketKey := func(t time.Time) string {
		if strings.EqualFold(bucket, "day") {
			return t.UTC().Format("2006-01-02")
		}
		return weekBucketKey(t)
	}

	bucketUsers := map[string]map[string]struct{}{}
	for _, ev := range events {
		if !eventInWindow(ev, start, end, buildingID) || !isActiveAccessEvent(ev) {
			continue
		}
		key := bucketKey(ev.At)
		if bucketUsers[key] == nil {
			bucketUsers[key] = map[string]struct{}{}
		}
		bucketUsers[key][ev.Actor] = struct{}{}
	}

	keys := make([]string, 0, len(bucketUsers))
	for key := range bucketUsers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	seen := map[string]struct{}{}
	out := make([]retentionBucket, 0, len(keys))
	prevActive := 0
	for i, key := range keys {
		users := bucketUsers[key]
		active := len(users)
		newUsers := 0
		returning := 0
		for actor := range users {
			if _, ok := seen[actor]; ok {
				returning++
			} else {
				newUsers++
			}
		}
		for actor := range users {
			seen[actor] = struct{}{}
		}
		rate := 0.0
		if i > 0 && prevActive > 0 {
			rate = float64(returning) / float64(prevActive)
		}
		out = append(out, retentionBucket{
			Start:          key,
			ActiveUsers:    active,
			NewUsers:       newUsers,
			ReturningUsers: returning,
			RetentionRate:  rate,
		})
		prevActive = active
	}
	return out
}
