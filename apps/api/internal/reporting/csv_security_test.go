package reporting

import (
	"bytes"
	"encoding/csv"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCSVNeutralizesSpreadsheetFormulas(t *testing.T) {
	for _, value := range []string{"=HYPERLINK(\"https://example.test\")", "+1+1", "-1+1", "@SUM(A1:A2)", "  =1+1", "\ttext", "\rtext", "\ntext"} {
		got := safeCSVRecord([]string{value, "APP-1", "3600"})
		if got[0] != "'"+value || got[1] != "APP-1" || got[2] != "3600" {
			t.Fatalf("unsafe record: %#v", got)
		}
	}
}

func TestDetailedAndSummaryCSVNeutralizeUserText(t *testing.T) {
	id := uuid.New()
	start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	data := reportData{Request: reportRequest{Location: time.UTC, Start: start, RangeEnd: start.Add(time.Hour), GroupBy: []string{"ticket"}}, Entries: []reportEntry{{ID: id, UserName: "=1+1", Description: "@SUM(A1)", SourceType: "MANUAL", StartedAt: start, EndedAt: start.Add(time.Hour), TicketIDs: []uuid.UUID{uuid.New()}, TicketReferences: []string{"+APP"}}}, Duration: map[uuid.UUID]int64{id: 3600}, Events: map[uuid.UUID][]timerEvent{}}
	for _, summary := range []bool{false, true} {
		var buffer bytes.Buffer
		writer := csv.NewWriter(&buffer)
		if summary {
			rows, err := summarize(data)
			if err != nil {
				t.Fatal(err)
			}
			writeSummaryCSV(writer, data, rows)
		} else {
			writeDetailedCSV(writer, data)
		}
		writer.Flush()
		records, err := csv.NewReader(&buffer).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		column, want := 0, "'=1+1"
		if summary {
			want = "'+APP"
		}
		if records[1][column] != want {
			t.Fatalf("CSV=%#v", records)
		}
	}
}
