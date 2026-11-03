package reporting

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func summaryFixture(entries []reportEntry) reportData {
	start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	for index := range entries {
		entries[index].SourceType = "MANUAL"
		entries[index].StartedAt = start
		entries[index].EndedAt = start.Add(time.Hour)
	}
	return reportData{Request: reportRequest{Start: start, RangeEnd: start.Add(time.Hour), Location: time.UTC, GroupBy: []string{"ticket", "tag"}}, Entries: entries}
}

func TestSummaryBoundsDistinctGroupsWithoutTruncation(t *testing.T) {
	entries := make([]reportEntry, maximumSummaryGroups+1)
	for index := range entries {
		entries[index].TicketIDs = []uuid.UUID{uuid.New()}
		entries[index].TicketReferences = []string{fmt.Sprintf("APP-%d", index)}
	}
	data := summaryFixture(entries[:maximumSummaryGroups])
	rows, err := summarize(data)
	if err != nil || len(rows) != maximumSummaryGroups {
		t.Fatalf("boundary groups=%d err=%v", len(rows), err)
	}
	data = summaryFixture(entries)
	rows, err = summarize(data)
	if !errors.Is(err, errReportComplexity) || rows != nil {
		t.Fatalf("oversized summary must fail, not truncate: groups=%d err=%v", len(rows), err)
	}
}

func TestSummaryBoundsRepeatedCartesianWork(t *testing.T) {
	entry := reportEntry{}
	for index := range 50 {
		entry.TicketIDs = append(entry.TicketIDs, uuid.New())
		entry.TicketReferences = append(entry.TicketReferences, fmt.Sprintf("APP-%d", index))
		entry.TagIDs = append(entry.TagIDs, uuid.New())
		entry.TagNames = append(entry.TagNames, fmt.Sprintf("Tag-%d", index))
	}
	entries := make([]reportEntry, maximumSummaryContributions/(50*50)+1)
	for index := range entries {
		entries[index] = entry
	}
	rows, err := summarize(summaryFixture(entries))
	if !errors.Is(err, errReportComplexity) || rows != nil {
		t.Fatalf("excessive contributions must fail: groups=%d err=%v", len(rows), err)
	}
}

func TestSummaryKeepsMultiTicketTagTotals(t *testing.T) {
	entry := reportEntry{TicketIDs: []uuid.UUID{uuid.New(), uuid.New()}, TicketReferences: []string{"APP-1", "APP-2"}, TagIDs: []uuid.UUID{uuid.New(), uuid.New()}, TagNames: []string{"Billable", "Backend"}}
	rows, err := summarize(summaryFixture([]reportEntry{entry}))
	if err != nil || len(rows) != 4 {
		t.Fatalf("groups=%d err=%v", len(rows), err)
	}
	for _, row := range rows {
		if row.DurationSeconds != 3600 {
			t.Fatalf("duration=%d", row.DurationSeconds)
		}
	}
}
