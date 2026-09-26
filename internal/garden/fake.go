package garden

import (
	"fmt"
	"math/rand"
	"time"
)

// FakeHistory generates a plausible, deterministic event history for a repo:
// bursts of work, quiet spells, the odd merge, release and issue. It stands in
// for the GitHub API while the renderer is being tuned.
func FakeHistory(name string, n int, start time.Time) []Event {
	r := rand.New(rand.NewSource(seedOf("history:" + name)))
	events := make([]Event, 0, n)
	at := start
	pr, issue, open := 1, 1, 0
	major, minor := 0, 1

	for len(events) < n {
		// Mostly short gaps between pushes, occasionally a lull of days or weeks.
		switch f := r.Float64(); {
		case f < 0.70:
			at = at.Add(time.Duration(1+r.Intn(8)) * time.Hour)
		case f < 0.93:
			at = at.Add(time.Duration(1+r.Intn(3)) * 24 * time.Hour)
		default:
			at = at.Add(time.Duration(6+r.Intn(20)) * 24 * time.Hour)
		}

		var e Event
		switch f := r.Float64(); {
		case f < 0.12:
			e = Event{Kind: Merge, Note: fmt.Sprintf("merged PR #%d", pr)}
			pr++
		case f < 0.15:
			e = Event{Kind: Release, Note: fmt.Sprintf("released v%d.%d.0", major, minor)}
			if minor++; minor > 9 {
				major, minor = major+1, 0
			}
		case f < 0.24:
			e = Event{Kind: IssueOpened, Note: fmt.Sprintf("issue #%d opened", issue)}
			issue++
			open++
		case f < 0.30 && open > 0:
			e = Event{Kind: IssueClosed, Note: fmt.Sprintf("issue #%d closed", issue-open)}
			open--
		default:
			e = Event{Kind: Push, Note: "pushed to main"}
		}
		e.At = at
		events = append(events, e)
	}
	return events
}
