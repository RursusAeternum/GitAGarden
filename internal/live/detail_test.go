package live

import (
	"testing"
	"time"
)

func TestDailyCommitsCountsLocalDays(t *testing.T) {
	loc := time.FixedZone("here", 2*60*60)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, loc)
	times := []time.Time{
		now.Add(-time.Hour),                           // today
		time.Date(2026, 6, 14, 0, 30, 0, 0, loc),      // today, just after midnight
		time.Date(2026, 6, 13, 22, 0, 0, 0, time.UTC), // midnight here: today
		time.Date(2026, 6, 13, 23, 30, 0, 0, loc),     // yesterday
		time.Date(2026, 6, 1, 9, 0, 0, 0, loc),        // 13 days ago: the first day
		time.Date(2026, 5, 31, 9, 0, 0, 0, loc),       // 14 days ago: too old
		now.Add(time.Hour),                            // the future
	}
	want := [14]int{0: 1, 12: 1, 13: 3}
	if got := DailyCommits(times, now); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
