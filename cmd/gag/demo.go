package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/live"
)

// The demo garden: fake repos with made-up histories and details, for
// -demo, the no-token fallback and the browser demo. Nothing here reaches
// GitHub, which keeps net/http out of the browser build.

// demoItem is an open PR in the demo garden.
type demoItem struct {
	days   int // how long ago it was opened
	number int
	title  string
}

type demoRepo struct {
	name, lang string
	events     int
	idleDays   int
	finished   bool
	ci         live.CI
	prs        []demoItem // open PRs
	drafts     int
	newIssues  int
	rising     bool
	stars      int
	commit     string // the last commit's headline
	by         string // who made the last push: an AI agent's name, or "" for a person
}

var demo = []demoRepo{
	{name: "gag-core", lang: "Go", events: 160, ci: live.CIPassing, rising: true, stars: 31,
		prs:    []demoItem{{2, 41, "Add a detail card"}, {10, 38, "Bump bubbletea to v1.3"}},
		commit: "Draw the card beside the plant"},
	{name: "rustyfs", lang: "Rust", events: 90, idleDays: 12, ci: live.CIFailing, stars: 12,
		commit: "Fix inode refcount on unlink"},
	{name: "notebook-api", lang: "Python", events: 70, idleDays: 3, ci: live.CIPending, newIssues: 4, stars: 3,
		commit: "Paginate the notes endpoint"},
	{name: "old-blog", lang: "Go", events: 120, idleDays: 400, finished: true,
		commit: "Final post: moving on"},
	{name: "dotfiles", lang: "Shell", events: 40, idleDays: 35, commit: "Add fish abbreviations"},
	{name: "tiny-cli", lang: "Rust", events: 12, idleDays: 1, drafts: 1,
		prs: []demoItem{{1, 7, "Support --json output"}}, commit: "Handle empty input"},
	{name: "site-v2", lang: "TypeScript", events: 110, idleDays: 70, commit: "Swap the hero image"},
	{name: "lsystem", lang: "C", events: 60, idleDays: 200, finished: true, stars: 2,
		commit: "Add a Koch curve example"},
}

var (
	// demoIssues are titles for the demo garden's issues, picked by number.
	demoIssues = []string{
		"Crash on an empty repo", "Docs: installing on a Pi", "Colours look off in tmux",
		"Support GitLab", "Slow first sync", "Typo in the README",
	}
	// demoPRTitles are titles for the demo garden's merged PRs, by number.
	demoPRTitles = []string{
		"Add a detail card", "Bump bubbletea to v1.3", "Support --json output",
		"Refactor the pan", "Water the plants on time", "Make storms readable",
	}
)

// demoGarden grows the demo repos, with fake histories whose last event
// lands idleDays before now and a few signals so every one shows.
func demoGarden(now time.Time) []live.Repo {
	var out []live.Repo
	for _, r := range demo {
		events := garden.FakeHistory(r.name, r.events, now)
		shift := now.Sub(events[len(events)-1].At) - time.Duration(r.idleDays)*24*time.Hour
		for i := range events {
			events[i].At = events[i].At.Add(shift)
		}
		lr := live.Repo{Name: r.name, Plant: garden.Grow(r.name, garden.SpeciesFor(r.lang), events),
			Finished: r.finished, Branch: "main", CI: r.ci, NewIssues: r.newIssues, Rising: r.rising, Stars: r.stars,
			Detail: demoDetail(r, events, now)}
		for _, pr := range r.prs {
			lr.PRs = append(lr.PRs, now.Add(-time.Duration(pr.days)*24*time.Hour))
		}
		out = append(out, lr)
	}
	return out
}

// demoDetail makes up a demo repo's card details from its fake history.
func demoDetail(r demoRepo, events []garden.Event, now time.Time) live.Detail {
	d := live.Detail{FullName: "demo/" + r.name, Language: r.lang, Drafts: r.drafts}
	var times []time.Time
	var open []live.Entry
	for _, e := range events {
		var n int
		switch e.Kind {
		case garden.Push:
			times = append(times, e.At)
			d.LastCommit = live.Entry{Title: r.commit, At: e.At, By: r.by}
		case garden.Merge:
			times = append(times, e.At)
			d.LastCommit = live.Entry{Title: r.commit, At: e.At}
			if _, err := fmt.Sscanf(e.Note, "merged PR #%d", &n); err == nil {
				d.LastMerge = live.Entry{Number: n, Title: demoPRTitles[n%len(demoPRTitles)], At: e.At}
			}
		case garden.Release:
			d.Release = live.Entry{Title: strings.TrimPrefix(e.Note, "released "), At: e.At}
		case garden.IssueOpened:
			if _, err := fmt.Sscanf(e.Note, "issue #%d opened", &n); err == nil {
				open = append(open, live.Entry{Number: n, Title: demoIssues[n%len(demoIssues)], At: e.At})
			}
		case garden.IssueClosed:
			if _, err := fmt.Sscanf(e.Note, "issue #%d closed", &n); err == nil {
				open = slices.DeleteFunc(open, func(is live.Entry) bool { return is.Number == n })
				d.LastClosed = live.Entry{Number: n, Title: demoIssues[n%len(demoIssues)], At: e.At}
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
