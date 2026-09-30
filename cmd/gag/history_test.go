package main

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/config"
	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/github"
	"github.com/RursusAeternum/GitAGarden/internal/live"
)

// bigRepo is a repo with 3000 commits, one every 17 hours up to now, as a
// full fetch has it and as a recent one does.
func bigRepo(now time.Time) (full, recent *github.Repo) {
	full = &github.Repo{NameWithOwner: "me/big", Language: "Go"}
	recent = &github.Repo{NameWithOwner: "me/big", Language: "Go", History: "recent", Totals: &github.Totals{Commits: 3000}}
	for i := 0; i < 3000; i++ {
		c := github.Commit{At: now.Add(-time.Duration(i) * 17 * time.Hour), Message: "work"}
		full.Commits = append(full.Commits, c)
		if now.Sub(c.At) <= github.RecentWindow {
			recent.Commits = append(recent.Commits, c)
		}
	}
	return full, recent
}

func leaves(p *garden.Plant) int {
	n := 0
	for _, row := range p.Grid {
		for _, c := range row {
			if c.Kind == garden.Leaf {
				n++
			}
		}
	}
	return n
}

func TestRecentHistoryGrowsTheSamePlant(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	full, recent := bigRepo(now)
	a, b := repoFor(full, now, now).Plant, repoFor(recent, now, now).Plant
	if a.Grid != b.Grid || a.Pushes != 3000 || b.Pushes != 3000 || !a.LastTended.Equal(b.LastTended) {
		t.Errorf("full and recent history grew different plants: %d and %d pushes", a.Pushes, b.Pushes)
	}
}

func TestSimulateThinsTheCanopy(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	full, _ := bigRepo(now)
	if busy, quiet := leaves(repoFor(full, now, now).Plant), leaves(repoFor(full, now, now.Add(200*24*time.Hour)).Plant); quiet >= busy {
		t.Errorf("200 days untouched: %d leaves, want fewer than today's %d", quiet, busy)
	}
}

func TestHistoryComesFromTheFile(t *testing.T) {
	file := config.Defaults()
	if got := merge(file, nil, settings{}); got.history != github.RecentHistory {
		t.Errorf("default history = %v, want recent", got.history)
	}
	file.History = "full"
	if got := merge(file, nil, settings{}); got.history != github.FullHistory {
		t.Errorf("history = full in the file gave %v", got.history)
	}
}

func TestReplayFetchesTheWholeHistory(t *testing.T) {
	if replayHistory != github.FullHistory {
		t.Error("replay should fetch a repo's whole life, whatever the history setting")
	}
}

func TestCardCountsOpenIssuesFromTotals(t *testing.T) {
	now := time.Now()
	r := &github.Repo{NameWithOwner: "me/busy", Totals: &github.Totals{OpenIssues: 120},
		Issues: []github.Issue{{Number: 500, Title: "newest", CreatedAt: now.Add(-time.Hour)}}}
	if d := detailFor(r, now); d.OpenIssues != 120 || d.NewestIssue.Number != 500 {
		t.Errorf("open issues %d, newest #%d; want GitHub's 120 and #500", d.OpenIssues, d.NewestIssue.Number)
	}
}

func TestTheModesAgreeOnWhatJustHappened(t *testing.T) {
	now := time.Now()
	closed := now.Add(-200 * 24 * time.Hour)
	full := &github.Repo{NameWithOwner: "me/x", Totals: &github.Totals{Commits: 1, Merged: 1},
		Commits: []github.Commit{{At: now.Add(-time.Hour), Message: "work"}},
		Issues:  []github.Issue{{Number: 3, Title: "fixed long ago", CreatedAt: now.Add(-300 * 24 * time.Hour), ClosedAt: &closed}},
		PRs:     []github.PR{{Number: 5, Title: "merged long ago", MergedAt: now.Add(-250 * 24 * time.Hour)}}}
	recent := *full // what a recent fetch of the same repo holds
	recent.History, recent.Issues, recent.PRs = "recent", nil, nil
	if a, b := detailFor(full, now), detailFor(&recent, now); a.LastClosed != b.LastClosed || a.LastMerge != b.LastMerge {
		t.Errorf("full: closed %+v, merged %+v; recent: closed %+v, merged %+v", a.LastClosed, a.LastMerge, b.LastClosed, b.LastMerge)
	}
	if got := live.ChangesBetween([]live.Repo{repoFor(&recent, now, now)}, []live.Repo{repoFor(full, now, now)}); len(got) != 0 {
		t.Errorf("the cache changing shape, say after a replay, played %+v", got)
	}
}
