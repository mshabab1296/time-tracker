package timetracking

import (
	"testing"
	"time"
)

func TestValidateCompletedTimerEvents(t *testing.T) {
	now := time.Date(2026, time.September, 14, 12, 0, 0, 0, time.UTC)
	start := now.Add(-2 * time.Hour)
	tests := []struct {
		name     string
		events   []timerEvent
		valid    bool
		duration int64
	}{
		{"start and stop", []timerEvent{{Type: "START", At: start}, {Type: "STOP", At: start.Add(time.Hour)}}, true, 3600},
		{"pause pair", []timerEvent{{Type: "START", At: start}, {Type: "PAUSE", At: start.Add(20 * time.Minute)}, {Type: "RESUME", At: start.Add(40 * time.Minute)}, {Type: "STOP", At: start.Add(time.Hour)}}, true, 2400},
		{"missing resume", []timerEvent{{Type: "START", At: start}, {Type: "PAUSE", At: start.Add(20 * time.Minute)}, {Type: "STOP", At: start.Add(time.Hour)}}, false, 0},
		{"wrong pair order", []timerEvent{{Type: "START", At: start}, {Type: "RESUME", At: start.Add(20 * time.Minute)}, {Type: "PAUSE", At: start.Add(40 * time.Minute)}, {Type: "STOP", At: start.Add(time.Hour)}}, false, 0},
		{"non chronological", []timerEvent{{Type: "START", At: start}, {Type: "STOP", At: start}}, false, 0},
		{"future", []timerEvent{{Type: "START", At: start}, {Type: "STOP", At: now.Add(time.Minute)}}, false, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			duration, valid := validateCompletedTimerEvents(test.events, now)
			if valid != test.valid || duration != test.duration {
				t.Fatalf("valid=%v duration=%d, want valid=%v duration=%d", valid, duration, test.valid, test.duration)
			}
		})
	}
}
