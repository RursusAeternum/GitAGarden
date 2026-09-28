package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/github"
	"github.com/RursusAeternum/GitAGarden/internal/live"
)

func TestRepoForFillsTheDetail(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	ago := func(h int) time.Time { return now.Add(-time.Duration(h) * time.Hour) }
	closed := ago(1)
	r := &github.Repo{NameWithOwner: "me/gag", Language: "Go",
		Commits: []github.Commit{{At: ago(50), Message: "older"}, {At: ago(2), Message: "newest", Agent: "Claude"}, {At: ago(480), Message: "ancient"}},
		PRs:     []github.PR{{Number: 40, Title: "Add sparkline data", MergedAt: ago(5)}, {Number: 38, Title: "Older", MergedAt: ago(100)}},
		OpenPRs: []github.OpenPR{
			{Number: 11, Title: "Add sparkline", CreatedAt: ago(48)},
			{Number: 12, Title: "WIP", CreatedAt: ago(1), Draft: true},
			{Number: 9, Title: "Bump deps", CreatedAt: ago(240)},
		},
		Issues: []github.Issue{
			{Number: 30, Title: "Old bug", CreatedAt: ago(216)},
			{Number: 31, Title: "Crash on empty repo", CreatedAt: ago(48)},
			{Number: 32, Title: "Fixed already", CreatedAt: ago(3), ClosedAt: &closed},
		},
		Releases: []github.Release{{Tag: "v0.5.0", CreatedAt: ago(1)}, {Tag: "v0.4.0", CreatedAt: ago(30)}},
	}
	want := live.Detail{FullName: "me/gag", Language: "Go",
		LastCommit:  live.Entry{Title: "newest", At: ago(2), By: "Claude"},
		Daily:       [14]int{11: 1, 13: 1},
		PRs:         []live.Entry{{Number: 9, Title: "Bump deps", At: ago(240)}, {Number: 11, Title: "Add sparkline", At: ago(48)}},
		Drafts:      1,
		OpenIssues:  2,
		NewestIssue: live.Entry{Number: 31, Title: "Crash on empty repo", At: ago(48)},
		LastMerge:   live.Entry{Number: 40, Title: "Add sparkline data", At: ago(5)},
		LastClosed:  live.Entry{Number: 32, Title: "Fixed already", At: ago(1)},
		Release:     live.Entry{Title: "v0.5.0", At: ago(1)},
	}
	if got := repoFor(r, now).Detail; !reflect.DeepEqual(got, want) {
		t.Errorf("detail =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDemoReposHaveDetails(t *testing.T) {
	var prTitles, drafts, releases, issues, merges, closed int
	for _, r := range demoGarden(time.Now()) {
		d := r.Detail
		if d.FullName == "" || d.Language == "" || d.LastCommit.Title == "" || d.LastCommit.At.IsZero() {
			t.Errorf("%s: detail %+v lacks its name, language or last commit", r.Name, d)
		}
		if len(d.PRs) != len(r.PRs) {
			t.Errorf("%s: %d PR titles for %d buds", r.Name, len(d.PRs), len(r.PRs))
		}
		if d.OpenIssues != r.Plant.OpenIssues {
			t.Errorf("%s: the card says %d open issues, the plant has %d weeds", r.Name, d.OpenIssues, r.Plant.OpenIssues)
		}
		prTitles += len(d.PRs)
		drafts += d.Drafts
		if strings.HasPrefix(d.Release.Title, "v") {
			releases++
		}
		if d.NewestIssue.Number > 0 && d.NewestIssue.Title != "" {
			issues++
		}
		if d.LastMerge.Number > 0 && d.LastMerge.Title != "" {
			merges++
		}
		if d.LastClosed.Number > 0 && d.LastClosed.Title != "" {
			closed++
		}
	}
	if prTitles == 0 || drafts == 0 || releases == 0 || issues == 0 || merges == 0 || closed == 0 {
		t.Errorf("the demo should show every kind of detail: %d PR titles, %d drafts, %d releases, %d newest issues, %d merges, %d closed issues",
			prTitles, drafts, releases, issues, merges, closed)
	}
}
