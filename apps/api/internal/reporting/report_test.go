package reporting

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIntervalDurationClipsToReportRange(t *testing.T) {
	reportStart := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	reportEnd := reportStart.Add(24 * time.Hour)
	startedAt := reportStart.Add(-time.Hour)
	endedAt := reportStart.Add(90 * time.Minute)

	if got, want := intervalDuration(startedAt, endedAt, reportStart, reportEnd), int64(90*60); got != want {
		t.Fatalf("intervalDuration() = %d, want %d", got, want)
	}
}

func TestActiveDurationExcludesPausesAndClipsToReportRange(t *testing.T) {
	reportStart := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	reportEnd := reportStart.Add(24 * time.Hour)
	events := []timerEvent{
		{Type: "START", At: reportStart.Add(-30 * time.Minute)},
		{Type: "PAUSE", At: reportStart.Add(30 * time.Minute)},
		{Type: "RESUME", At: reportStart.Add(60 * time.Minute)},
		{Type: "STOP", At: reportStart.Add(2 * time.Hour)},
	}

	if got, want := activeDuration(events, reportStart, reportEnd), int64(90*60); got != want {
		t.Fatalf("activeDuration() = %d, want %d", got, want)
	}
}

func TestSplitAtMidnightUsesReportTimezone(t *testing.T) {
	location, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 1, 23, 30, 0, 0, location)
	end := time.Date(2026, 9, 2, 1, 0, 0, 0, location)
	segments := splitAtMidnight(interval{Start: start, End: end}, location)
	if len(segments) != 2 {
		t.Fatalf("splitAtMidnight() returned %d segments, want 2", len(segments))
	}
	if got := segments[0].End.In(location).Format("2006-01-02 15:04"); got != "2026-09-02 00:00" {
		t.Fatalf("first segment ends at %s", got)
	}
}

func TestDateBucketWeekStartsMonday(t *testing.T) {
	date := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	key, _ := dateBucket(date, "week")
	if key != "2026-09-14" {
		t.Fatalf("dateBucket() key = %s, want Monday 2026-09-14", key)
	}
}

func TestTagGroupingDuplicatesDurationAcrossTags(t *testing.T) {
	entry := reportEntry{TagIDs: []uuid.UUID{uuid.New(), uuid.New()}, TagNames: []string{"Billable", "Backend"}}
	groups := groupCombinations(entry, time.Now(), []string{"tag"}, "day")
	if len(groups) != 2 {
		t.Fatalf("groupCombinations() returned %d groups, want 2", len(groups))
	}
}

func TestTicketGroupingIncludesEveryLinkedTicket(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	entry := reportEntry{TicketIDs: []uuid.UUID{first, second}, TicketReferences: []string{"APP-1", "APP-2"}}
	groups := groupCombinations(entry, time.Now(), []string{"ticket"}, "day")
	if len(groups) != 2 || groups[0][0].Key != first.String() || groups[1][0].Key != second.String() {
		t.Fatalf("ticket groups = %+v", groups)
	}
	entry.TicketIDs = nil
	if got := groupCombinations(entry, time.Now(), []string{"ticket"}, "day"); len(got) != 1 || got[0][0].Label != "No ticket" {
		t.Fatalf("unlinked ticket group = %+v", got)
	}
}
