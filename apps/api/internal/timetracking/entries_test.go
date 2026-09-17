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
