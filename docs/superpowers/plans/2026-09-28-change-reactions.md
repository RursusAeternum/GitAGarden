# Change Reactions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the live garden react to real changes between refreshes. A push waters the plant, and a drone does it when an AI agent made the commit. A merge bursts a bud, a release sparkles, new and closed issues grow and pull weeds, and CI turning red or green rolls a storm in or clears it with a rainbow. Every change gets a ticker note, the camera visits changes off screen, and the demo acts all of it out.

**Architecture:**
- **`internal/live`** diffs each online snapshot against the last one (`ChangesBetween`), queues a reaction and a ticker note for every change, and sends the camera on visits.
- **`internal/scene`** draws each reaction as a pure function of its age. A plot keeps its old shape until the reaction's change moment, then its new cells grow in (`garden.Paint` with `Style.Before` and `Grown`).
- **`cmd/gag`** fills the new card data, reads which agent authored a commit (`github.Commit.Agent`), and runs a small scripted demo world.

**Tech Stack:** Go 1.26, Bubble Tea v1.3.10, go-runewidth; GitHub GraphQL `Commit.authors`.

**Spec:** `docs/superpowers/specs/2026-09-28-change-reactions-design.md`, on top of main (v0.6 plus the browser demo, `RursusAeternum/GitAGarden#1`). It ships as v0.7.0.

**Plan refinements of the spec.** Each one keeps the spec's intent:
1. **Squash and merge commits aren't pushes.** A push counts only when the plant gained more commits than merges. Squash and merge commits would otherwise water the plant for every merge. The count in the note is the difference.
2. **A closed issue's plant changes at the start, not the end.** The pulled weed leaves the soil straight away and is drawn lifting off, which is the same picture.
3. **Changes on screen play first.** Changed plants on screen play straight away. The camera's visits to off-screen plants begin once those reactions have finished, so nothing on screen goes unseen.
4. **Newest notes first.** Notes show newest first, and a new note restarts the ticker's rotation.
5. **Note text is cleaned.** Control characters and zero-width runes are dropped from note text, so a Gitmoji headline or a stray escape code can't shift the ticker line.

## Global Constraints

- Module `github.com/RursusAeternum/GitAGarden`, `go 1.26.0`. Go is at `~/sdk/go/bin`: run `export PATH=$HOME/sdk/go/bin:$PATH` before any `go` command.
- No new third-party modules.
- With no reactions, `scene.Draw` produces exactly today's frame: `internal/scene/testdata/garden.golden` passes unchanged in every task. `--once` and replay never react.
- Ticker icons are two-cell default-emoji glyphs: 💧 🌸 ✨ 🐛 ✅ ⚡ 🌈 join the existing ones.
- Note wording is exactly as in spec §1: `💧 gag-core: "…" · 3 commits · by Claude`, `🌸 gag-core: merged #41 … · +1 more`, `✨ gag-core: released v0.6.0`, `🐛 gag-core: #31 …`, `✅ gag-core: closed #30 …`, `⚡ gag-core: CI failing on main`, `🌈 gag-core: CI passing again`.
- A plant's reactions play one at a time, at least 3 s apart. Notes last one minute.
- Agents are matched by email or GitHub login, never by a person's name. The only name rule is aider's `(aider)` suffix.
- `cmd/gag/demo.go` must not import `internal/github`. The browser build reaches it, and keeping GitHub out keeps `net/http` out of the wasm.
- Frame budget: a 240×65 live frame with a reaction playing on every plant renders in under 10 ms.
- After every task, `go vet ./... && go test ./...` passes and `gofmt -l .` prints nothing.
- Every commit message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Real-world commit headlines in notes.** Gitmoji with variation selectors, tag-sequence flags, tabs, escape codes and very long titles must leave the ticker line exactly the window's width. Pinned by Task 6 `TestNoteTextIsCleaned` and Task 7 `TestNoteTextKeepsTheTickerWhole`.
2. **A burst of changes across a panning garden,** such as every repo changing in one refresh after a long sleep. The visits must chain, then the camera resumes automatic panning, the queue drains, and nothing stays held. Pinned by Task 7 `TestManyChangesSettle`.
3. **The garden's order shifting while reactions are queued,** for example when a repo leaves. Reactions must follow their plant by name. Pinned by Task 7 `TestReactionsFollowTheirPlant`.
4. **Tiny and crowded plants:** a seedling with no top to speak of, a weed slot the plant covers, a flower at the grid's edge. Every reaction must draw without panicking and stay inside its slot, apart from the flying critters and the drone. Pinned by Task 5 `TestReactionsStayInTheirSlot`, which includes a seedling.
5. **A card open over a reacting plant, or a selection made mid-visit.** The card must stay on top, the selection must hold the camera, and the visits must stop. Pinned by Task 5 `TestCardCoversReactions` and Task 7 `TestSelectingDuringAVisitHoldsTheCamera`.

---

### Task 1: GitHub: which agent authored a commit

**Files:**
- Create: `internal/github/agents.go`, `internal/github/agents_test.go`
- Modify: `internal/github/repo.go` (`Commit.Agent`), `internal/github/sync.go` (authors in the history query)

**Interfaces:**
- Consumes:
  - `(*Client).commitsSince(ctx, owner, name string, since time.Time) ([]Commit, error)`
  - the test helper `fakeGitHub(t, answer)`
- Produces:
  - `Commit.Agent string` (JSON `agent,omitempty`)
  - unexported `gitActor`, `agentOf([]gitActor) string` and `noreplyLogin(email string) string`

- [ ] **Step 1: Write the failing test**

`internal/github/agents_test.go`:

```go
package github

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCommitsNameTheirAgents(t *testing.T) {
	c := fakeGitHub(t, func(q string) string {
		if !strings.Contains(q, "authors(first:3)") {
			t.Errorf("the history query should ask for authors: %s", q)
		}
		return `{"data":{"repository":{"defaultBranchRef":{"target":{"history":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[
			{"committedDate":"2026-09-28T10:00:00Z","messageHeadline":"pair work","authors":{"nodes":[
				{"name":"Kaine","email":"kaine@example.com","user":{"login":"kaine"}},
				{"name":"Claude","email":"noreply@anthropic.com","user":null}]}},
			{"committedDate":"2026-09-28T09:00:00Z","messageHeadline":"codex work","authors":{"nodes":[
				{"name":"chatgpt-codex-connector[bot]","email":"199175422+chatgpt-codex-connector[bot]@users.noreply.github.com","user":null}]}},
			{"committedDate":"2026-09-28T08:00:00Z","messageHeadline":"copilot work","authors":{"nodes":[
				{"name":"Copilot","email":"198982749+Copilot@users.noreply.github.com","user":{"login":"Copilot"}}]}},
			{"committedDate":"2026-09-28T07:00:00Z","messageHeadline":"a painting","authors":{"nodes":[
				{"name":"Claude Monet","email":"claude@monet.fr","user":{"login":"cmonet"}}]}},
			{"committedDate":"2026-09-28T06:00:00Z","messageHeadline":"aider work","authors":{"nodes":[
				{"name":"Paul (aider)","email":"paul@example.com","user":null}]}}
		]}}}}}}`
	})
	commits, err := c.commitsSince(context.Background(), "me", "x", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Claude", "Codex", "Copilot", "", "aider"}
	if len(commits) != len(want) {
		t.Fatalf("%d commits, want %d", len(commits), len(want))
	}
	for i, w := range want {
		if commits[i].Agent != w {
			t.Errorf("%q: agent %q, want %q", commits[i].Message, commits[i].Agent, w)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/github/ -run TestCommitsNameTheirAgents`
Expected: FAIL, with the build error `commits[i].Agent undefined`.

- [ ] **Step 3: Implement**

`internal/github/agents.go`:

```go
package github

import "strings"

// gitActor is one of a commit's authors as GitHub reports them, including
// co-authors from Co-Authored-By trailers.
type gitActor struct {
	Name  string
	Email string
	User  *struct{ Login string }
}

// agents are the AI coding agents GAG recognises, by an author's email or
// GitHub login and never by name alone: a person called Claude stays a
// person. An email starting with "@" matches its whole domain. One line per
// agent, so adding one is easy.
var agents = []struct {
	name           string
	emails, logins []string
}{
	{"Claude", []string{"noreply@anthropic.com"}, []string{"claude", "claude[bot]"}},
	{"Codex", []string{"@openai.com"}, []string{"chatgpt-codex-connector[bot]"}},
	{"Copilot", nil, []string{"copilot", "copilot-swe-agent[bot]"}},
	{"Devin", nil, []string{"devin-ai-integration[bot]"}},
	{"Cursor", []string{"cursoragent@cursor.com"}, nil},
	{"Gemini", nil, []string{"gemini-code-assist[bot]"}},
	{"Jules", nil, []string{"google-labs-jules[bot]"}},
}

// agentOf names the AI coding agent among a commit's authors, or "" when
// people wrote it. aider marks its commits by adding "(aider)" to the
// author's name: the one match by name.
func agentOf(authors []gitActor) string {
	for _, a := range authors {
		email := strings.ToLower(strings.TrimSpace(a.Email))
		login := ""
		if a.User != nil {
			login = strings.ToLower(a.User.Login)
		}
		if login == "" {
			login = noreplyLogin(email)
		}
		for _, ag := range agents {
			for _, e := range ag.emails {
				if email == e || (strings.HasPrefix(e, "@") && strings.HasSuffix(email, e)) {
					return ag.name
				}
			}
			for _, l := range ag.logins {
				if login == l {
					return ag.name
				}
			}
		}
		if strings.HasSuffix(strings.TrimSpace(a.Name), "(aider)") {
			return "aider"
		}
	}
	return ""
}

// noreplyLogin is the login in a GitHub no-reply address,
// <id>+<login>@users.noreply.github.com, or "" for any other address.
func noreplyLogin(email string) string {
	local, ok := strings.CutSuffix(email, "@users.noreply.github.com")
	if !ok {
		return ""
	}
	if _, login, ok := strings.Cut(local, "+"); ok {
		return login
	}
	return local
}
```

In `internal/github/repo.go`, replace

```go
type Commit struct {
	At      time.Time `json:"at"`
	Message string    `json:"msg"`
}
```

with

```go
type Commit struct {
	At      time.Time `json:"at"`
	Message string    `json:"msg"`
	Agent   string    `json:"agent,omitempty"` // the AI coding agent behind it; "" for people
}
```

In `internal/github/sync.go`, in `commitsSince`:

1. In the query string, replace `nodes{committedDate messageHeadline}` with `nodes{committedDate messageHeadline authors(first:3){nodes{name email user{login}}}}`.
2. Replace

```go
								Nodes    []struct {
									CommittedDate   time.Time
									MessageHeadline string
								}
```

with

```go
								Nodes    []struct {
									CommittedDate   time.Time
									MessageHeadline string
									Authors         struct{ Nodes []gitActor }
								}
```

3. Replace `				all = append(all, Commit{At: n.CommittedDate, Message: n.MessageHeadline})` with `				all = append(all, Commit{At: n.CommittedDate, Message: n.MessageHeadline, Agent: agentOf(n.Authors.Nodes)})`.

- [ ] **Step 4: Run the tests**

Run: `gofmt -w internal/github && go test ./internal/github/ && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/github
git commit -m "github: tell which AI coding agent, if any, authored each commit

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Card data for changes, and the demo in its own file

**Files:**
- Create: `cmd/gag/demo.go`
- Modify:
  - `internal/live/detail.go`: `Entry.By`, `Detail.LastMerge`, `Detail.LastClosed`
  - `cmd/gag/detail.go`: `detailFor` only, as a full replacement
  - `cmd/gag/garden.go`: move the demo code out
  - `cmd/gag/detail_test.go`: full replacement

**Interfaces:**
- Consumes: Task 1's `github.Commit.Agent`; `github.PR{Number, Title, MergedAt}`; `github.Issue.ClosedAt`.
- Produces:
  - `live.Entry.By string`
  - `live.Detail.LastMerge Entry` and `live.Detail.LastClosed Entry`
  - `cmd/gag/demo.go`, holding `demoItem`, `demoRepo` (plus `by string`), `demo`, `demoIssues`, `demoPRTitles`, `demoGarden(now)` and `demoDetail(r, events, now)`

- [ ] **Step 1: Write the failing tests**

Replace `cmd/gag/detail_test.go` with:

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./cmd/gag/`
Expected: FAIL, with build errors such as `unknown field By in struct literal of type live.Entry` and `unknown field LastMerge`.

- [ ] **Step 3: The live fields**

In `internal/live/detail.go`:

1. In `type Entry struct`, add after the `At` field: `	By     string // the AI coding agent behind a commit; "" for people`.
2. In `type Detail struct`, add after the `NewestIssue` field:

```go
	LastMerge   Entry   // the newest merged PR; zero when none
	LastClosed  Entry   // the most recently closed issue; zero when none
```

- [ ] **Step 4: `detailFor`, and the demo in its own file**

Replace `cmd/gag/detail.go` with:

```go
package main

import (
	"sort"
	"time"

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
			d.LastCommit = live.Entry{Title: c.Message, At: c.At, By: c.Agent}
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
	for _, pr := range r.PRs {
		if pr.MergedAt.After(d.LastMerge.At) {
			d.LastMerge = live.Entry{Number: pr.Number, Title: pr.Title, At: pr.MergedAt}
		}
	}
	for _, is := range r.Issues {
		if is.ClosedAt != nil {
			if is.ClosedAt.After(d.LastClosed.At) {
				d.LastClosed = live.Entry{Number: is.Number, Title: is.Title, At: *is.ClosedAt}
			}
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
```

In `cmd/gag/garden.go`, delete everything from the line `// demoItem is an open PR in the demo garden.` down to, but not including, the line `// repoFor grows a repo's plant and works out its signals as of now.`.

Create `cmd/gag/demo.go`:

```go
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
```

- [ ] **Step 5: Run the tests**

Run: `gofmt -w internal/live cmd/gag && go vet ./... && go test ./cmd/gag/ && go test ./...`
Expected: PASS. `TestDemoShowsTheSignals` and `TestOnceDrawsTheConfiguredSky` (48 stars) still pass.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/live cmd/gag
git commit -m "Card data for changes: last merge, last closed issue, who pushed; demo in demo.go

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Garden: new cells grow in

**Files:**
- Create: `internal/garden/reveal_test.go`
- Modify: `internal/garden/paint.go` (`Style.Before`, `Style.Grown`, `unfurl`), `internal/garden/plant.go` (`Spot`, `NewSpots`, `WeedSpot`, `Top`)

**Interfaces:**
- Consumes: v0.2's `Paint`, `Grow`, `FakeHistory`, `Plant.weedSlots`, `center`, `ground`; the test variable `t0` from `plant_test.go`.
- Produces:
  - `Style.Before *Plant` and `Style.Grown float64`
  - `type Spot struct{ X, Y int }` with `(Spot).At(baseX, baseY int) (int, int)`
  - `(*Plant).NewSpots(before *Plant, k CellKind) []Spot`
  - `(*Plant).WeedSpot(i int) (Spot, bool)`
  - `(*Plant).Top() int`
  - unexported `unfurl(g float64, x int) float64`

- [ ] **Step 1: Write the failing tests**

`internal/garden/reveal_test.go`:

```go
package garden

import (
	"slices"
	"testing"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// revealFrame paints p on a small canvas with its ground row near the bottom.
func revealFrame(p *Plant, st Style) *pixel.Canvas {
	c := pixel.New(Width+4, Height+4)
	Paint(c, p, center+2, Height+1, st)
	return c
}

func sameCanvas(a, b *pixel.Canvas) bool {
	for y := 0; y < a.H; y++ {
		for x := 0; x < a.W; x++ {
			if a.At(x, y) != b.At(x, y) {
				return false
			}
		}
	}
	return true
}

func paintedPixels(c *pixel.Canvas) int {
	n := 0
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if c.At(x, y) != (pixel.RGB{}) {
				n++
			}
		}
	}
	return n
}

func TestNewCellsGrowIn(t *testing.T) {
	events := FakeHistory("reveal", 80, t0)
	before, after := Grow("reveal", Shrub, events[:50]), Grow("reveal", Shrub, events)
	full := revealFrame(after, Style{Health: 1})
	if !sameCanvas(revealFrame(after, Style{Health: 1, Before: before, Grown: 1}), full) {
		t.Error("fully grown in, the new shape should look as painted plainly")
	}
	old := revealFrame(before, Style{Health: 1})
	start := revealFrame(after, Style{Health: 1, Before: before, Grown: 0})
	for y := 0; y < start.H; y++ {
		for x := 0; x < start.W; x++ {
			if start.At(x, y) != (pixel.RGB{}) && old.At(x, y) == (pixel.RGB{}) {
				t.Fatalf("pixel %d,%d shows before the new cells have begun to grow in", x, y)
			}
		}
	}
	if paintedPixels(start) >= paintedPixels(full) {
		t.Error("at the start the new cells should still be missing")
	}
}

func TestUnfurlStartsAtTheStem(t *testing.T) {
	for _, c := range []struct {
		g    float64
		x    int
		want float64
	}{{0, center, 0}, {0.5, center, 1}, {0.5, 0, 0}, {1, 0, 1}, {1, Width - 1, 1}} {
		if got := unfurl(c.g, c.x); got != c.want {
			t.Errorf("unfurl(%v, %d) = %v, want %v", c.g, c.x, got, c.want)
		}
	}
	if mid := unfurl(0.5, center/2); mid <= 0 || mid >= 1 {
		t.Errorf("halfway, a cell halfway out should be partly in, got %v", mid)
	}
}

func TestNewWeedsGrowIn(t *testing.T) {
	base := []Event{{Kind: Push, At: t0}, {Kind: IssueOpened, At: t0}}
	before := Grow("weeds", Shrub, base)
	after := Grow("weeds", Shrub, append(slices.Clone(base), Event{Kind: IssueOpened, At: t0}))
	sp, ok := after.WeedSpot(1)
	if !ok {
		t.Fatal("the second weed should be drawn")
	}
	x, y := sp.At(center+2, Height+1)
	if revealFrame(after, Style{Health: 1, Before: before, Grown: 0}).At(x, y) != (pixel.RGB{}) {
		t.Error("the new weed shows before it grows")
	}
	if revealFrame(after, Style{Health: 1, Before: before, Grown: 1}).At(x, y) == (pixel.RGB{}) {
		t.Error("the new weed never grows in")
	}
}

func TestSpotsAndTop(t *testing.T) {
	before := Grow("spots", Shrub, []Event{{Kind: Push, At: t0}})
	after := *before
	after.Grid[3][4] = Cell{Kind: Flower}
	if got := after.NewSpots(before, Flower); len(got) != 1 || got[0] != (Spot{4, 3}) {
		t.Errorf("NewSpots = %v, want [{4 3}]", got)
	}
	if after.Top() != 3 || before.Top() <= 3 {
		t.Errorf("Top: after %d (want 3), before %d (want below)", after.Top(), before.Top())
	}
	if x, y := (Spot{4, 3}).At(20, 40); x != 20-center+4 || y != 40-ground+3 {
		t.Errorf("Spot.At = %d,%d", x, y)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/garden/`
Expected: FAIL, with build errors such as `unknown field Before in struct literal of type Style` and `undefined: unfurl`.

- [ ] **Step 3: Implement**

In `internal/garden/paint.go`:

1. In `type Style struct`, add after the `Rising` field:

```go
	Before *Plant  // an earlier shape of this plant: the cells it lacks grow in
	Grown  float64 // how far they have come in, 0 to 1; by the stem first
```

2. In `Paint`, replace

```go
			c.Set(baseX-center+x+dx, baseY-ground+y, col.Scale(0.9+0.2*hash01(p.Name, x, y)))
```

with

```go
			col = col.Scale(0.9 + 0.2*hash01(p.Name, x, y))
			a := 1.0
			if st.Before != nil && st.Before.Grid[y][x].Kind != cell.Kind {
				a = unfurl(st.Grown, x) // a new cell, still coming in
			}
			c.Blend(baseX-center+x+dx, baseY-ground+y, col, a)
```

3. In `Paint`, replace

```go
		c.Set(baseX-center+s, baseY, weedColor)
		if i%2 == 0 {
			c.Set(baseX-center+s, baseY-1, weedColor.Scale(1.15))
		}
```

with

```go
		low, high := 1.0, 1.0
		if st.Before != nil && i >= st.Before.OpenIssues { // a new weed grows up from the soil
			low, high = math.Min(1, 2*st.Grown), math.Max(0, 2*st.Grown-1)
		}
		c.Blend(baseX-center+s, baseY, weedColor, low)
		if i%2 == 0 {
			c.Blend(baseX-center+s, baseY-1, weedColor.Scale(1.15), high)
		}
```

4. Add at the end of the file:

```go
// unfurl is how far a new cell in column x has come in once the plant's new
// cells are g of the way in: those by the stem first, the outermost last.
func unfurl(g float64, x int) float64 {
	d := math.Abs(float64(x-center)) / center
	return math.Max(0, math.Min(1, 2*g-d))
}
```

`pixel.Blend` with alpha 1 sets the colour exactly and with alpha 0 changes nothing, so plain painting is unchanged.

In `internal/garden/plant.go`, add at the end of the file:

```go
// Spot is a cell of the plant grid: X from the left, Y from the top.
type Spot struct{ X, Y int }

// At is where the spot lands on a canvas for a plant painted with its ground
// row on baseY and centered on column baseX, as Paint places it before sway.
func (s Spot) At(baseX, baseY int) (int, int) { return baseX - center + s.X, baseY - ground + s.Y }

// NewSpots lists the cells of kind k that p has and before hadn't, top to
// bottom, left to right.
func (p *Plant) NewSpots(before *Plant, k CellKind) []Spot {
	var out []Spot
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			if p.Grid[y][x].Kind == k && before.Grid[y][x].Kind != k {
				out = append(out, Spot{x, y})
			}
		}
	}
	return out
}

// WeedSpot is where the i-th weed grows, and false when that weed isn't
// drawn: past the last slot, or where the plant itself stands.
func (p *Plant) WeedSpot(i int) (Spot, bool) {
	if i < 0 || i >= len(p.weedSlots) {
		return Spot{}, false
	}
	s := p.weedSlots[i]
	return Spot{s, ground}, p.Grid[ground][s].Kind == Empty
}

// Top is the grid row of the plant's highest cell, or the ground row when it
// has none.
func (p *Plant) Top() int {
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			if p.Grid[y][x].Kind != Empty {
				return y
			}
		}
	}
	return ground
}
```

- [ ] **Step 4: Run the tests**

Run: `gofmt -w internal/garden && go test ./internal/garden/ && go test ./...`
Expected: PASS, including the scene golden frame.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/garden
git commit -m "garden: paint a plant's new cells growing in over its old shape

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Scene: reactions, the watering can, the drone and the weeds

**Files:**
- Create: `internal/scene/react.go`, `internal/scene/react_draw.go`, `internal/scene/react_test.go`
- Modify: `internal/scene/draw.go` (`View.Reactions`; `Draw` shapes plots and draws reactions)

**Interfaces:**
- Consumes:
  - Task 3's `Style.Before` and `Grown`, `NewSpots`, `WeedSpot`, `Spot.At` and `Top`
  - v0.6's `geometry`, `slotLeft`, `BedCols`, `headroom`, `plantBaseY`, `rainColor`, `critterInk`
  - the test helpers `demoPlots()`, `at()` and `colored()`
- Produces:
  - `type ReactKind int`, with `ReactPush`, `ReactMerge`, `ReactRelease`, `ReactWeedIn`, `ReactWeedOut`, `ReactStorm` and `ReactClear`
  - `type Reaction struct{ Plot int; Kind ReactKind; Start time.Time; Before *garden.Plant; Reveals, Drone bool; Seed int64 }`
  - `(Reaction).Duration() time.Duration` and `(Reaction).ChangeAt() time.Duration`
  - `View.Reactions []Reaction`
  - unexported, for Task 5: `(View).shaped(i int, pl Plot) Plot`, `placedPlot`, `drawReactions`, `plantTop`, `plantBase`, `slotBlend`

- [ ] **Step 1: Write the failing tests**

`internal/scene/react_test.go`:

```go
package scene

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// earlier is gag-core's plant from early on in the demo plots' history.
func earlier() *garden.Plant {
	return garden.Grow("gag-core", garden.Shrub, garden.FakeHistory("gag-core", 20, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
}

// reacting is the demo plots at 14:00 with r on the first, age into it.
func reacting(r Reaction, age time.Duration) View {
	now := at(14, 0)
	r.Plot, r.Start = 1, now.Add(-age)
	return View{Cols: 3 * BedCols, Plots: demoPlots(), Now: now, Seed: 1, Reactions: []Reaction{r}}
}

func frameText(v View) string { return Draw(v).Encode(pixel.TrueColor) }

func TestReactionsPlayThenLeave(t *testing.T) {
	plain := frameText(View{Cols: 3 * BedCols, Plots: demoPlots(), Now: at(14, 0), Seed: 1})
	for _, r := range []Reaction{{Kind: ReactPush}, {Kind: ReactPush, Drone: true}, {Kind: ReactWeedIn}, {Kind: ReactWeedOut}} {
		r.Before, r.Reveals, r.Seed = earlier(), true, 3
		if frameText(reacting(r, r.Duration()/2)) == plain {
			t.Errorf("kind %v (drone %v): nothing drawn halfway through", r.Kind, r.Drone)
		}
		if frameText(reacting(r, r.Duration())) != plain {
			t.Errorf("kind %v (drone %v): something left behind after it ended", r.Kind, r.Drone)
		}
	}
}

func TestOldShapeUntilTheChange(t *testing.T) {
	pl, before := demoPlots()[0], earlier()
	v := View{Plots: []Plot{pl}, Now: at(14, 0)}
	shape := func(age time.Duration) Plot {
		v.Reactions = []Reaction{{Plot: 1, Kind: ReactPush, Start: v.Now.Add(-age), Before: before, Reveals: true}}
		return v.shaped(0, pl)
	}
	for _, age := range []time.Duration{-time.Second, 0, 700 * time.Millisecond} {
		if got := shape(age); got.Plant != before || got.Style.Before != nil {
			t.Errorf("%v in: the plant should keep its old shape", age)
		}
	}
	if got := shape(1300 * time.Millisecond); got.Plant != pl.Plant || got.Style.Before != before || got.Style.Grown <= 0 || got.Style.Grown >= 1 {
		t.Errorf("1.3 s in: the new cells should be growing in, got Grown %v", got.Style.Grown)
	}
	if got := shape(2 * time.Second); got.Plant != pl.Plant || got.Style.Before != nil {
		t.Error("2 s in: the plant should have its new shape")
	}
}

func TestWeatherWaitsForTheStorm(t *testing.T) {
	pl := demoPlots()[0]
	pl.Weather = Storm
	v := View{Plots: []Plot{pl}, Now: at(14, 0)}
	v.Reactions = []Reaction{{Plot: 1, Kind: ReactStorm, Start: v.Now.Add(-time.Second)}}
	if v.shaped(0, pl).Weather != Clear {
		t.Error("the steady storm should wait for its cloud to roll in")
	}
	v.Reactions[0].Start = v.Now.Add(-1600 * time.Millisecond)
	if v.shaped(0, pl).Weather != Storm {
		t.Error("once the cloud has arrived the storm should stay")
	}
	pl.Weather = Clear
	v.Reactions = []Reaction{{Plot: 1, Kind: ReactClear, Start: v.Now.Add(time.Second)}}
	if v.shaped(0, pl).Weather != Storm {
		t.Error("the old storm should stay until its clearing starts")
	}
	v.Reactions[0].Start = v.Now.Add(-time.Second)
	if v.shaped(0, pl).Weather != Clear {
		t.Error("once clearing, the storm should be gone")
	}
}

func TestDroneReplacesTheCan(t *testing.T) {
	can := Draw(reacting(Reaction{Kind: ReactPush, Before: earlier(), Reveals: true}, 800*time.Millisecond))
	drone := Draw(reacting(Reaction{Kind: ReactPush, Drone: true, Before: earlier(), Reveals: true}, 800*time.Millisecond))
	if colored(can, canColor) == 0 || colored(can, droneBody) != 0 {
		t.Errorf("a person's push: can %d px, drone %d px", colored(can, canColor), colored(can, droneBody))
	}
	if colored(drone, droneBody) == 0 || colored(drone, canColor) != 0 {
		t.Errorf("an agent's push: drone %d px, can %d px", colored(drone, droneBody), colored(drone, canColor))
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/scene/`
Expected: FAIL, with build errors such as `undefined: Reaction` and `unknown field Reactions in struct literal of type View`.

- [ ] **Step 3: Reactions**

`internal/scene/react.go`:

```go
package scene

import (
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// ReactKind is what a reaction shows: the change that caused it.
type ReactKind int

const (
	ReactPush    ReactKind = iota // a watering can, or an agent's drone, waters the plant
	ReactMerge                    // a bud bursts and a butterfly flies off
	ReactRelease                  // sparkles, then bees
	ReactWeedIn                   // a new issue's weed comes up
	ReactWeedOut                  // a closed issue's weed is pulled
	ReactStorm                    // CI failed: the storm rolls in
	ReactClear                    // CI recovered: the storm clears, a rainbow
)

// Reaction is an animation on one plot, from Start.
type Reaction struct {
	Plot    int // 1 + the plot's index, like View.Selected
	Kind    ReactKind
	Start   time.Time
	Before  *garden.Plant // the plant before the change; nil when unknown
	Reveals bool          // the plot shows Before until this reaction's change moment
	Drone   bool          // a push by an AI agent: a drone waters instead of a can
	Seed    int64
}

// growFor is how long new cells take to come in.
const growFor = time.Second

// Duration is how long the reaction plays.
func (r Reaction) Duration() time.Duration {
	switch r.Kind {
	case ReactPush:
		if r.Drone {
			return 3 * time.Second
		}
		return 2500 * time.Millisecond
	case ReactMerge:
		return 3 * time.Second
	case ReactRelease:
		return 4 * time.Second
	case ReactWeedIn, ReactWeedOut:
		return 1500 * time.Millisecond
	case ReactStorm:
		return 2 * time.Second
	case ReactClear:
		return 5 * time.Second
	}
	return 0
}

// ChangeAt is when, into the reaction, the plot takes its new shape: when
// the drops land, the bud bursts or the storm cloud arrives.
func (r Reaction) ChangeAt() time.Duration {
	switch r.Kind {
	case ReactPush:
		if r.Drone {
			return time.Second
		}
		return 800 * time.Millisecond
	case ReactMerge:
		return 600 * time.Millisecond
	case ReactStorm:
		return 1500 * time.Millisecond
	}
	return 0
}

// shaped is plot i as its reactions show it at v.Now. Until a revealing
// reaction's change moment, even while that reaction still waits, the plant
// keeps its old shape; then its new cells grow in over growFor. A storm's
// steady weather waits for its cloud to roll in, and a clearing storm stays
// until its reaction starts.
func (v View) shaped(i int, pl Plot) Plot {
	for _, r := range v.Reactions {
		if r.Plot != i+1 {
			continue
		}
		age := v.Now.Sub(r.Start)
		if age >= r.Duration() {
			continue
		}
		if r.Reveals && r.Before != nil && !pl.Finished {
			switch at := r.ChangeAt(); {
			case age < at:
				pl.Plant = r.Before
			case age < at+growFor:
				pl.Style.Before, pl.Style.Grown = r.Before, float64(age-at)/float64(growFor)
			}
		}
		switch {
		case r.Kind == ReactStorm && age < r.ChangeAt():
			pl.Weather = Clear
		case r.Kind == ReactClear && age < 0:
			pl.Weather = Storm
		}
	}
	return pl
}

// placedPlot is where Draw put a plot: its index, the column its slot is
// centered on, and its bed's top pixel row.
type placedPlot struct{ index, cx, oy int }

// drawReactions draws the reactions playing on one placed plot.
func drawReactions(c *pixel.Canvas, v View, p placedPlot) {
	pl := v.Plots[p.index]
	for _, r := range v.Reactions {
		if r.Plot != p.index+1 || pl.Finished {
			continue
		}
		age := v.Now.Sub(r.Start)
		if age < 0 || age >= r.Duration() {
			continue
		}
		switch r.Kind {
		case ReactPush:
			drawWatering(c, r, age, p.cx, p.oy, plantTop(pl.Plant, p.oy))
		case ReactWeedIn:
			drawWeedIn(c, r, age, p.cx, p.oy, pl.Plant)
		case ReactWeedOut:
			drawWeedOut(c, r, age, p.cx, p.oy)
		}
	}
}

// plantTop is the pixel row of a plant's highest cell, in a bed whose top is
// pixel row oy.
func plantTop(p *garden.Plant, oy int) int { return oy + headroom + p.Top() }

// plantBase is the pixel row a bed's plants stand on.
func plantBase(oy int) int { return oy + plantBaseY }

// slotBlend blends col onto (x, y) when x is inside the slot centered on cx,
// so a plot's effects stay in its own slot.
func slotBlend(c *pixel.Canvas, cx, x, y int, col pixel.RGB, a float64) {
	if x >= slotLeft(cx) && x < slotLeft(cx)+BedCols {
		c.Blend(x, y, col, a)
	}
}
```

`internal/scene/react_draw.go`:

```go
package scene

import (
	"math"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

var (
	canColor   = rgb(96, 132, 128) // galvanised tin
	canShine   = rgb(170, 200, 196)
	droneBody  = rgb(150, 155, 165)
	droneRotor = rgb(210, 215, 225)
	dropColor  = rainColor // steel blue: darker than the day sky, lighter than the night
	dirtColor  = rgb(120, 85, 55)
	weedGreen  = rgb(130, 140, 60) // the garden's weed colour
)

// drawWatering draws a push. A watering can tips over the plant, or for an
// agent's push a drone flies in, hovers and flies off; either sprinkles an
// arc of drops that land on the plant at the change moment. top is the
// plant's top pixel row.
func drawWatering(c *pixel.Canvas, r Reaction, age time.Duration, cx, oy, top int) {
	s := age.Seconds()
	hoverX, hoverY := cx+4, max(top-7, oy+1)
	pour0, pour1 := 0.3, 1.1 // when the first and last drops leave
	if r.Drone {
		pour0, pour1 = 0.6, 1.4
		x := hoverX
		switch {
		case s < 0.6:
			x = hoverX + int(math.Round((1-s/0.6)*12)) // flies in from the right
		case s > 1.8:
			x = hoverX - int(math.Round((s-1.8)/1.2*24)) // and off to the left
		}
		drawDrone(c, x, hoverY, s)
	} else if s < 1.6 {
		drawCan(c, hoverX, hoverY, s > pour0)
	}
	for k := 0; k < 6; k++ { // each drop falls for half a second
		t := s - (pour0 + (pour1-pour0)*float64(k)/6)
		if t < 0 || t > 0.5 {
			continue
		}
		f := t / 0.5
		x := hoverX - 2 - int(math.Round(f*float64(3+k%3)))
		y := hoverY + 2 + int(math.Round(f*f*float64(top-hoverY)))
		c.Set(x, y, dropColor)
	}
}

// drawCan is a small watering can at (x, y): a body 4 px wide, a handle on
// the right and a spout to the left that dips while it pours.
func drawCan(c *pixel.Canvas, x, y int, pouring bool) {
	for dx := 0; dx < 4; dx++ {
		for dy := 0; dy < 3; dy++ {
			c.Set(x+dx, y+dy, canColor)
		}
	}
	c.Set(x+1, y, canShine)
	c.Set(x+4, y+1, canColor)
	c.Set(x-1, y+1, canColor)
	if pouring {
		c.Set(x-2, y+2, canColor)
	} else {
		c.Set(x-2, y, canColor)
	}
}

// drawDrone is a small quadcopter centered on column x: rotor arms along row
// y whose blades blur as they spin, a grey body and a nozzle below.
func drawDrone(c *pixel.Canvas, x, y int, secs float64) {
	spin := int(secs*20) % 2
	for dx := -3; dx <= 3; dx++ {
		if dx == -3 || dx == 3 || (dx+3+spin)%2 == 0 {
			c.Set(x+dx, y, droneRotor)
		}
	}
	for dx := -1; dx <= 1; dx++ {
		c.Set(x+dx, y+1, droneBody)
	}
	c.Set(x, y+2, critterInk)
}

// drawWeedIn draws a new issue: a puff of dirt where its weed comes up. The
// weed itself grows in with the plant's new shape.
func drawWeedIn(c *pixel.Canvas, r Reaction, age time.Duration, cx, oy int, now *garden.Plant) {
	s := age.Seconds()
	if r.Before == nil || s >= 0.6 {
		return
	}
	sp, ok := now.WeedSpot(r.Before.OpenIssues)
	if !ok {
		return
	}
	x, y := sp.At(cx, plantBase(oy))
	a, lift := 1-s/0.6, int(s*4)
	for k, d := range [][2]int{{-2, 0}, {2, 0}, {-1, -1}, {1, -1}} {
		slotBlend(c, cx, x+d[0]*(1+lift/2), y+d[1]-lift*(k%2), dirtColor, a)
	}
}

// drawWeedOut draws a closed issue: the pulled weed lifts out of the soil,
// rises and fades.
func drawWeedOut(c *pixel.Canvas, r Reaction, age time.Duration, cx, oy int) {
	if r.Before == nil || r.Before.OpenIssues == 0 {
		return
	}
	i := r.Before.OpenIssues - 1 // the last weed is the one that goes
	sp, ok := r.Before.WeedSpot(i)
	if !ok {
		return
	}
	x, y := sp.At(cx, plantBase(oy))
	f := age.Seconds() / 1.5
	lift, a := int(math.Round(f*8)), 1-f
	slotBlend(c, cx, x, y-lift, weedGreen, a)
	if i%2 == 0 {
		slotBlend(c, cx, x, y-1-lift, weedGreen, a)
	}
}
```

- [ ] **Step 4: Draw them**

In `internal/scene/draw.go`:

1. In `type View struct`, add after the `Card` field:

```go
	Reactions  []Reaction // change animations on plots, playing or waiting
```

2. In `Draw`, replace `	var hosts []Host` with:

```go
	var hosts []Host
	var placed []placedPlot
```

3. In `Draw`, replace

```go
		for _, s := range g.slots(v, b) {
			pl := v.Plots[s.index]
```

with

```go
		for _, s := range g.slots(v, b) {
			pl := v.shaped(s.index, v.Plots[s.index])
			placed = append(placed, placedPlot{s.index, s.cx, oy})
```

4. In `Draw`, replace

```go
	if v.Motion {
		DrawCritters(c, v.Now, hosts, v.Seed)
	}
```

with

```go
	if v.Motion {
		DrawCritters(c, v.Now, hosts, v.Seed)
	}
	for _, p := range placed {
		drawReactions(c, v, p)
	}
```

- [ ] **Step 5: Run the tests**

Run: `gofmt -w internal/scene && go vet ./internal/scene/ && go test ./internal/scene/ && go test ./...`
Expected: PASS, and `TestComposeGolden` passes unchanged.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/scene
git commit -m "scene: reactions; a can or an agent's drone waters, weeds come and go

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Scene: bursts, sparkles, storms and rainbows

**Files:**
- Modify:
  - `internal/scene/react_draw.go`: the merge, release, storm and clear effects, and `cloudIn`
  - `internal/scene/react.go`: new cases in `drawReactions`
  - `internal/scene/critters.go`: `drawBee` and `drawButterfly`, taken out of `DrawCritters`
- Test: `internal/scene/react_test.go` (append)

**Interfaces:**
- Consumes:
  - Task 4's `Reaction`, `drawReactions`, `plantTop`, `plantBase`, `slotBlend`, `reacting`, `earlier` and `frameText`
  - `puff`, `greyCloud`, `stormCloud`, `lightning`, `beeBody`, `beeWing`, `critterInk`, `wingColors`, `noise`
  - the test helpers `dist()`, `sampleCard()` and `CardRect`
- Produces: unexported `drawBurst`, `drawSparkle`, `drawStormIn`, `drawClearing`, `cloudIn`, `drawBee`, `drawButterfly`, `budPink`, `petalColor`, `sparkGold`, `sparkWhite` and `rainbow`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/scene/react_test.go`:

```go
func TestMoreReactionsPlayThenLeave(t *testing.T) {
	plain := frameText(View{Cols: 3 * BedCols, Plots: demoPlots(), Now: at(14, 0), Seed: 1})
	for _, k := range []ReactKind{ReactMerge, ReactRelease, ReactStorm, ReactClear} {
		r := Reaction{Kind: k, Before: earlier(), Reveals: true, Seed: 5}
		if frameText(reacting(r, r.Duration()/2)) == plain {
			t.Errorf("kind %v: nothing drawn halfway through", k)
		}
		if frameText(reacting(r, r.Duration())) != plain {
			t.Errorf("kind %v: something left behind after it ended", k)
		}
	}
}

func TestReactionColoursStandOut(t *testing.T) {
	for _, now := range []time.Time{at(12, 0), at(23, 30)} {
		top, bottom := SkyAt(now)
		for name, col := range map[string]pixel.RGB{
			"drops": dropColor, "gold sparkle": sparkGold, "white sparkle": sparkWhite, "drone": droneBody,
			"rainbow red": rainbow[0], "rainbow yellow": rainbow[2], "rainbow violet": rainbow[5],
		} {
			if dist(col, top) < 60 || dist(col, bottom) < 60 {
				t.Errorf("%s %v blends into the %s sky", name, col, now.Format("15:04"))
			}
		}
	}
}

// strayPixels counts the pixels r changes outside the middle plot's slot, at
// age into it. The frame is 3 plots wide with 5-column margins, so the middle
// slot is columns 31-56.
func strayPixels(plots []Plot, r Reaction, age time.Duration) int {
	now := at(14, 0)
	v := View{Cols: 3*BedCols + 10, Plots: plots, Now: now, Seed: 1}
	plain := Draw(v)
	r.Plot, r.Start = 2, now.Add(-age)
	v.Reactions = []Reaction{r}
	got := Draw(v)
	n := 0
	for y := 0; y < got.H; y++ {
		for x := 0; x < got.W; x++ {
			if (x < 31 || x >= 31+BedCols) && plain.At(x, y) != got.At(x, y) {
				n++
			}
		}
	}
	return n
}

func TestReactionsStayInTheirSlot(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seedling := demoPlots()
	seedling[1] = Plot{Plant: garden.Grow("tiny", garden.Shrub, []garden.Event{{Kind: garden.Push, At: t0}}),
		Style: garden.Style{Health: 1}, Name: "tiny", Status: "new"}
	gardens := []struct {
		plots  []Plot
		before *garden.Plant
	}{
		{demoPlots(), garden.Grow("rustyfs", garden.Cactus, garden.FakeHistory("rustyfs", 30, t0))},
		{seedling, garden.Grow("tiny", garden.Shrub, nil)},
	}
	ms := func(ms ...int) []time.Duration {
		var out []time.Duration
		for _, m := range ms {
			out = append(out, time.Duration(m)*time.Millisecond)
		}
		return out
	}
	cases := []struct {
		kind ReactKind
		ages []time.Duration // before any critter takes off
	}{
		{ReactPush, ms(200, 600, 1100, 1500, 2000)},
		{ReactMerge, ms(300, 800)},
		{ReactRelease, ms(300, 900)},
		{ReactWeedIn, ms(200, 1000)},
		{ReactWeedOut, ms(200, 1000)},
		{ReactStorm, ms(300, 1000, 1600)},
		{ReactClear, ms(300, 1200, 3000, 4800)},
	}
	for gi, g := range gardens {
		for _, c := range cases {
			for _, age := range c.ages {
				r := Reaction{Kind: c.kind, Before: g.before, Reveals: true, Seed: 9}
				if n := strayPixels(g.plots, r, age); n > 0 {
					t.Errorf("garden %d, kind %v at %v: %d pixels outside its slot", gi, c.kind, age, n)
				}
			}
		}
	}
}

func TestCardCoversReactions(t *testing.T) {
	now := at(14, 0)
	v := View{Cols: 3 * BedCols, Plots: demoPlots(), Now: now, Seed: 1, Selected: 1, Card: sampleCard()}
	x, y, w, h, ok := CardRect(v)
	if !ok {
		t.Fatal("no card")
	}
	plain := Draw(v)
	v.Reactions = []Reaction{{Plot: 2, Kind: ReactRelease, Start: now.Add(-time.Second), Seed: 1}}
	got := Draw(v)
	for py := 2 * y; py < 2*(y+h); py++ {
		for px := x; px < x+w; px++ {
			if plain.At(px, py) != got.At(px, py) {
				t.Fatalf("a reaction shows through the card at %d,%d", px, py)
			}
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/scene/`
Expected: FAIL, with build errors such as `undefined: sparkGold` and `undefined: rainbow`.

- [ ] **Step 3: The critter helpers**

In `internal/scene/critters.go`, in `DrawCritters`, replace

```go
		if i%2 == 0 {
			c.Set(x, y, beeBody)
			c.Set(x+1, y, critterInk)
			c.Set(x, y-1, beeWing)
			continue
		}
		wing := wingColors[i%len(wingColors)]
		c.Set(x, y, critterInk)
		if int(secs*6)%2 == 0 { // wings open
			c.Set(x-1, y-1, wing)
			c.Set(x+1, y-1, wing)
		} else {
			c.Set(x-1, y, wing)
			c.Set(x+1, y, wing)
		}
```

with

```go
		if i%2 == 0 {
			drawBee(c, x, y)
			continue
		}
		drawButterfly(c, x, y, wingColors[i%len(wingColors)], int(secs*6)%2 == 0)
```

and add at the end of the file:

```go
// drawBee is a bee at (x, y): a yellow body, a dark tail and a pale wing.
func drawBee(c *pixel.Canvas, x, y int) {
	c.Set(x, y, beeBody)
	c.Set(x+1, y, critterInk)
	c.Set(x, y-1, beeWing)
}

// drawButterfly is a butterfly at (x, y) with wings of wing, open or closed.
func drawButterfly(c *pixel.Canvas, x, y int, wing pixel.RGB, open bool) {
	c.Set(x, y, critterInk)
	if open {
		c.Set(x-1, y-1, wing)
		c.Set(x+1, y-1, wing)
	} else {
		c.Set(x-1, y, wing)
		c.Set(x+1, y, wing)
	}
}
```

- [ ] **Step 4: The effects**

Append to `internal/scene/react_draw.go`:

```go
var (
	budPink    = rgb(235, 110, 150)
	petalColor = rgb(255, 170, 210)
	sparkGold  = rgb(255, 215, 90)
	sparkWhite = rgb(255, 255, 255)
	rainbow    = []pixel.RGB{rgb(230, 80, 70), rgb(240, 150, 60), rgb(240, 220, 90),
		rgb(110, 200, 100), rgb(90, 150, 230), rgb(150, 110, 210)}
)

// drawBurst draws a merge. A bud swells where the new flower opens and
// bursts into petals at the change moment, then a butterfly lifts off and
// flies away.
func drawBurst(c *pixel.Canvas, r Reaction, age time.Duration, cx, oy int, now *garden.Plant) {
	x, y := cx, plantTop(now, oy)
	if r.Before != nil {
		if spots := now.NewSpots(r.Before, garden.Flower); len(spots) > 0 {
			x, y = spots[0].At(cx, plantBase(oy))
		}
	}
	s := age.Seconds()
	switch {
	case s < 0.6:
		slotBlend(c, cx, x, y, budPink, 1)
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			slotBlend(c, cx, x+d[0], y+d[1], budPink, s/0.6)
		}
	case s < 1.0:
		f := (s - 0.6) / 0.4
		rad := 1 + f*3
		for k := 0; k < 8; k++ {
			th := float64(k) * math.Pi / 4
			slotBlend(c, cx, x+int(math.Round(rad*math.Cos(th))), y+int(math.Round(rad*math.Sin(th))), petalColor, 1-f/2)
		}
	}
	if s >= 1.0 {
		f := (s - 1.0) / 2.0
		bx := x + int(math.Round(f*14))
		by := y - int(math.Round(f*18)) + int(math.Round(math.Sin(s*9)))
		drawButterfly(c, bx, by, wingColors[int(r.Seed&0xff)%len(wingColors)], int(s*6)%2 == 0)
	}
}

// drawSparkle draws a release: gold and white sparkles burst out around the
// plant, then two bees circle it and leave.
func drawSparkle(c *pixel.Canvas, r Reaction, age time.Duration, cx, oy int, now *garden.Plant) {
	s := age.Seconds()
	mx, my := cx, (plantTop(now, oy)+plantBase(oy))/2
	if s < 1.5 {
		rad, a := 3+7*s/1.5, 1-s/1.5
		for k := 0; k < 10; k++ {
			th := float64(k)*math.Pi/5 + noise(r.Seed, k, 1)
			x, y := mx+int(math.Round(rad*math.Cos(th))), my+int(math.Round(rad*0.8*math.Sin(th)))
			slotBlend(c, cx, x, y, sparkWhite, a)
			for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				slotBlend(c, cx, x+d[0], y+d[1], sparkGold, a*0.8)
			}
		}
	}
	if s >= 1.0 {
		t := s - 1.0
		out := math.Max(0, t-2.2) / 0.8 // in their last 0.8 s they fly off
		for i := 0; i < 2; i++ {
			th, rad := 2.4*t+float64(i)*math.Pi, 7+20*out
			drawBee(c, mx+int(math.Round(rad*math.Cos(th))), my+int(math.Round(rad*0.6*math.Sin(th))))
		}
	}
}

// drawStormIn draws CI going red. The storm cloud slides down into place,
// darkening as it comes, and flashes once as it arrives; after that the
// plot's steady storm takes over.
func drawStormIn(c *pixel.Canvas, age time.Duration, cx, oy int) {
	top, s := max(oy, 0), age.Seconds()
	if s < 1.5 {
		f := s / 1.5
		cloudIn(c, cx, cx, top-5+int(math.Round(f*6)), 8, pixel.Lerp(greyCloud, stormCloud, f), 1, top)
		return
	}
	if s < 1.8 {
		for i, dx := range []int{-1, 0, -1, 0, 1} {
			c.Set(cx+dx, top+3+i, lightning)
		}
	}
}

// drawClearing draws CI recovering: the storm cloud drifts off and fades,
// and a rainbow arcs over the plant, fading in and out.
func drawClearing(c *pixel.Canvas, age time.Duration, cx, oy int) {
	top, s := max(oy, 0), age.Seconds()
	if s < 1.5 {
		f := s / 1.5
		cloudIn(c, cx, cx+int(math.Round(f*6)), top+1, 8, stormCloud, 1-f, top)
	}
	if s < 1.0 || s >= 5.0 {
		return
	}
	a := 0.85 * math.Min(1, (s-1.0)/0.5) * math.Min(1, (5.0-s)/0.5)
	for b, col := range rainbow {
		rad := float64(10 - b)
		for k := 0; k <= 24; k++ {
			th := math.Pi + math.Pi*float64(k)/24
			slotBlend(c, cx, cx+int(math.Round(rad*math.Cos(th))), top+13+int(math.Round(rad*0.7*math.Sin(th))), col, a)
		}
	}
}

// cloudIn draws a cloud the shape of puff, centered on column x with its base
// on row y+1, blended by a. It is clipped to the slot centered on slotCx and
// to rows from minY down.
func cloudIn(c *pixel.Canvas, slotCx, x, y, half int, col pixel.RGB, a float64, minY int) {
	for dx := -half; dx <= half; dx++ {
		for _, yy := range []int{y - 1, y, y + 1} {
			in := yy == y+1 || (yy == y && dx > -half && dx < half) || (yy == y-1 && dx >= -half/2 && dx <= half/2)
			if in && yy >= minY {
				slotBlend(c, slotCx, x+dx, yy, col, a)
			}
		}
	}
}
```

In `internal/scene/react.go`, in `drawReactions`, replace

```go
		case ReactWeedOut:
			drawWeedOut(c, r, age, p.cx, p.oy)
		}
```

with

```go
		case ReactWeedOut:
			drawWeedOut(c, r, age, p.cx, p.oy)
		case ReactMerge:
			drawBurst(c, r, age, p.cx, p.oy, pl.Plant)
		case ReactRelease:
			drawSparkle(c, r, age, p.cx, p.oy, pl.Plant)
		case ReactStorm:
			drawStormIn(c, age, p.cx, p.oy)
		case ReactClear:
			drawClearing(c, age, p.cx, p.oy)
		}
```

- [ ] **Step 5: Run the tests**

Run: `gofmt -w internal/scene && go vet ./internal/scene/ && go test ./internal/scene/ && go test ./...`
Expected: PASS, with the golden frame unchanged and the critter behaviour unchanged.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/scene
git commit -m "scene: merges burst, releases sparkle, storms roll in and clear to a rainbow

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Live: what changed between two loads

**Files:**
- Create: `internal/live/changes.go`, `internal/live/changes_test.go`
- Modify: `internal/live/ticker_test.go` (the icon list)

**Interfaces:**
- Consumes:
  - Task 2's `Detail.LastMerge`, `Detail.LastClosed` and `Entry.By`
  - Task 4's `scene.ReactKind` constants
  - `Repo{Name, Plant, Finished, Branch, CI, Detail}`, `CIFailing` and `CIPassing`
  - the test helpers `grow` and `t0`
- Produces:
  - `type Change struct{ Repo string; Kind scene.ReactKind; Icon, Text string; Before *garden.Plant; Agent string }`
  - `func ChangesBetween(before, after []Repo) []Change`
  - unexported `tickerText(s string) string`

- [ ] **Step 1: Write the failing tests**

`internal/live/changes_test.go`:

```go
package live

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

// baseRepo is gag-core as a refresh found it: CI passing on main, a release
// and an open issue.
func baseRepo() Repo {
	r := grow("gag-core", 10, 1, t0.Add(-2*time.Hour))
	r.Branch, r.CI = "main", CIPassing
	r.Detail = Detail{
		LastCommit:  Entry{Title: "old", At: t0.Add(-2 * time.Hour)},
		Release:     Entry{Title: "v0.5.0", At: t0.Add(-48 * time.Hour)},
		NewestIssue: Entry{Number: 30, Title: "Old bug", At: t0.Add(-72 * time.Hour)},
	}
	return r
}

// regrown is r with its plant grown to pushes and merges, as of t0.
func regrown(r Repo, pushes, merges int) Repo {
	r.Plant = grow(r.Name, pushes, merges, t0).Plant
	return r
}

func TestChangesAreDetected(t *testing.T) {
	cases := []struct {
		name   string
		change func(Repo) Repo
		kind   scene.ReactKind
		note   string
		agent  string
	}{
		{"push", func(r Repo) Repo {
			r = regrown(r, 13, 1)
			r.Detail.LastCommit = Entry{Title: "Draw the card beside the plant", At: t0, By: "Claude"}
			return r
		}, scene.ReactPush, `💧 gag-core: "Draw the card beside the plant" · 3 commits · by Claude`, "Claude"},
		{"merge", func(r Repo) Repo {
			r = regrown(r, 10, 3)
			r.Detail.LastMerge = Entry{Number: 41, Title: "Add a detail card", At: t0}
			return r
		}, scene.ReactMerge, "🌸 gag-core: merged #41 Add a detail card · +1 more", ""},
		{"release", func(r Repo) Repo {
			r.Detail.Release = Entry{Title: "v0.6.0", At: t0}
			return r
		}, scene.ReactRelease, "✨ gag-core: released v0.6.0", ""},
		{"new issue", func(r Repo) Repo {
			r.Detail.NewestIssue = Entry{Number: 31, Title: "Crash on empty repo", At: t0}
			return r
		}, scene.ReactWeedIn, "🐛 gag-core: #31 Crash on empty repo", ""},
		{"closed issue", func(r Repo) Repo {
			r.Detail.LastClosed = Entry{Number: 30, Title: "Old bug", At: t0}
			return r
		}, scene.ReactWeedOut, "✅ gag-core: closed #30 Old bug", ""},
		{"CI red", func(r Repo) Repo {
			r.CI = CIFailing
			return r
		}, scene.ReactStorm, "⚡ gag-core: CI failing on main", ""},
	}
	for _, c := range cases {
		before := baseRepo()
		got := ChangesBetween([]Repo{before}, []Repo{c.change(baseRepo())})
		if len(got) != 1 {
			t.Errorf("%s: %d changes: %+v", c.name, len(got), got)
			continue
		}
		g := got[0]
		if g.Repo != "gag-core" || g.Kind != c.kind || g.Icon+" "+g.Text != c.note || g.Agent != c.agent ||
			g.Before == nil || g.Before.Pushes != before.Plant.Pushes {
			t.Errorf("%s: %+v, want kind %v, %q, agent %q", c.name, g, c.kind, c.note, c.agent)
		}
	}
	red := baseRepo()
	red.CI = CIFailing
	if got := ChangesBetween([]Repo{red}, []Repo{baseRepo()}); len(got) != 1 || got[0].Kind != scene.ReactClear ||
		got[0].Icon+" "+got[0].Text != "🌈 gag-core: CI passing again" {
		t.Errorf("CI green: %+v", got)
	}
}

func TestASquashMergeIsNotAPush(t *testing.T) {
	after := regrown(baseRepo(), 11, 2) // one new commit: the merge's own
	after.Detail.LastCommit = Entry{Title: "Add a detail card (#41)", At: t0}
	after.Detail.LastMerge = Entry{Number: 41, Title: "Add a detail card", At: t0}
	if got := ChangesBetween([]Repo{baseRepo()}, []Repo{after}); len(got) != 1 || got[0].Kind != scene.ReactMerge {
		t.Errorf("a squash merge: %+v, want just the merge", got)
	}
}

func TestChangesThatDontCount(t *testing.T) {
	pushed := regrown(baseRepo(), 12, 1)
	pushed.Detail.LastCommit = Entry{Title: "new", At: t0}
	if got := ChangesBetween(nil, []Repo{pushed}); got != nil {
		t.Errorf("the first load is the baseline: %+v", got)
	}
	newcomer := pushed
	newcomer.Name = "newcomer"
	if got := ChangesBetween([]Repo{baseRepo()}, []Repo{baseRepo(), newcomer}); len(got) != 0 {
		t.Errorf("a repo that just joined: %+v", got)
	}
	if got := ChangesBetween([]Repo{baseRepo(), baseRepo()}, []Repo{pushed, pushed}); len(got) != 0 {
		t.Errorf("repos sharing a name: %+v", got)
	}
	done := pushed
	done.Finished = true
	if got := ChangesBetween([]Repo{baseRepo()}, []Repo{done}); len(got) != 0 {
		t.Errorf("a finished repo: %+v", got)
	}
	if got := ChangesBetween([]Repo{baseRepo()}, []Repo{baseRepo()}); len(got) != 0 {
		t.Errorf("nothing changed: %+v", got)
	}
}

func TestChangesComeInOrder(t *testing.T) {
	a := regrown(baseRepo(), 14, 2) // four commits, one of them a merge's
	a.CI = CIFailing
	a.Detail.LastCommit = Entry{Title: "Speed up", At: t0}
	a.Detail.LastMerge = Entry{Number: 41, Title: "Add a detail card", At: t0}
	other := grow("zz", 5, 0, t0.Add(-time.Hour))
	red := other
	red.CI = CIFailing
	got := ChangesBetween([]Repo{baseRepo(), other}, []Repo{a, red})
	want := []struct {
		repo string
		kind scene.ReactKind
	}{{"gag-core", scene.ReactPush}, {"gag-core", scene.ReactMerge}, {"gag-core", scene.ReactStorm}, {"zz", scene.ReactStorm}}
	if len(got) != len(want) {
		t.Fatalf("changes: %+v", got)
	}
	for i, w := range want {
		if got[i].Repo != w.repo || got[i].Kind != w.kind {
			t.Errorf("change %d = %s %v, want %s %v", i, got[i].Repo, got[i].Kind, w.repo, w.kind)
		}
	}
	if got[0].Text != `gag-core: "Speed up" · 3 commits` || got[3].Text != "zz: CI failing" {
		t.Errorf("notes: %q, %q", got[0].Text, got[3].Text)
	}
}

func TestNoteTextIsCleaned(t *testing.T) {
	a := regrown(baseRepo(), 11, 1)
	a.Detail.LastCommit = Entry{Title: "♻️ Tidy\tup\x1b[31m", At: t0}
	if got := ChangesBetween([]Repo{baseRepo()}, []Repo{a}); len(got) != 1 || got[0].Text != `gag-core: "♻ Tidy up[31m"` {
		t.Errorf("cleaned note: %+v", got)
	}
}
```

In `internal/live/ticker_test.go`, change `[]string{"⚡", "🌷", "🥀", "🐌", "🌱", "⭐"}` to `[]string{"⚡", "🌷", "🥀", "🐌", "🌱", "⭐", "💧", "🌸", "✨", "🐛", "✅", "🌈"}`. This check passes as soon as it's written, because it only guards the chosen glyphs.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/live/`
Expected: FAIL, with the build errors `undefined: ChangesBetween` and `undefined: scene.ReactPush`.

- [ ] **Step 3: Implement**

`internal/live/changes.go`:

```go
package live

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

// Change is something that happened to a repo between two online loads,
// with the ticker note that says so.
type Change struct {
	Repo   string
	Kind   scene.ReactKind
	Icon   string        // the note's icon
	Text   string        // and its text
	Before *garden.Plant // the plant before the change
	Agent  string        // the AI agent behind a push; "" for people
}

// ChangesBetween lists what happened from before to after: repos in after's
// order and, per repo, push, merge, release, new issue, closed issue, then
// CI. There are none without a before, and none for repos that just joined,
// repos sharing a short name, or finished repos.
func ChangesBetween(before, after []Repo) []Change {
	if before == nil {
		return nil
	}
	was, now := unique(before), unique(after)
	var out []Change
	for _, r := range after {
		b, known := was[r.Name]
		if _, single := now[r.Name]; !known || !single || r.Finished || b.Finished || r.Plant == nil || b.Plant == nil {
			continue
		}
		out = append(out, changesOf(b, r)...)
	}
	return out
}

// unique maps repo names to repos, leaving out names that appear twice: a
// fork and its upstream can't be told apart.
func unique(repos []Repo) map[string]Repo {
	count := map[string]int{}
	for _, r := range repos {
		count[r.Name]++
	}
	out := make(map[string]Repo, len(repos))
	for _, r := range repos {
		if count[r.Name] == 1 {
			out[r.Name] = r
		}
	}
	return out
}

// changesOf is what happened to one repo from b to r.
func changesOf(b, r Repo) []Change {
	var out []Change
	add := func(k scene.ReactKind, icon, text, agent string) {
		out = append(out, Change{Repo: r.Name, Kind: k, Icon: icon, Text: r.Name + ": " + text, Before: b.Plant, Agent: agent})
	}
	d, bd := r.Detail, b.Detail
	pushes, merges := r.Plant.Pushes-b.Plant.Pushes, r.Plant.Merges-b.Plant.Merges
	if d.LastCommit.At.After(bd.LastCommit.At) && pushes > merges { // not when the new commits are the merges' own
		text := "pushed"
		if title := tickerText(d.LastCommit.Title); title != "" {
			text = `"` + title + `"`
		}
		if n := pushes - merges; n > 1 {
			text += fmt.Sprintf(" · %d commits", n)
		}
		if d.LastCommit.By != "" {
			text += " · by " + d.LastCommit.By
		}
		add(scene.ReactPush, "💧", text, d.LastCommit.By)
	}
	if merges > 0 {
		text := "merged a PR"
		if d.LastMerge.Number != 0 {
			text = fmt.Sprintf("merged #%d %s", d.LastMerge.Number, tickerText(d.LastMerge.Title))
		}
		if merges > 1 {
			text += fmt.Sprintf(" · +%d more", merges-1)
		}
		add(scene.ReactMerge, "🌸", text, "")
	}
	if d.Release.Title != "" && d.Release.Title != bd.Release.Title {
		add(scene.ReactRelease, "✨", "released "+tickerText(d.Release.Title), "")
	}
	if d.NewestIssue.Number != 0 && d.NewestIssue.At.After(bd.NewestIssue.At) {
		add(scene.ReactWeedIn, "🐛", fmt.Sprintf("#%d %s", d.NewestIssue.Number, tickerText(d.NewestIssue.Title)), "")
	}
	if d.LastClosed.Number != 0 && d.LastClosed.At.After(bd.LastClosed.At) {
		add(scene.ReactWeedOut, "✅", fmt.Sprintf("closed #%d %s", d.LastClosed.Number, tickerText(d.LastClosed.Title)), "")
	}
	switch {
	case r.CI == CIFailing && b.CI != CIFailing:
		text := "CI failing"
		if r.Branch != "" {
			text += " on " + r.Branch
		}
		add(scene.ReactStorm, "⚡", text, "")
	case b.CI == CIFailing && r.CI == CIPassing:
		add(scene.ReactClear, "🌈", "CI passing again", "")
	}
	return out
}

// tickerText cleans text for a ticker note. Tabs and newlines become spaces,
// and control characters and zero-width runes (joiners, combining marks,
// variation selectors) go, so a Gitmoji or a stray escape code can't shift
// the line.
func tickerText(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n':
			return ' '
		case unicode.IsControl(r), unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Variation_Selector):
			return -1
		}
		return r
	}, s)
}
```

- [ ] **Step 4: Run the tests**

Run: `gofmt -w internal/live && go test ./internal/live/ && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/live
git commit -m "live: tell what changed between two loads, and word its ticker note

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Live: reactions, camera visits and change notes

**Files:**
- Create: `internal/live/react.go`, `internal/live/react_test.go`
- Modify: `internal/live/model.go` (notes shared by stars and changes; `loadedMsg`, the tick, `busy`, `prune`, `sceneView` and `tickerLine`)

**Interfaces:**
- Consumes:
  - Task 6's `ChangesBetween` and `Change`
  - Task 4's `scene.Reaction` and `View.Reactions`
  - v0.6's `holdOn`, `release`, `layout`, `pan`, `sceneView`, `scene.OnScreen` and `scene.SlideFor`
  - the test helpers `ready`, `clocked`, `newModel`, `step`, `eight`, `garden3`, `grow`, `arrow`, `contains`, `checkSize`, `visible` and `t0`
- Produces:
  - `note{repo string; n int; icon, text string; from, until time.Time}`, replacing `starNote`
  - `reaction{repo string; anim scene.Reaction}` and `visit{repo string; from, until time.Time; started bool}`
  - `Model` fields `seen []Repo`, `seenDemo bool`, `reacts []reaction` and `visits []visit`
  - methods `noticeChanges`, `schedule`, `queue`, `queueEnd`, `indexOf`, `runVisits`, `reacting`, `addChangeNote`, `noteItems` and `latestNote`
  - constants `reactGap` and `visitLinger`

- [ ] **Step 1: Write the failing tests**

`internal/live/react_test.go`:

```go
package live

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

// pushedTo is r after new commits: its plant grown to pushes, the newest
// commit titled title and made by by (an agent, or "" for a person) at at.
func pushedTo(r Repo, pushes int, at time.Time, title, by string) Repo {
	out := grow(r.Name, pushes, r.Plant.Merges, at)
	out.Branch, out.CI, out.Stars, out.Finished = r.Branch, r.CI, r.Stars, r.Finished
	out.Detail = r.Detail
	out.Detail.LastCommit = Entry{Title: title, At: at, By: by}
	return out
}

func tickerOf(m Model) string {
	lines := strings.Split(m.View(), "\n")
	return visible(lines[len(lines)-1])
}

func TestAPushReactsAndNotes(t *testing.T) {
	now := t0
	m := ready(clocked(garden3(), &now), 100, 30)
	after := garden3()
	after.Repos[0] = pushedTo(after.Repos[0], 65, t0, "Draw the card beside the plant", "Claude")
	m, _ = step(m, loadedMsg{snap: after})
	if len(m.reacts) != 1 {
		t.Fatalf("reactions = %+v, want one", m.reacts)
	}
	r := m.reacts[0]
	if r.repo != "bloom" || r.anim.Kind != scene.ReactPush || !r.anim.Drone || !r.anim.Reveals ||
		!r.anim.Start.Equal(t0.Add(slowFrame)) || r.anim.Before == nil || r.anim.Before.Pushes != 60 {
		t.Errorf("reaction = %+v", r)
	}
	if m.frameInterval() != fastFrame {
		t.Error("a waiting reaction needs the fast frame rate")
	}
	if strings.Contains(tickerOf(m), "💧") {
		t.Error("the note should wait for its reaction to start")
	}
	now = now.Add(slowFrame)
	if want := `💧 bloom: "Draw the card beside the plant" · 5 commits · by Claude`; !strings.Contains(tickerOf(m), want) {
		t.Errorf("ticker = %q, want %q", tickerOf(m), want)
	}
	if v := m.sceneView(m.now()); len(v.Reactions) != 1 || v.Reactions[0].Plot != 1 {
		t.Errorf("the frame's reactions = %+v", v.Reactions)
	}
}

func TestQuietLoadsDontReact(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	offline := garden3()
	offline.Offline = true
	offline.Repos[0] = pushedTo(offline.Repos[0], 65, t0, "cached", "")
	if m, _ = step(m, loadedMsg{snap: offline}); len(m.reacts) != 0 {
		t.Error("an offline load should not react")
	}
	demo := garden3()
	demo.Demo, demo.Note = true, "demo · no GitHub token: run gh auth login"
	m = ready(newModel(demo, nil, t0), 100, 30)
	real := garden3()
	real.Repos[0] = pushedTo(real.Repos[0], 65, t0, "real", "")
	if m, _ = step(m, loadedMsg{snap: real}); len(m.reacts) != 0 {
		t.Error("signing in after the demo should not react")
	}
}

func TestAPlantsReactionsQueue(t *testing.T) {
	now := t0
	m := ready(clocked(garden3(), &now), 100, 30)
	after := garden3()
	r := pushedTo(after.Repos[0], 63, t0, "Speed up", "")
	r.Plant = grow("bloom", 63, 5, t0).Plant
	r.Detail.LastMerge = Entry{Number: 41, Title: "Add a detail card", At: t0}
	r.CI, r.Branch = CIFailing, "main"
	after.Repos[0] = r
	m, _ = step(m, loadedMsg{snap: after})
	want := []scene.ReactKind{scene.ReactPush, scene.ReactMerge, scene.ReactStorm}
	if len(m.reacts) != len(want) {
		t.Fatalf("reactions = %+v", m.reacts)
	}
	for i, w := range want {
		a := m.reacts[i].anim
		if a.Kind != w || a.Reveals != (i == 0) {
			t.Errorf("reaction %d: kind %v reveals %v, want %v and %v", i, a.Kind, a.Reveals, w, i == 0)
		}
		if i > 0 {
			prev := m.reacts[i-1].anim
			if gap := a.Start.Sub(prev.Start); gap < reactGap || gap < prev.Duration() {
				t.Errorf("reactions %d and %d start %v apart", i-1, i, gap)
			}
		}
	}
}

func TestCameraVisitsOffScreenPlants(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	after := eight()
	after.Repos[6] = pushedTo(after.Repos[6], 25, t0, "Far away", "")
	m, _ = step(m, loadedMsg{snap: after})
	if len(m.visits) != 1 || m.visits[0].repo != "p6" || len(m.reacts) != 1 {
		t.Fatalf("visits %+v, reactions %+v", m.visits, m.reacts)
	}
	vis := m.visits[0]
	if !m.reacts[0].anim.Start.Equal(vis.from.Add(scene.SlideFor)) {
		t.Errorf("the reaction should start as the camera arrives")
	}
	now = vis.from
	m, _ = step(m, tickMsg{})
	now = now.Add(scene.SlideFor)
	if on := scene.OnScreen(m.sceneView(m.now())); !contains(on, 6) {
		t.Errorf("during the visit the plants on screen are %v; want p6 among them", on)
	}
	held, _ := m.pan(m.layout())
	now = vis.until
	m, _ = step(m, tickMsg{})
	if len(m.visits) != 0 || m.cam.held {
		t.Errorf("after the visit: visits %+v, held %v", m.visits, m.cam.held)
	}
	if p, _ := m.pan(m.layout()); p != held {
		t.Errorf("automatic panning should resume from %v, not %v", held, p)
	}
}

func TestSelectionKeepsTheCameraStill(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	m, _ = step(m, arrow(tea.KeyRight)) // p0
	after := eight()
	after.Repos[6] = pushedTo(after.Repos[6], 25, t0, "Far away", "")
	m, _ = step(m, loadedMsg{snap: after})
	if len(m.visits) != 0 || len(m.reacts) != 1 || !m.reacts[0].anim.Start.Equal(t0.Add(slowFrame)) {
		t.Fatalf("visits %+v, reactions %+v", m.visits, m.reacts)
	}
	now = now.Add(slowFrame)
	if !strings.Contains(tickerOf(m), `💧 p6: "Far away"`) {
		t.Errorf("the note should still appear: %q", tickerOf(m))
	}
}

func TestSelectingDuringAVisitHoldsTheCamera(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	after := eight()
	after.Repos[6] = pushedTo(after.Repos[6], 25, t0, "Far away", "")
	m, _ = step(m, loadedMsg{snap: after})
	now = m.visits[0].from
	m, _ = step(m, tickMsg{})
	m, _ = step(m, arrow(tea.KeyLeft))
	name := m.sel.name
	m, _ = step(m, tickMsg{})
	if len(m.visits) != 0 || !m.cam.held || m.sel.name != name || name == "" {
		t.Errorf("visits %+v, held %v, selected %q", m.visits, m.cam.held, m.sel.name)
	}
}

func TestManyChangesSettle(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	after := eight()
	for i := range after.Repos {
		after.Repos[i] = pushedTo(after.Repos[i], 25, t0, fmt.Sprintf("change %d", i), "")
	}
	m, _ = step(m, loadedMsg{snap: after})
	if len(m.reacts) != 8 {
		t.Fatalf("%d reactions, want 8", len(m.reacts))
	}
	for s := 0; s < 60; s++ {
		now = now.Add(time.Second)
		m, _ = step(m, tickMsg{})
	}
	if len(m.visits) != 0 || m.cam.held || len(m.reacts) != 0 || m.reacting() {
		t.Errorf("after a minute: visits %+v, held %v, %d reactions", m.visits, m.cam.held, len(m.reacts))
	}
}

func TestReactionsFollowTheirPlant(t *testing.T) {
	now := t0
	m := ready(clocked(garden3(), &now), 100, 30)
	after := garden3()
	after.Repos[2] = pushedTo(after.Repos[2], 5, t0, "seedling", "")
	m, _ = step(m, loadedMsg{snap: after})
	fewer := after
	fewer.Repos = after.Repos[1:] // bloom leaves: seed moves from 3rd to 2nd
	m, _ = step(m, loadedMsg{snap: fewer})
	if v := m.sceneView(m.now()); len(v.Reactions) != 1 || v.Reactions[0].Plot != 2 {
		t.Errorf("the reaction should follow seed to plot 2: %+v", v.Reactions)
	}
}

func TestNoteTextKeepsTheTickerWhole(t *testing.T) {
	now := t0
	m := ready(clocked(garden3(), &now), 100, 30)
	after := garden3()
	title := "♻️ Refactor\tthe pan \U0001F3F4\U000E0067\U000E0062\U000E0073\U000E0063\U000E0074\U000E007F" + strings.Repeat(" very long", 20)
	after.Repos[0] = pushedTo(after.Repos[0], 61, t0, title, "")
	m, _ = step(m, loadedMsg{snap: after})
	now = now.Add(slowFrame)
	checkSize(t, m.View(), 100, 30)
}

func BenchmarkLiveFrameWithReactions(b *testing.B) {
	snap := garden3()
	for i := 0; len(snap.Repos) < 12; i++ {
		r := snap.Repos[i%3]
		r.Name += strings.Repeat("x", i+1)
		snap.Repos = append(snap.Repos, r)
	}
	m := ready(newModel(snap, nil, t0), 240, 65)
	for i, r := range m.repos {
		m.reacts = append(m.reacts, reaction{repo: r.Name, anim: scene.Reaction{Kind: scene.ReactKind(i % 7),
			Start: t0.Add(-time.Second), Before: grow(r.Name, 5, 0, t0).Plant, Reveals: true, Drone: i%2 == 0, Seed: int64(i)}})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/live/`
Expected: FAIL, with build errors such as `m.reacts undefined` and `undefined: reaction`.

- [ ] **Step 3: Scheduling**

`internal/live/react.go`:

```go
package live

import (
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

const (
	reactGap    = 3 * time.Second // a plant's reactions start at least this far apart
	visitLinger = time.Second     // the camera stays this long after a visit's reactions
)

// reaction is an animation scheduled on a repo's plant; its Plot is filled
// in when a frame is drawn, since plants can move.
type reaction struct {
	repo string
	anim scene.Reaction
}

// visit is the camera going to an off-screen plant while its reactions
// play, from from until until, in scene time.
type visit struct {
	repo        string
	from, until time.Time
	started     bool
}

// noticeChanges compares an online load with the one before and schedules
// a reaction and a ticker note for every change. Switching between the demo
// garden and a real one starts a fresh baseline.
func (m *Model) noticeChanges(snap Snapshot) {
	if snap.Demo != m.seenDemo {
		m.seen = nil
	}
	m.seenDemo = snap.Demo
	changes := ChangesBetween(m.seen, snap.Repos)
	m.seen = snap.Repos
	if m.seen == nil {
		m.seen = []Repo{} // loaded, even if empty: the next load compares with it
	}
	m.schedule(changes)
}

// schedule queues each changed plant's reactions. Plants on screen start at
// once. When the garden pans and nothing is selected, the camera then visits
// the others in turn, and their reactions start as it arrives.
func (m *Model) schedule(changes []Change) {
	if len(changes) == 0 {
		return
	}
	at := m.now()
	base := at.Add(slowFrame) // by then the fast frames have started
	on := map[int]bool{}
	for _, i := range scene.OnScreen(m.sceneView(at)) {
		on[i] = true
	}
	visiting := m.layout().Overflow && m.sel.name == ""
	var later [][]Change
	nextVisit := base
	for len(changes) > 0 {
		n := 1
		for n < len(changes) && changes[n].Repo == changes[0].Repo {
			n++
		}
		group := changes[:n]
		changes = changes[n:]
		i := m.indexOf(group[0].Repo)
		if i < 0 {
			continue
		}
		if visiting && !on[i] {
			later = append(later, group)
			continue
		}
		if end := m.queue(group, laterOf(base, m.queueEnd(group[0].Repo))); end.After(nextVisit) {
			nextVisit = end
		}
	}
	for _, v := range m.visits {
		nextVisit = laterOf(nextVisit, v.until)
	}
	for _, group := range later {
		from := laterOf(nextVisit, m.queueEnd(group[0].Repo))
		end := m.queue(group, from.Add(scene.SlideFor))
		nextVisit = end.Add(visitLinger)
		m.visits = append(m.visits, visit{repo: group[0].Repo, from: from, until: nextVisit})
	}
}

// queue schedules one plant's changes one at a time from start, at least
// reactGap apart. Each change's note appears as its reaction starts, and
// the first reaction brings the plant's new shape. It returns when the last
// one ends.
func (m *Model) queue(group []Change, start time.Time) time.Time {
	t, end := start, start
	for k, ch := range group {
		r := scene.Reaction{Kind: ch.Kind, Start: t, Before: ch.Before, Reveals: k == 0, Drone: ch.Agent != "",
			Seed: t.UnixMilli() + int64(k)}
		m.reacts = append(m.reacts, reaction{repo: ch.Repo, anim: r})
		m.addChangeNote(ch.Icon, ch.Text, t.Add(-m.cfg.Ahead))
		end = t.Add(r.Duration())
		t = t.Add(max(r.Duration(), reactGap))
	}
	return end
}

// queueEnd is when repo's last scheduled reaction ends; zero when none.
func (m Model) queueEnd(repo string) time.Time {
	var end time.Time
	for _, r := range m.reacts {
		if e := r.anim.Start.Add(r.anim.Duration()); r.repo == repo && e.After(end) {
			end = e
		}
	}
	return end
}

// indexOf is the index of the repo named name, or -1.
func (m Model) indexOf(name string) int {
	for i, r := range m.repos {
		if r.Name == name {
			return i
		}
	}
	return -1
}

// runVisits moves the camera for the visits due now: to each changed
// off-screen plant as its visit starts, and back to automatic panning after
// the last one ends. A selection holds the camera, so it cancels them.
func (m *Model) runVisits() {
	if len(m.visits) == 0 {
		return
	}
	if m.sel.name != "" {
		m.visits = nil
		return
	}
	at := m.now()
	var keep []visit
	ended, busy := false, false
	for _, v := range m.visits {
		if !at.Before(v.until) {
			ended = true
			continue
		}
		if !v.started && !at.Before(v.from) {
			if i := m.indexOf(v.repo); i >= 0 {
				m.holdOn(i)
			}
			v.started = true
		}
		busy = busy || v.started
		keep = append(keep, v)
	}
	m.visits = keep
	if ended && !busy {
		m.release()
	}
}

// reacting reports whether a reaction is playing or waiting, or the camera
// has visits to make.
func (m Model) reacting() bool {
	at := m.now()
	for _, r := range m.reacts {
		if at.Before(r.anim.Start.Add(r.anim.Duration())) {
			return true
		}
	}
	return len(m.visits) > 0
}

// laterOf is the later of two times.
func laterOf(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
```

- [ ] **Step 4: Notes for every change, and the model's wiring**

In `internal/live/model.go`:

1. Add `"sort"` to the imports.
2. In `type Model struct`, replace

```go
	notes      []starNote       // ticker notes about new stars
	rotFrom    time.Time        // the ticker's rotation restarts here when a star note arrives
```

with

```go
	notes      []note           // ticker notes about new stars and other changes
	seen       []Repo           // the repos at the last online load; nil before the first
	seenDemo   bool             // whether that load was the demo garden
	reacts     []reaction       // change animations, playing or waiting
	visits     []visit          // the camera's visits to off-screen plants that changed
```

3. Replace

```go
// starNote tells the ticker that repo got n new stars, until until.
type starNote struct {
	repo  string
	n     int
	until time.Time
}
```

with

```go
// note is a ticker note about something that just happened, shown from from
// until until, ahead of the attention items. A star note counts new stars
// (n) and merges with the next ones; a change note has its icon and text.
type note struct {
	repo        string
	n           int
	icon, text  string
	from, until time.Time
}
```

4. In `noticeNewStars`, delete the line `			m.rotFrom = wall // show the note now, while its shooting star flies`.
5. Replace the whole `addNote` function with:

```go
// addNote tells the ticker repo got n new stars. While that repo's star note
// is still showing, the stars are added to it and its minute starts over.
func (m *Model) addNote(repo string, n int, wall time.Time) {
	notes := slices.Clone(m.notes)
	for i := range notes {
		if notes[i].icon == "" && notes[i].repo == repo && wall.Before(notes[i].until) {
			notes[i].n += n
			notes[i].from, notes[i].until = wall, wall.Add(noteFor)
			m.notes = notes
			return
		}
	}
	m.notes = append(notes, note{repo: repo, n: n, from: wall, until: wall.Add(noteFor)})
}

// addChangeNote tells the ticker what changed, for a minute from from.
func (m *Model) addChangeNote(icon, text string, from time.Time) {
	m.notes = append(slices.Clone(m.notes), note{icon: icon, text: text, from: from, until: from.Add(noteFor)})
}
```

6. In `prune`, replace `	var notes []starNote` with `	var notes []note`, and replace `	m.shots, m.notes = shots, notes` with:

```go
	var reacts []reaction
	for _, r := range m.reacts {
		if at.Before(r.anim.Start.Add(r.anim.Duration())) {
			reacts = append(reacts, r)
		}
	}
	m.shots, m.notes, m.reacts = shots, notes, reacts
```

7. Replace the whole `starItems` function with:

```go
// noteItems are the ticker's notes showing now, newest first: new stars and
// other changes.
func (m Model) noteItems() []Item {
	wall := m.cfg.Now()
	var showing []note
	for _, n := range m.notes {
		if !wall.Before(n.from) && wall.Before(n.until) {
			showing = append(showing, n)
		}
	}
	sort.SliceStable(showing, func(i, j int) bool { return showing[i].from.After(showing[j].from) })
	var items []Item
	for _, n := range showing {
		if n.icon != "" {
			items = append(items, Item{n.icon, n.text})
			continue
		}
		text := "New star on " + n.repo
		if n.n > 1 {
			text = fmt.Sprintf("%d new stars on %s", n.n, n.repo)
		}
		items = append(items, Item{"⭐", text})
	}
	return items
}

// latestNote is when the newest note showing now appeared. The ticker's
// rotation restarts there, so a new note shows at once.
func (m Model) latestNote() (time.Time, bool) {
	wall := m.cfg.Now()
	var latest time.Time
	for _, n := range m.notes {
		if !wall.Before(n.from) && wall.Before(n.until) && n.from.After(latest) {
			latest = n.from
		}
	}
	return latest, !latest.IsZero()
}
```

8. In `tickerLine`, replace

```go
	elapsed := m.elapsed()
	if !m.rotFrom.IsZero() {
		elapsed = m.cfg.Now().Sub(m.rotFrom)
	}
	return Line(append(m.starItems(), items...), elapsed, status, m.cols)
```

with

```go
	elapsed := m.elapsed()
	if from, ok := m.latestNote(); ok {
		elapsed = m.cfg.Now().Sub(from)
	}
	return Line(append(m.noteItems(), items...), elapsed, status, m.cols)
```

9. In `case loadedMsg:`, replace

```go
			if !msg.snap.Offline && !msg.snap.Demo { // cached or fake counts are no news
				m.noticeNewStars(msg.snap.Repos)
			}
```

with

```go
			if !msg.snap.Offline && !msg.snap.Demo { // cached or fake counts are no news
				m.noticeNewStars(msg.snap.Repos)
			}
			if !msg.snap.Offline {
				m.noticeChanges(msg.snap)
			}
```

10. In `case tickMsg:`, replace

```go
		if m.sel.name != "" && m.cfg.Now().Round(0).Sub(m.sel.lastInput) >= idleClear {
			m.unselect()
		}
```

with

```go
		if m.sel.name != "" && m.cfg.Now().Round(0).Sub(m.sel.lastInput) >= idleClear {
			m.unselect()
		}
		m.runVisits()
```

11. In `busy`, replace

```go
	for _, s := range m.shots {
		if at.Sub(s.Start) < scene.ShootingFor {
			return true // a shooting star is flying, or queued to
		}
	}
```

with

```go
	for _, s := range m.shots {
		if at.Sub(s.Start) < scene.ShootingFor {
			return true // a shooting star is flying, or queued to
		}
	}
	if m.reacting() {
		return true // a reaction plays or waits, or the camera has visits to make
	}
```

12. In `sceneView`, replace `	v.Pan, _ = m.pan(m.layout())` with:

```go
	v.Pan, _ = m.pan(m.layout())
	for _, r := range m.reacts {
		if i := m.indexOf(r.repo); i >= 0 {
			a := r.anim
			a.Plot = i + 1
			v.Reactions = append(v.Reactions, a)
		}
	}
```

- [ ] **Step 5: Run the tests and the frame budget**

Run: `gofmt -w internal/live && go vet ./... && go test ./internal/live/ && go test -race ./internal/live/ && go test ./... && go test ./internal/live/ -bench LiveFrame -run '^$'`
Expected:
- PASS, including the v0.5 star-note tests.
- `BenchmarkLiveFrameWithReactions` reports less than `10000000 ns/op`.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/live
git commit -m "live: changes react on their plants, the camera visits, notes say what happened

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: The demo acts it out

**Files:**
- Create: `cmd/gag/demo_test.go`
- Modify:
  - `cmd/gag/demo.go`: the world
  - `cmd/gag/garden.go`: the demo source and the 30 s refresh
  - `cmd/gag/main_js.go`: the browser demo
  - `README.md`

**Interfaces:**
- Consumes:
  - Task 2's `demoRepo` (with `by`), `demo`, `demoDetail`, `demoIssues` and `demoPRTitles`
  - Task 6's `live.ChangesBetween`
  - Task 4's `scene.ReactKind` constants
- Produces:
  - `const demoRefresh = 30 * time.Second`
  - `type demoWorld`, `newDemoWorld(now) *demoWorld`, `(*demoWorld).snapshot(now) live.Snapshot` and `(*demoWorld).garden(now) []live.Repo`
  - `sourceState.world`

- [ ] **Step 1: Write the failing tests**

`cmd/gag/demo_test.go`:

```go
package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/live"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func TestDemoWorldActsOutEveryKind(t *testing.T) {
	now := time.Now()
	w := newDemoWorld(now)
	prev := w.snapshot(now)
	var kinds []scene.ReactKind
	var agents []string
	for i := 0; i < 14; i++ {
		now = now.Add(demoRefresh)
		next := w.snapshot(now)
		changes := live.ChangesBetween(prev.Repos, next.Repos)
		if len(changes) != 1 {
			t.Fatalf("step %d made %d changes: %+v", i, len(changes), changes)
		}
		kinds = append(kinds, changes[0].Kind)
		if changes[0].Kind == scene.ReactPush {
			agents = append(agents, changes[0].Agent)
		}
		prev = next
	}
	cycle := []scene.ReactKind{scene.ReactPush, scene.ReactMerge, scene.ReactRelease, scene.ReactWeedIn,
		scene.ReactWeedOut, scene.ReactStorm, scene.ReactClear}
	if want := append(append([]scene.ReactKind{}, cycle...), cycle...); !reflect.DeepEqual(kinds, want) {
		t.Errorf("kinds = %v, want %v", kinds, want)
	}
	if !reflect.DeepEqual(agents, []string{"", "Claude"}) {
		t.Errorf("the demo's pushes were by %q; want a person, then Claude", agents)
	}
}

func TestDemoWorldStartsAsTheDemo(t *testing.T) {
	now := time.Now()
	first, still := newDemoWorld(now).snapshot(now), demoGarden(now)
	if !first.Demo || len(first.Repos) != len(still) {
		t.Fatalf("first snapshot: demo %v, %d repos", first.Demo, len(first.Repos))
	}
	for i := range still {
		if first.Repos[i].Name != still[i].Name || first.Repos[i].Plant.Pushes != still[i].Plant.Pushes {
			t.Errorf("repo %d: %s with %d pushes, want %s with %d", i, first.Repos[i].Name,
				first.Repos[i].Plant.Pushes, still[i].Name, still[i].Plant.Pushes)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./cmd/gag/`
Expected: FAIL, with the build errors `undefined: newDemoWorld` and `undefined: demoRefresh`.

- [ ] **Step 3: The world**

In `cmd/gag/demo.go`, add `"math/rand"` and `"sync"` to the imports. Then replace the whole `demoGarden` function with:

```go
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
		lr := live.Repo{Name: r.name, Plant: garden.Grow(r.name, garden.SpeciesFor(r.lang), s.events),
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
```

- [ ] **Step 4: The live demo and the browser use it**

In `cmd/gag/garden.go`:

1. Replace

```go
type sourceState struct {
	real bool // a real GitHub garden has loaded
}
```

with

```go
type sourceState struct {
	real  bool       // a real GitHub garden has loaded
	world *demoWorld // the -demo garden in the live view: it changes on every load
}
```

2. In `snapshot`, replace

```go
	if s.demo {
		return live.Snapshot{Repos: demoGarden(now), FetchedAt: now, Note: "demo garden", Demo: true}, nil
	}
```

with

```go
	if s.demo {
		if s.state == nil { // one frame: the demo as it starts
			return live.Snapshot{Repos: demoGarden(now), FetchedAt: now, Note: "demo garden", Demo: true}, nil
		}
		if s.state.world == nil {
			s.state.world = newDemoWorld(now)
		}
		return s.state.world.snapshot(now), nil
	}
```

3. In `runGarden`, directly after the line that starts `	opts := merge(loadConfig(), set,`, add:

```go
	if *demoFlag && !set["refresh"] {
		opts.refresh = demoRefresh // the live demo acts out a change on every load
	}
```

In `cmd/gag/main_js.go`, replace

```go
		load := func(context.Context, func(live.Progress)) (live.Snapshot, error) {
			now := time.Now()
			return live.Snapshot{Repos: demoGarden(now), FetchedAt: now, Note: "demo garden", Demo: true}, nil
		}
		m = live.New(live.Config{Load: load, DecayDays: 45, Profile: pixel.TrueColor})
```

with

```go
		world := newDemoWorld(time.Now())
		load := func(context.Context, func(live.Progress)) (live.Snapshot, error) {
			return world.snapshot(time.Now()), nil
		}
		m = live.New(live.Config{Load: load, Refresh: demoRefresh, DecayDays: 45, Profile: pixel.TrueColor})
```

In `README.md`, directly before `## Layout`, add:

```markdown
## When something happens

On every refresh GAG compares your repos with the last load, and the garden
reacts:
- a push waters the plant (a drone does it when an AI agent made the commit)
- a merge bursts a bud and sends a butterfly off
- a release sparkles and brings bees
- a new issue sprouts a weed, and a closed one pulls it
- a failing build rolls a storm in, and a fixed one clears it with a rainbow

The ticker says what happened, and when the garden pans the camera goes to
the plant. `gag garden -demo` acts all of this out every 30 seconds.

```

- [ ] **Step 5: Run everything, including the browser build**

Run: `gofmt -l . ; go vet ./... && go test ./... && go test -race ./internal/live/`
Expected: PASS.

Then build the browser demo, as `web/build.sh` does minus the npm step:

Run: `WORK=$(mktemp -d) && cp -R "$(go list -m -f '{{.Dir}}' github.com/charmbracelet/bubbletea)" "$WORK/bubbletea" && chmod -R u+w "$WORK/bubbletea" && cp web/_bubbletea/*.go "$WORK/bubbletea/" && cp go.mod go.sum "$WORK/" && go mod edit -replace "github.com/charmbracelet/bubbletea=$WORK/bubbletea" "$WORK/go.mod" && GOOS=js GOARCH=wasm go vet -modfile="$WORK/go.mod" ./cmd/gag/ && GOOS=js GOARCH=wasm go build -modfile="$WORK/go.mod" -trimpath -ldflags "-s -w" -o "$WORK/gag.wasm" ./cmd/gag && ls -lh "$WORK/gag.wasm"; rm -rf "$WORK"`
Expected: vet prints nothing, and the build writes `gag.wasm` of about 7.4 MB (as for v0.6), well under 10 MB.

Also check that `grep -n 'internal/github' cmd/gag/demo.go` prints nothing.

- [ ] **Step 6: Commit**

```bash
git add cmd/gag README.md
git commit -m "The demo acts out every change every 30 s, in the terminal and the browser

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Real-world check and the v0.7.0 release gate

**Files:**
- None, unless the check turns up bugs.

- [ ] **Step 1: The demo, by hand**

Run: `go build -o gag ./cmd/gag && GAG_COLOR=truecolor ./gag garden --once -demo | head -3`
Expected: one still frame (the one-shot demo never acts).

- [ ] **Step 2: Hand it to the user**

Ask the user to run `./gag garden -demo` and watch for a minute or two. Pressing `r` plays the next change at once. They should see, in the cycle's order:
- a watering can, and every other push a drone
- a bud bursting, with a butterfly
- sparkles and bees
- a weed coming up and one being pulled
- a storm rolling in, and a rainbow

Each should come with its ticker note. Then run `./gag` on the real garden and push something. Fix anything they report, each with a failing test first.

- [ ] **Step 3: Release gate**

Merging to `main` and tagging publish a public release. Do this only on the user's explicit yes:

```bash
git tag -a v0.7.0 -m "v0.7.0: the garden reacts"
git push origin main v0.7.0
gh run watch "$(gh run list -R RursusAeternum/GitAGarden -w release -L 1 --json databaseId --jq '.[0].databaseId')" -R RursusAeternum/GitAGarden --exit-status
gh workflow run update-gag.yml -R RursusAeternum/homebrew-tap
```
