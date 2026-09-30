package main

import (
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"strings"
	"sync"
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

// demoRefresh is how often the live demo loads, and so acts out a change.
const demoRefresh = 30 * time.Second

// demoHeadlines are the demo's scripted commit headlines.
var demoHeadlines = []string{
	"Tidy the pot colours", "Handle empty input", "Speed up the first sync", "Fix a typo in the help",
	"Draw the card beside the plant", "Cache the listing", "Retry on a flaky network", "Name the snail",
}

// demoWorld is the demo garden as a small world that changes: every
// snapshot after the first applies one scripted change, cycling through
// push, merge, release, new issue, closed issue, CI red and CI green.
type demoWorld struct {
	mu     sync.Mutex
	repos  []demoState
	loads  int
	next   int // the next step in the cycle
	pushes int
	rng    *rand.Rand
}

// demoState is one demo repo as the world has changed it.
type demoState struct {
	demoRepo
	events []garden.Event
}

// demoSteps is the cycle the demo acts out. A step reports false when no
// plant suits it, and the world moves on to the next.
var demoSteps = []func(*demoWorld, time.Time) bool{
	(*demoWorld).push, (*demoWorld).merge, (*demoWorld).release, (*demoWorld).newIssue,
	(*demoWorld).closeIssue, (*demoWorld).ciRed, (*demoWorld).ciGreen,
}

// newDemoWorld is the demo garden as it starts: fake histories whose last
// event lands idleDays before now.
func newDemoWorld(now time.Time) *demoWorld {
	w := &demoWorld{rng: rand.New(rand.NewSource(7))}
	for _, r := range demo {
		events := garden.FakeHistory(r.name, r.events, now)
		shift := now.Sub(events[len(events)-1].At) - time.Duration(r.idleDays)*24*time.Hour
		for i := range events {
			events[i].At = events[i].At.Add(shift)
		}
		w.repos = append(w.repos, demoState{demoRepo: r, events: events})
	}
	return w
}

// demoGarden grows the demo repos as they start out.
func demoGarden(now time.Time) []live.Repo { return newDemoWorld(now).garden(now) }

// garden grows the world's repos, with their signals and details, as of now.
func (w *demoWorld) garden(now time.Time) []live.Repo {
	var out []live.Repo
	for _, s := range w.repos {
		r := s.demoRepo
		lr := live.Repo{Name: r.name, Plant: garden.GrowAt(r.name, garden.SpeciesFor(r.lang), garden.Totals{}, s.events, now),
			Finished: r.finished, Branch: "main", CI: r.ci, NewIssues: r.newIssues, Rising: r.rising, Stars: r.stars,
			Detail: demoDetail(r, s.events, now)}
		for _, pr := range r.prs {
			lr.PRs = append(lr.PRs, now.Add(-time.Duration(pr.days)*24*time.Hour))
		}
		out = append(out, lr)
	}
	return out
}

// snapshot is the world for the live view: the first as it starts, each
// later one a scripted change further on.
func (w *demoWorld) snapshot(now time.Time) live.Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.loads > 0 {
		for tries := 0; tries < len(demoSteps); tries++ {
			step := demoSteps[w.next%len(demoSteps)]
			w.next++
			if step(w, now) {
				break
			}
		}
	}
	w.loads++
	return live.Snapshot{Repos: w.garden(now), FetchedAt: now, Note: "demo garden", Demo: true}
}

// pick is a random unfinished demo repo that ok accepts, or nil.
func (w *demoWorld) pick(ok func(*demoState) bool) *demoState {
	var fits []*demoState
	for i := range w.repos {
		if s := &w.repos[i]; !s.finished && ok(s) {
			fits = append(fits, s)
		}
	}
	if len(fits) == 0 {
		return nil
	}
	return fits[w.rng.Intn(len(fits))]
}

func anyRepo(*demoState) bool { return true }

// countKind is how many events of kind k a history has.
func countKind(events []garden.Event, k garden.EventKind) int {
	n := 0
	for _, e := range events {
		if e.Kind == k {
			n++
		}
	}
	return n
}

// openIssues are the numbers of a history's open issues, oldest first.
func openIssues(events []garden.Event) []int {
	var open []int
	for _, e := range events {
		var n int
		switch e.Kind {
		case garden.IssueOpened:
			if _, err := fmt.Sscanf(e.Note, "issue #%d opened", &n); err == nil {
				open = append(open, n)
			}
		case garden.IssueClosed:
			if _, err := fmt.Sscanf(e.Note, "issue #%d closed", &n); err == nil {
				open = slices.DeleteFunc(open, func(o int) bool { return o == n })
			}
		}
	}
	return open
}

// push commits to a repo; every other push is by Claude, so the drone shows.
func (w *demoWorld) push(now time.Time) bool {
	s := w.pick(anyRepo)
	if s == nil {
		return false
	}
	w.pushes++
	s.commit, s.by = demoHeadlines[w.pushes%len(demoHeadlines)], ""
	if w.pushes%2 == 0 {
		s.by = "Claude"
	}
	s.events = append(s.events, garden.Event{Kind: garden.Push, At: now, Note: "pushed to main"})
	return true
}

// merge merges the repo's next PR.
func (w *demoWorld) merge(now time.Time) bool {
	s := w.pick(anyRepo)
	if s == nil {
		return false
	}
	n := countKind(s.events, garden.Merge) + 1
	s.events = append(s.events, garden.Event{Kind: garden.Merge, At: now, Note: fmt.Sprintf("merged PR #%d", n)})
	return true
}

// release tags the repo's next version, numbered as FakeHistory numbers them.
func (w *demoWorld) release(now time.Time) bool {
	s := w.pick(anyRepo)
	if s == nil {
		return false
	}
	v := countKind(s.events, garden.Release) + 1
	s.events = append(s.events, garden.Event{Kind: garden.Release, At: now, Note: fmt.Sprintf("released v%d.%d.0", v/10, v%10)})
	return true
}

// newIssue opens the repo's next issue.
func (w *demoWorld) newIssue(now time.Time) bool {
	s := w.pick(anyRepo)
	if s == nil {
		return false
	}
	n := countKind(s.events, garden.IssueOpened) + 1
	s.events = append(s.events, garden.Event{Kind: garden.IssueOpened, At: now, Note: fmt.Sprintf("issue #%d opened", n)})
	return true
}

// closeIssue closes the oldest open issue of a repo that has one.
func (w *demoWorld) closeIssue(now time.Time) bool {
	s := w.pick(func(s *demoState) bool { return len(openIssues(s.events)) > 0 })
	if s == nil {
		return false
	}
	n := openIssues(s.events)[0]
	s.events = append(s.events, garden.Event{Kind: garden.IssueClosed, At: now, Note: fmt.Sprintf("issue #%d closed", n)})
	return true
}

// ciRed breaks the build of a repo whose CI isn't failing.
func (w *demoWorld) ciRed(time.Time) bool {
	s := w.pick(func(s *demoState) bool { return s.ci != live.CIFailing })
	if s == nil {
		return false
	}
	s.ci = live.CIFailing
	return true
}

// ciGreen fixes the build of a repo whose CI is failing.
func (w *demoWorld) ciGreen(time.Time) bool {
	s := w.pick(func(s *demoState) bool { return s.ci == live.CIFailing })
	if s == nil {
		return false
	}
	s.ci = live.CIPassing
	return true
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
