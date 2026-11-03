package timetracking

import (
	"testing"
	"time"
)

func TestClippedDurationUsesOnlySelectedDayOverlap(t *testing.T) {
	dayStart := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	startedAt := dayStart.Add(-30 * time.Minute)
	endedAt := dayStart.Add(90 * time.Minute)

	if got, want := clippedDuration(startedAt, endedAt, dayStart, dayEnd), int64(90*60); got != want {
		t.Fatalf("clippedDuration() = %d, want %d", got, want)
	}
}

func TestClippedRangePreservesOriginalEntryAndUsesLocalMidnight(t *testing.T) {
	location, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	startedAt := time.Date(2026, 9, 29, 16, 2, 12, 0, location)
	endedAt := time.Date(2026, 9, 30, 23, 49, 41, 0, location)
	firstDay := time.Date(2026, 9, 29, 0, 0, 0, 0, location)
	secondDay := firstDay.AddDate(0, 0, 1)
	thirdDay := secondDay.AddDate(0, 0, 1)

	firstStart, firstEnd := clippedRange(startedAt, endedAt, firstDay, secondDay)
	if !firstStart.Equal(startedAt) || !firstEnd.Equal(secondDay) {
		t.Fatalf("first day range = %v–%v, want %v–%v", firstStart, firstEnd, startedAt, secondDay)
	}
	secondStart, secondEnd := clippedRange(startedAt, endedAt, secondDay, thirdDay)
	if !secondStart.Equal(secondDay) || !secondEnd.Equal(endedAt) {
		t.Fatalf("second day range = %v–%v, want %v–%v", secondStart, secondEnd, secondDay, endedAt)
	}
	if got := clippedDuration(startedAt, endedAt, firstDay, secondDay); got != int64((7*time.Hour + 57*time.Minute + 48*time.Second).Seconds()) {
		t.Fatalf("first day duration = %d", got)
	}
	if got := clippedDuration(startedAt, endedAt, secondDay, thirdDay); got != int64((23*time.Hour + 49*time.Minute + 41*time.Second).Seconds()) {
		t.Fatalf("second day duration = %d", got)
	}
}
