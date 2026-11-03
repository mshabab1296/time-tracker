package timetracking

import (
	"testing"
	"time"
)

func TestDurationStopWhilePausedExcludesPausedTime(t *testing.T) {
	start := time.Date(2026, time.September, 29, 9, 0, 0, 0, time.UTC)
	now := start.Add(time.Hour)
	paused := []timerEvent{
		{Type: "START", At: start},
		{Type: "PAUSE", At: start.Add(20 * time.Minute)},
	}
	if seconds, valid := duration(paused, now); !valid || seconds != 1200 {
		t.Fatalf("paused timer duration=%d valid=%v, want 1200", seconds, valid)
	}
	stopped := append(append([]timerEvent{}, paused...), timerEvent{Type: "STOP", At: now})
	if seconds, valid := duration(stopped, now); !valid || seconds != 1200 {
		t.Fatalf("stopped timer duration=%d valid=%v, want 1200", seconds, valid)
	}
	invalid := append(stopped, timerEvent{Type: "STOP", At: now})
	if _, valid := duration(invalid, now); valid {
		t.Fatal("a second STOP event must be rejected")
	}
}
