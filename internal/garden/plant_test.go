package garden

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestGrowIsDeterministic(t *testing.T) {
	events := FakeHistory("repo", 200, t0)
	for _, sp := range AllSpecies {
		a, b := Grow("repo", sp, events), Grow("repo", sp, events)
		if a.Grid != b.Grid {
			t.Errorf("%s: same inputs produced different plants", sp)
		}
	}
}

func TestDifferentNamesGrowDifferently(t *testing.T) {
	events := FakeHistory("repo", 80, t0)
	if Grow("alpha", Shrub, events).Grid == Grow("beta", Shrub, events).Grid {
		t.Error("different repo names produced identical shrubs")
	}
}

func TestEveryEarlyPushAddsSomething(t *testing.T) {
	for _, sp := range AllSpecies {
		var events []Event
		prev := Grow("repo", sp, nil)
		for i := 0; i < 30; i++ {
			events = append(events, Event{Kind: Push, At: t0.Add(time.Duration(i) * time.Hour)})
			next := Grow("repo", sp, events)
			if next.Grid == prev.Grid {
				t.Fatalf("%s: push %d left the plant unchanged", sp, i+1)
			}
			prev = next
		}
	}
}

func TestLongHistoriesDoNotPanic(t *testing.T) {
	for _, sp := range AllSpecies {
		for _, name := range []string{"a", "b", "c", "d", "e"} {
			p := Grow(name, sp, FakeHistory(name, 2000, t0))
			_ = Render(p, RenderOpts{Now: t0.Add(24 * time.Hour * 400)})
			_ = Card(p, RenderOpts{Now: t0, Finished: true})
		}
	}
}

func TestHealthDecays(t *testing.T) {
	p := Grow("repo", Shrub, []Event{{Kind: Push, At: t0}})
	if h := Health(p, t0.Add(24*time.Hour), 45); h != 1 {
		t.Errorf("health a day after tending = %v, want 1", h)
	}
	if h := Health(p, t0.Add(24*time.Hour*20), 45); h <= 0 || h >= 1 {
		t.Errorf("health after 20 idle days = %v, want between 0 and 1", h)
	}
	if h := Health(p, t0.Add(24*time.Hour*100), 45); h != 0 {
		t.Errorf("health after 100 idle days = %v, want 0", h)
	}
}

func TestIssuesDoNotCountAsTending(t *testing.T) {
	p := Grow("repo", Shrub, []Event{
		{Kind: Push, At: t0},
		{Kind: IssueOpened, At: t0.Add(48 * time.Hour)},
	})
	if !p.LastTended.Equal(t0) || p.OpenIssues != 1 {
		t.Errorf("LastTended=%v OpenIssues=%d", p.LastTended, p.OpenIssues)
	}
}
