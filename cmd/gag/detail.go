package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/github"
	"github.com/RursusAeternum/GitAGarden/internal/live"
)

// detailFor is what r's detail card shows, as of now.
func detailFor(r *github.Repo, now time.Time) live.Detail {
	d := live.Detail{FullName: r.NameWithOwner, Language: r.Language}
	var times []time.Time
	for _, c := range r.Commits {
		times = append(times, c.At)
		if c.At.After(d.LastCommit.At) {
			d.LastCommit = live.Entry{Title: c.Message, At: c.At}
		}
	}
	d.Daily = live.DailyCommits(times, now)
	for _, pr := range r.OpenPRs {
		if pr.Draft {
			d.Drafts++
			continue
		}
		d.PRs = append(d.PRs, live.Entry{Number: pr.Number, Title: pr.Title, At: pr.CreatedAt})
	}
	sort.SliceStable(d.PRs, func(i, j int) bool { return d.PRs[i].At.Before(d.PRs[j].At) })
	for _, is := range r.Issues {
		if is.ClosedAt != nil {
			continue
		}
		d.OpenIssues++
		if is.CreatedAt.After(d.NewestIssue.At) {
			d.NewestIssue = live.Entry{Number: is.Number, Title: is.Title, At: is.CreatedAt}
		}
	}
	for _, rel := range r.Releases {
		if rel.CreatedAt.After(d.Release.At) {
			d.Release = live.Entry{Title: rel.Tag, At: rel.CreatedAt}
		}
	}
	return d
}

// demoIssues are titles for the demo garden's open issues, picked by number.
var demoIssues = []string{
	"Crash on an empty repo", "Docs: installing on a Pi", "Colours look off in tmux",
	"Support GitLab", "Slow first sync", "Typo in the README",
}

// demoDetail makes up a demo repo's card details from its fake history.
func demoDetail(r demoRepo, events []garden.Event, now time.Time) live.Detail {
	d := live.Detail{FullName: "demo/" + r.name, Language: r.lang, Drafts: r.drafts}
	var times []time.Time
	var open []live.Entry
	for _, e := range events {
		var n int
		switch e.Kind {
		case garden.Push, garden.Merge:
			times = append(times, e.At)
			d.LastCommit = live.Entry{Title: r.commit, At: e.At}
		case garden.Release:
			d.Release = live.Entry{Title: strings.TrimPrefix(e.Note, "released "), At: e.At}
		case garden.IssueOpened:
			if _, err := fmt.Sscanf(e.Note, "issue #%d opened", &n); err == nil {
				open = append(open, live.Entry{Number: n, Title: demoIssues[n%len(demoIssues)], At: e.At})
			}
		case garden.IssueClosed:
			if _, err := fmt.Sscanf(e.Note, "issue #%d closed", &n); err == nil {
				open = slices.DeleteFunc(open, func(is live.Entry) bool { return is.Number == n })
			}
		}
	}
	d.Daily = live.DailyCommits(times, now)
	d.OpenIssues = len(open)
	if len(open) > 0 {
		d.NewestIssue = open[len(open)-1]
	}
	for _, pr := range r.prs {
		d.PRs = append(d.PRs, live.Entry{Number: pr.number, Title: pr.title, At: now.Add(-time.Duration(pr.days) * 24 * time.Hour)})
	}
	sort.SliceStable(d.PRs, func(i, j int) bool { return d.PRs[i].At.Before(d.PRs[j].At) })
	return d
}
