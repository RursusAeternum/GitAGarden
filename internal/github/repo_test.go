package github

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
)

func TestEventsAreTimeOrdered(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	at := func(h int) time.Time { return t0.Add(time.Duration(h) * time.Hour) }
	closed := at(5)
	r := &Repo{
		// Commits arrive newest first from the API.
		Commits:  []Commit{{At: at(6), Message: "c2"}, {At: at(1), Message: "c1"}},
		PRs:      []PR{{Number: 1, MergedAt: at(3)}},
		Issues:   []Issue{{Number: 2, CreatedAt: at(2), ClosedAt: &closed}},
		Releases: []Release{{Tag: "v1", CreatedAt: at(4)}},
	}
	want := []garden.EventKind{garden.Push, garden.IssueOpened, garden.Merge, garden.Release, garden.IssueClosed, garden.Push}
	got := r.Events()
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d", len(got), len(want))
	}
	for i, e := range got {
		if e.Kind != want[i] {
			t.Errorf("event %d: got %s, want %s", i, e.Kind, want[i])
		}
	}
}

func TestFinished(t *testing.T) {
	cases := map[string]*Repo{
		"archived": {Archived: true},
		"topic":    {Topics: []string{"go", "finished"}},
		"gag":      {Topics: []string{"gag-finished"}},
	}
	for name, r := range cases {
		if !r.Finished() {
			t.Errorf("%s: want finished", name)
		}
	}
	if (&Repo{Topics: []string{"wip"}}).Finished() {
		t.Error("active repo reported finished")
	}
}
