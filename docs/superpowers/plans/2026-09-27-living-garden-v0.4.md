# Living Garden v0.4 "Signals" Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the garden say what needs attention:
- open PRs as buds that droop after a week
- a storm over plants whose CI is failing
- bright shoots on plants gaining momentum
- flowers only for the last 30 days of merges (older ones go to seed)
- a snail when issues pile up
- a ticker ordered by urgency
- a progress bar while the garden loads

**Architecture:** The data comes first. `internal/github` fetches open PRs and the default branch's CI state, and `Sync` reports progress per repo. The plant grid stays deterministic: every cell now remembers the time of the event that grew it, so ages (seed heads, fresh shoots) are decided when the plant is painted. Buds are painted from a per-frame list of PR open times. `internal/scene` adds per-plant weather and a snail. `internal/live` turns repo signals into plots (`live.Plot`, shared by the live view and `--once`), orders the ticker by attention, and streams load progress into a loading screen. `cmd/gag` computes the signals from GitHub data and the demo garden shows them all.

**Tech Stack:** Go 1.26, Bubble Tea v1.3, lipgloss, go-runewidth, `net/http/httptest` (tests only).

**Spec:** `docs/superpowers/specs/2026-09-27-living-garden-design.md`. This plan covers phase 3, "v0.4 Signals", on top of v0.3.0 as shipped. The "animation on change" column of the spec's signals table (storm rolling in, rainbow, weed pulled…) is v0.5.

**Beyond the spec, by request or carried over (deliberate):**
- **Loading progress bar** (requested by the user). The first load shows a bar with the repo being fetched. Refreshes show a compact bar in the ticker. `--once` draws a bar on stderr when stderr is a terminal.
- **v0.3 review leftovers.** Waking the machine triggers a refresh, freshness uses the wall clock, `-simulate` is labelled in the live view, and the README gaps are filled.

**Rulings on points the spec leaves open:**
- **The ticker uses ⚡ for failing CI, not the spec's ⛈.** U+26C8 has no default emoji presentation, so terminals disagree on its width (1 or 2 cells), which would wrap the ticker. Every ticker icon must be a default-emoji, two-cell glyph: ⚡ 🌷 🥀 🐌 🌱.
- **Draft PRs are neither buds nor "waiting".** A draft isn't waiting on anyone.
- **Finished repos show no signals.** No buds, weather or snail under glass.
- **Signal ages follow the simulated clock; signal counts follow the load.** Flowers and buds age by the frame's (possibly `-simulate`d) time. "Issues this week" and momentum are counted at load time against the wall clock.
- **Momentum is "rising" when a repo has more commits in the last 14 days than in the 14 before.** Rising plants paint growth younger than 14 days as bright shoots. Plants that aren't rising show none.

## Global Constraints

- Module `github.com/RursusAeternum/GitAGarden`, `go 1.26.0`. Go is at `~/sdk/go/bin`: run `export PATH=$HOME/sdk/go/bin:$PATH` before any `go` command.
- No new third-party modules.
- `garden.Grow` stays deterministic: the same name, species and events give the same grid, including each cell's `At`. Existing plants' shapes must not change: the golden frame `internal/scene/testdata/garden.golden` must pass unchanged in every task.
- `scene.Draw` stays a pure function of its `View`. Rain and snail positions come from `View.Now`.
- The live view writes nothing to stdout or stderr outside Bubble Tea's `View`. Progress goes through messages, never prints.
- Frame budget: a 240×65-cell live frame renders in under 10 ms. Frame rate is fast (125 ms) while critters fly, a storm rains, or the camera slides, and slow (500 ms) otherwise.
- Ticker icons are two-cell default-emoji glyphs (see rulings). Every ticker line is exactly the window's width.
- After every task, `go vet ./... && go test ./...` passes and `gofmt -l .` prints nothing.
- Every commit message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Repos with no CI, or unusual check states** (`ERROR`, `EXPECTED`, or a state GitHub adds later): no checks and unknown states mean clear skies, `ERROR` counts as failing, and `EXPECTED` as running. Pinned by Task 6 `TestCIForMapsGitHubStates`.
2. **Caches written by v0.3** (no PR or CI fields), and repos served from cache within the TTL: old caches load and show no signals until the next fetch, and cached repos keep their signals across refreshes. Pinned by Task 1 `TestOldCachesLoadWithoutSignals` and `TestSyncKeepsSignalsFromCache`.
3. **Many open PRs** (a dependabot flood of 40) or a seedling with little room: at most 5 buds, fewer when there's no room, and no panic, while the ticker still counts every PR. Pinned by Task 3 `TestBudsAreCappedAndFitSmallPlants` and Task 5 `TestItemsFollowTheAttentionOrder`.
4. **A slow first load** (20 s+ on a poor connection): the progress bar moves repo by repo and never looks hung, and a failing load still ends on the error screen. Pinned by Task 2 `TestLoadStreamsProgressThenGarden` and the existing `TestFirstLoadFailureOffersRetry`.
5. **Waking the laptop after hours:** the garden refreshes right away, and "updated … ago" is measured on the wall clock. Pinned by Task 7 `TestWakingUpRefreshes`.

---

### Task 1: GitHub: open PRs and CI state

**Files:**
- Modify: `internal/github/client.go` (the endpoint becomes a `Client` field, so tests can use a fake server), `internal/github/repo.go` (`OpenPR`, new `Repo` fields), `internal/github/sync.go` (`branchStatus`, fetching open PRs, keeping signals for cached repos)
- Create: `internal/github/sync_test.go`

**Interfaces:**
- Consumes: the existing `Client.query`, `connection[T]`, `Repo`, `Store`, `Sync(ctx, c, s, metas, ttl, log)`.
- Produces:
  - `type OpenPR struct { Number int; Title string; CreatedAt time.Time; Draft bool }`
  - New `Repo` fields: `OpenPRs []OpenPR`, `Branch string`, `CI string` (a `statusCheckRollup` state: `SUCCESS`, `FAILURE`, `ERROR`, `PENDING`, `EXPECTED`, or `""` for no checks)
  - `Client` gains an unexported `url` field. `NewClient` sets it to the GitHub endpoint; tests point it at `httptest` servers.

- [ ] **Step 1: Write the failing tests**

`internal/github/sync_test.go`:

```go
package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeGitHub answers GraphQL queries with whatever answer returns for the
// query text.
func fakeGitHub(t *testing.T, answer func(query string) string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Query string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		io.WriteString(w, answer(body.Query))
	}))
	t.Cleanup(srv.Close)
	return &Client{token: "test", hc: srv.Client(), url: srv.URL}
}

const emptyConn = `{"data":{"repository":{"conn":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}`

func TestFetchReadsOpenPRsAndCI(t *testing.T) {
	c := fakeGitHub(t, func(q string) string {
		switch {
		case strings.Contains(q, "statusCheckRollup"):
			return `{"data":{"repository":{"defaultBranchRef":{"name":"main","target":{"statusCheckRollup":{"state":"FAILURE"}}}}}}`
		case strings.Contains(q, "history("):
			return `{"data":{"repository":{"defaultBranchRef":null}}}`
		case strings.Contains(q, "states:OPEN"):
			return `{"data":{"repository":{"conn":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[
				{"number":7,"title":"Add bees","createdAt":"2026-09-01T10:00:00Z","isDraft":false},
				{"number":8,"title":"WIP","createdAt":"2026-09-20T10:00:00Z","isDraft":true}]}}}}`
		}
		return emptyConn
	})
	r := &Repo{NameWithOwner: "me/garden"}
	if err := c.fetch(context.Background(), r, nil); err != nil {
		t.Fatal(err)
	}
	if r.Branch != "main" || r.CI != "FAILURE" {
		t.Errorf("branch %q, ci %q; want main, FAILURE", r.Branch, r.CI)
	}
	if len(r.OpenPRs) != 2 || r.OpenPRs[0].Number != 7 || r.OpenPRs[0].Draft || !r.OpenPRs[1].Draft {
		t.Errorf("open PRs = %+v", r.OpenPRs)
	}
}

func TestFetchWithoutChecksHasNoCI(t *testing.T) {
	c := fakeGitHub(t, func(q string) string {
		switch {
		case strings.Contains(q, "statusCheckRollup"):
			return `{"data":{"repository":{"defaultBranchRef":{"name":"trunk","target":{"statusCheckRollup":null}}}}}`
		case strings.Contains(q, "history("):
			return `{"data":{"repository":{"defaultBranchRef":null}}}`
		}
		return emptyConn
	})
	r := &Repo{NameWithOwner: "me/quiet"}
	if err := c.fetch(context.Background(), r, nil); err != nil {
		t.Fatal(err)
	}
	if r.Branch != "trunk" || r.CI != "" || len(r.OpenPRs) != 0 {
		t.Errorf("branch %q, ci %q, open PRs %d; want trunk, no CI, none", r.Branch, r.CI, len(r.OpenPRs))
	}
}

func TestSyncKeepsSignalsFromCache(t *testing.T) {
	cached := &Repo{NameWithOwner: "me/x", FetchedAt: time.Now(), CI: "SUCCESS", Branch: "main",
		OpenPRs: []OpenPR{{Number: 1, CreatedAt: time.Now()}}}
	s := &Store{Repos: map[string]*Repo{"me/x": cached}}
	repos, err := Sync(context.Background(), nil, s, []*Repo{{NameWithOwner: "me/x"}}, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r := repos[0]; r.CI != "SUCCESS" || r.Branch != "main" || len(r.OpenPRs) != 1 {
		t.Errorf("cached signals lost: ci %q, branch %q, open PRs %d", r.CI, r.Branch, len(r.OpenPRs))
	}
}

func TestOldCachesLoadWithoutSignals(t *testing.T) {
	old := `{"repos":{"me/x":{"nameWithOwner":"me/x","commits":[],"fetchedAt":"2026-09-01T00:00:00Z"}}}`
	var s Store
	if err := json.Unmarshal([]byte(old), &s); err != nil {
		t.Fatal(err)
	}
	if r := s.Repos["me/x"]; r.CI != "" || r.Branch != "" || r.OpenPRs != nil {
		t.Errorf("a v0.3 cache should load with no signals, got %+v", r)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/github/`
Expected: FAIL (build errors: `unknown field url`, `r.Branch undefined`, `undefined: OpenPR`).

- [ ] **Step 3: Make the endpoint a field**

In `internal/github/client.go`:

1. Change the struct to:

```go
type Client struct {
	token string
	hc    *http.Client
	url   string // GraphQL endpoint; tests point it at a fake server
}
```

2. In `NewClient`, change `return &Client{token: tok, hc: &http.Client{Timeout: 90 * time.Second}}, nil` to `return &Client{token: tok, hc: &http.Client{Timeout: 90 * time.Second}, url: endpoint}, nil`.
3. In `query`, change `http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))` to `http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))`.

- [ ] **Step 4: Add the repo fields**

In `internal/github/repo.go`, after the `Release` type, add:

```go
// OpenPR is a pull request that is neither merged nor closed.
type OpenPR struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	Draft     bool      `json:"draft,omitempty"`
}
```

and in `type Repo struct`, insert before `FetchedAt     time.Time `json:"fetchedAt"``:

```go
	OpenPRs       []OpenPR  `json:"openPRs,omitempty"`
	Branch        string    `json:"branch,omitempty"` // the default branch
	CI            string    `json:"ci,omitempty"`     // statusCheckRollup state of its latest commit
```

- [ ] **Step 5: Fetch open PRs and CI, keep them for cached repos**

In `internal/github/sync.go`:

1. Append the method:

```go
// branchStatus returns the default branch's name and the combined CI state
// of its latest commit: SUCCESS, FAILURE, ERROR, PENDING, EXPECTED, or ""
// when the commit has no checks or the repo is empty.
func (c *Client) branchStatus(ctx context.Context, owner, name string) (branch, state string, err error) {
	q := `query($owner:String!,$name:String!){repository(owner:$owner,name:$name){defaultBranchRef{name target{... on Commit{statusCheckRollup{state}}}}}}`
	var out struct {
		Repository struct {
			DefaultBranchRef *struct {
				Name   string
				Target struct {
					StatusCheckRollup *struct{ State string }
				}
			}
		}
	}
	if err := c.query(ctx, q, map[string]any{"owner": owner, "name": name}, &out); err != nil {
		return "", "", err
	}
	ref := out.Repository.DefaultBranchRef
	if ref == nil {
		return "", "", nil
	}
	if ref.Target.StatusCheckRollup != nil {
		state = ref.Target.StatusCheckRollup.State
	}
	return ref.Name, state, nil
}
```

2. In `fetch`, replace

```go
	r.FetchedAt = time.Now()
	return nil
}
```

with

```go
	type openPRNode struct {
		Number    int
		Title     string
		CreatedAt time.Time
		IsDraft   bool
	}
	open, err := connection[openPRNode](ctx, c, owner, name, "pullRequests", ",states:OPEN", "number title createdAt isDraft")
	if err != nil {
		return fmt.Errorf("open pull requests: %w", err)
	}
	branch, ci, err := c.branchStatus(ctx, owner, name)
	if err != nil {
		return fmt.Errorf("ci status: %w", err)
	}
	r.OpenPRs, r.Branch, r.CI = nil, branch, ci
	for _, n := range open {
		r.OpenPRs = append(r.OpenPRs, OpenPR{Number: n.Number, Title: n.Title, CreatedAt: n.CreatedAt, Draft: n.IsDraft})
	}
	r.FetchedAt = time.Now()
	return nil
}
```

3. In `Sync`, replace

```go
			m.Commits, m.PRs, m.Issues, m.Releases, m.FetchedAt = cached.Commits, cached.PRs, cached.Issues, cached.Releases, cached.FetchedAt
```

with

```go
			m.Commits, m.PRs, m.Issues, m.Releases, m.FetchedAt = cached.Commits, cached.PRs, cached.Issues, cached.Releases, cached.FetchedAt
			m.OpenPRs, m.Branch, m.CI = cached.OpenPRs, cached.Branch, cached.CI
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/github/ && go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/github
git commit -m "github: fetch open PRs and the default branch's CI state

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: A progress bar while the garden loads

**Files:**
- Modify: `internal/github/sync.go` (`Sync` reports progress), `internal/github/sync_test.go` (new `Sync` argument), `cmd/gag/main.go` (`loadRepos` passes progress), `cmd/gag/garden.go` (`snapshot` reports progress; `--once` draws a stderr bar), `cmd/gag/garden_test.go` (new `snapshot` argument), `internal/live/model.go` (streamed loads, loading screen, refresh bar in the ticker), `internal/live/model_test.go`
- Create: `internal/live/progress.go`, `internal/live/progress_test.go`, `cmd/gag/progress.go`, `cmd/gag/progress_test.go`

**Interfaces:**
- Consumes: Task 1's `Sync`, and v0.3's `live.Config`, `Model.load`, `loadedMsg`, `center`, `tickerLine`.
- Produces:
  - `github.Sync(ctx, c, s, metas, ttl, log func(string), progress func(done, total int, current string))`. It reports `(i, n, name)` before each repo and `(n, n, "")` at the end.
  - `cmd/gag`: `loadRepos(ctx, names, owner, limit, ttl, log, progress func(done, total int, current string))`. It reports `(0, 0, "")` before listing repos.
  - `live.Progress{Done, Total int; Current string}` and `live.Bar(done, total, width int) string`
  - `live.Config.Load` becomes `func(ctx context.Context, progress func(Progress)) (Snapshot, error)`
  - `cmd/gag`: `source.snapshot(ctx, progress func(live.Progress))` and `stderrProgress(w io.Writer) func(done, total int, current string)`

- [ ] **Step 1: Write the failing tests**

Append to `internal/github/sync_test.go`:

```go
func TestSyncReportsProgress(t *testing.T) {
	s := &Store{Repos: map[string]*Repo{}}
	var metas []*Repo
	for _, n := range []string{"me/a", "me/b"} {
		s.Repos[n] = &Repo{NameWithOwner: n, FetchedAt: time.Now()}
		metas = append(metas, &Repo{NameWithOwner: n})
	}
	var got []string
	Sync(context.Background(), nil, s, metas, time.Hour, nil, func(done, total int, current string) {
		got = append(got, fmt.Sprintf("%d/%d %s", done, total, current))
	})
	if want := "0/2 me/a|1/2 me/b|2/2 "; strings.Join(got, "|") != want {
		t.Errorf("progress = %q, want %q", strings.Join(got, "|"), want)
	}
}
```

and add `"fmt"` to that file's imports.

`internal/live/progress_test.go`:

```go
package live

import "testing"

func TestBar(t *testing.T) {
	cases := []struct {
		done, total, width int
		want               string
	}{
		{3, 8, 8, "███░░░░░"},
		{0, 0, 4, "░░░░"},
		{9, 8, 4, "████"},
		{1, 2, 0, ""},
	}
	for _, c := range cases {
		if got := Bar(c.done, c.total, c.width); got != c.want {
			t.Errorf("Bar(%d, %d, %d) = %q, want %q", c.done, c.total, c.width, got, c.want)
		}
	}
}
```

Append to `internal/live/model_test.go`:

```go
func TestLoadStreamsProgressThenGarden(t *testing.T) {
	m := New(Config{
		Load: func(_ context.Context, progress func(Progress)) (Snapshot, error) {
			progress(Progress{Done: 1, Total: 3, Current: "me/quiet"})
			return garden3(), nil
		},
		DecayDays: 45,
		Now:       func() time.Time { return t0 },
	})
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	msg := m.load(false)()
	pm, ok := msg.(progressMsg)
	if !ok {
		t.Fatalf("first message = %T, want a progress update", msg)
	}
	m, cmd := step(m, pm)
	if v := visible(m.View()); !strings.Contains(v, "1/3") || !strings.Contains(v, "fetching me/quiet") {
		t.Errorf("loading screen = %q", v)
	}
	m = finishLoad(m, cmd)
	if len(m.repos) != 3 {
		t.Errorf("garden not loaded after progress: %d plants", len(m.repos))
	}
}

func TestLoadingScreenBeforeTheListIsKnown(t *testing.T) {
	m := newModel(garden3(), nil, t0)
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if v := visible(m.View()); !strings.Contains(v, "finding your repos") {
		t.Errorf("view = %q", v)
	}
}

func TestRefreshShowsProgressInTheTicker(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	m, _ = step(m, key("r"))
	m, _ = step(m, progressMsg{p: Progress{Done: 2, Total: 8}, ch: make(chan tea.Msg)})
	lines := strings.Split(m.View(), "\n")
	if last := visible(lines[len(lines)-1]); !strings.Contains(last, "refreshing") || !strings.Contains(last, "2/8") {
		t.Errorf("ticker = %q", last)
	}
}
```

`cmd/gag/progress_test.go`:

```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestStderrProgress(t *testing.T) {
	var buf bytes.Buffer
	p := stderrProgress(&buf)
	p(0, 0, "")
	p(3, 8, "me/garden")
	p(8, 8, "")
	out := buf.String()
	for _, want := range []string{"finding your repos", "3/8", "me/garden"} {
		if !strings.Contains(out, want) {
			t.Errorf("progress output lacks %q: %q", want, out)
		}
	}
	if !strings.HasSuffix(out, "\r\x1b[K") {
		t.Errorf("the finished bar should be erased: %q", out)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/github/ ./internal/live/ ./cmd/gag/`
Expected: FAIL (build errors: too many arguments to `Sync`, `undefined: Bar`, `undefined: progressMsg`, `undefined: stderrProgress`).

- [ ] **Step 3: Report progress from `Sync`**

In `internal/github/sync.go`:

1. Change the doc comment and signature of `Sync` to:

```go
// Sync brings each repo's history up to date, skipping any fetched within
// ttl. It saves after every repo so progress survives a slow or dropped
// connection. Repos that fail to refresh fall back to their cached copy;
// the returned error reports the first failure. progress, if not nil, hears
// (i, n, name) before each repo and (n, n, "") at the end.
func Sync(ctx context.Context, c *Client, s *Store, metas []*Repo, ttl time.Duration, log func(string), progress func(done, total int, current string)) ([]*Repo, error) {
```

2. Replace `	for _, m := range metas {` in `Sync` with:

```go
	report := func(done int, current string) {
		if progress != nil {
			progress(done, len(metas), current)
		}
	}
	for i, m := range metas {
		report(i, m.NameWithOwner)
```

3. Replace the final `	return out, firstErr` of `Sync` with:

```go
	report(len(metas), "")
	return out, firstErr
```

4. In `internal/github/sync_test.go`, change the `Sync(...)` call in `TestSyncKeepsSignalsFromCache` to pass a final `nil`: `Sync(context.Background(), nil, s, []*Repo{{NameWithOwner: "me/x"}}, time.Hour, nil, nil)`.

- [ ] **Step 4: Pass progress through `loadRepos`**

In `cmd/gag/main.go`:

1. Change the signature to:

```go
func loadRepos(ctx context.Context, names []string, owner string, limit int, ttl time.Duration, log func(string), progress func(done, total int, current string)) ([]*github.Repo, bool, error) {
```

and extend its doc comment with the sentence `progress, if not nil, hears (0, 0, "") while repos are listed, then Sync's per-repo progress.`

2. Directly after `	c, err := github.NewClient()` and its `if err != nil { return nil, false, err }`, add:

```go
	if progress != nil {
		progress(0, 0, "") // listing repos; the total isn't known yet
	}
```

3. Change `repos, err := github.Sync(ctx, c, store, metas, ttl, log)` to `repos, err := github.Sync(ctx, c, store, metas, ttl, log, progress)`.
4. In `runReplay`, change `loadRepos(context.Background(), []string{*repo}, "", 1, *ttl, logf)` to `loadRepos(context.Background(), []string{*repo}, "", 1, *ttl, logf, nil)`.

- [ ] **Step 5: The progress type and bar**

`internal/live/progress.go`:

```go
package live

import "strings"

// Progress is how far a load has come.
type Progress struct {
	Done, Total int    // repos fetched so far, out of Total (0 while repos are still being listed)
	Current     string // the repo being fetched now
}

// Bar draws a progress bar width cells wide.
func Bar(done, total, width int) string {
	if width <= 0 {
		return ""
	}
	filled := 0
	if total > 0 {
		filled = min(width, width*done/total)
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
```

- [ ] **Step 6: Stream loads in the model**

In `internal/live/model.go`:

1. Add `"fmt"` to the imports.
2. In `Config`, change the `Load` field to:

```go
	Load      func(ctx context.Context, progress func(Progress)) (Snapshot, error)
```

3. In `Model`, add the field `progress Progress // how far the current load has come`.
4. Replace the message type block with:

```go
type (
	tickMsg   struct{}
	loadedMsg struct {
		snap Snapshot
		err  error
	}
	refreshMsg  struct{ gen int }
	progressMsg struct {
		p  Progress
		ch chan tea.Msg // where the rest of this load's messages arrive
	}
)
```

5. Replace the `load` method with:

```go
// load runs Load in the background. Its progress updates and final result
// arrive as messages on one channel: each progressMsg carries the channel so
// Update can wait for the next message, and loadedMsg ends the stream.
func (m Model) load(force bool) tea.Cmd {
	load := m.cfg.Load
	return func() tea.Msg {
		ctx := context.Background()
		if force {
			ctx = context.WithValue(ctx, forceKey{}, true)
		}
		ch := make(chan tea.Msg, 8)
		go func() {
			s, err := load(ctx, func(p Progress) {
				select {
				case ch <- progressMsg{p: p, ch: ch}:
				default: // the screen is behind; a later update catches up
				}
			})
			ch <- loadedMsg{s, err}
		}()
		return <-ch
	}
}

// next waits for a load's next message.
func next(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}
```

6. In `Update`, add a case before `case loadedMsg:`:

```go
	case progressMsg:
		m.progress = msg.p
		return m, next(msg.ch)
```

and in `case loadedMsg:`, add `m.progress = Progress{}` right after `m.loading = false`.

7. In `View`, replace `		return center(m.cols, m.rows, "🌱 growing your garden…")` with `		return m.loadingView()`, and add:

```go
// loadingView is the first-load screen: a progress bar and the repo being
// fetched, so a slow connection doesn't look like a hang.
func (m Model) loadingView() string {
	p := m.progress
	text := "🌱 growing your garden\n\nfinding your repos…"
	if p.Total > 0 {
		width := max(4, min(30, m.cols-12))
		text = fmt.Sprintf("🌱 growing your garden\n\n%s %d/%d", Bar(p.Done, p.Total, width), p.Done, p.Total)
		if p.Current != "" {
			text += "\nfetching " + p.Current
		}
	}
	return center(m.cols, m.rows, text)
}
```

8. In `tickerLine`, right after `	status := Status(m.snap, m.cfg.Now(), m.loading, m.loadErr != nil)`, add:

```go
	if m.loading && m.progress.Total > 0 {
		status = fmt.Sprintf("refreshing %s %d/%d", Bar(m.progress.Done, m.progress.Total, 8), m.progress.Done, m.progress.Total)
	}
```

- [ ] **Step 7: Update the model tests for the new `Load`**

In `internal/live/model_test.go`:

1. Replace every `func(context.Context) (Snapshot, error)` with `func(context.Context, func(Progress)) (Snapshot, error)`, and every `func(ctx context.Context) (Snapshot, error)` with `func(ctx context.Context, _ func(Progress)) (Snapshot, error)`.
2. Replace the `ready` helper with:

```go
// ready sizes the window and runs the first load to completion.
func ready(m Model, cols, rows int) Model {
	m, _ = step(m, tea.WindowSizeMsg{Width: cols, Height: rows})
	return finishLoad(m, m.load(false))
}

// finishLoad runs a load command and feeds its messages back until the
// load is done.
func finishLoad(m Model, cmd tea.Cmd) Model {
	for cmd != nil {
		msg := cmd()
		var next tea.Cmd
		m, next = step(m, msg)
		if _, done := msg.(loadedMsg); done {
			return m
		}
		cmd = next
	}
	return m
}
```

- [ ] **Step 8: Report progress from `cmd/gag`**

`cmd/gag/progress.go`:

```go
package main

import (
	"fmt"
	"io"

	"github.com/RursusAeternum/GitAGarden/internal/live"
)

// stderrProgress draws a one-line progress bar on w (stderr) while --once
// loads, and erases it when the load is done.
func stderrProgress(w io.Writer) func(done, total int, current string) {
	return func(done, total int, current string) {
		switch {
		case total == 0:
			fmt.Fprint(w, "\r\x1b[K🌱 finding your repos…")
		case done >= total:
			fmt.Fprint(w, "\r\x1b[K")
		default:
			fmt.Fprintf(w, "\r\x1b[K🌱 %s %d/%d  %s", live.Bar(done, total, 20), done, total, current)
		}
	}
}
```

In `cmd/gag/garden.go`:

1. Change `func (s source) snapshot(ctx context.Context) (live.Snapshot, error) {` to `func (s source) snapshot(ctx context.Context, progress func(live.Progress)) (live.Snapshot, error) {`.
2. In `snapshot`, replace `	repos, offline, err := loadRepos(ctx, s.names, s.owner, s.limit, ttl, nil)` with:

```go
	var report func(done, total int, current string)
	if progress != nil {
		report = func(done, total int, current string) {
			progress(live.Progress{Done: done, Total: total, Current: current})
		}
	}
	repos, offline, err := loadRepos(ctx, s.names, s.owner, s.limit, ttl, nil, report)
```

3. In `printOnce`, replace `		repos, offline, err := loadRepos(context.Background(), src.names, src.owner, src.limit, src.ttl, logf)` with:

```go
		log, progress := logf, (func(done, total int, current string))(nil)
		if term.IsTerminal(int(os.Stderr.Fd())) {
			log, progress = nil, stderrProgress(os.Stderr) // a bar instead of "fetching …" lines
		}
		repos, offline, err := loadRepos(context.Background(), src.names, src.owner, src.limit, src.ttl, log, progress)
```

4. In `cmd/gag/garden_test.go`, replace every `.snapshot(context.Background())` with `.snapshot(context.Background(), nil)`.

- [ ] **Step 9: Run the tests**

Run: `gofmt -l . ; go vet ./... && go test ./...`
Expected: PASS, including the four new tests and all earlier live tests with the updated helper.

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "Show a progress bar while the garden loads

The first load shows a bar with the repo being fetched, refreshes show a
compact bar in the ticker, and --once draws one on stderr.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Garden: flowers go to seed, buds for PRs, shoots for momentum

**Files:**
- Modify: `internal/garden/plant.go` (cell birth times, `Blooming`, bud spots, age constants), `internal/garden/growers.go` (cells carry their birth time; a cactus carries a displaced bloom's time), `internal/garden/paint.go` (`Style.Now/Buds/Rising`; seed heads, buds, shoots)
- Test: `internal/garden/paint_test.go` (append)

**Interfaces:**
- Consumes: v0.3's `Grow`, `Paint`, `Style`, and the test helpers `pushes(n)`, `t0` and `paintAlone(p, st)`.
- Produces:
  - `Cell.At int64` (Unix seconds of the event that grew or last thickened the cell; 0 for the seedling)
  - `garden.MaxBuds = 5`
  - `func (p *Plant) Blooming(now time.Time) int` (flowers not yet gone to seed; a zero `now` counts all)
  - `Style.Now time.Time`, `Style.Buds []time.Time`, `Style.Rising bool`
  - Unexported colors `seedColor`, `budColor` and `paleBud`, used by the tests.

- [ ] **Step 1: Write the failing tests**

Append to `internal/garden/paint_test.go`:

```go
// tinted reports whether px is col under Paint's ±10% texture.
func tinted(px, col pixel.RGB) bool {
	if col.R == 0 {
		return false
	}
	f := float64(px.R) / float64(col.R)
	near := func(a, b uint8) bool { d := float64(a) - float64(b)*f; return d > -4 && d < 4 }
	return f > 0.88 && f < 1.12 && near(px.G, col.G) && near(px.B, col.B)
}

func count(c *pixel.Canvas, match func(pixel.RGB) bool) int {
	n := 0
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if match(c.At(x, y)) {
				n++
			}
		}
	}
	return n
}

func TestFlowersGoToSeedAfterThirtyDays(t *testing.T) {
	merged := t0.Add(40 * time.Hour)
	p := Grow("r", Shrub, append(pushes(30), Event{Kind: Merge, At: merged}))
	if n := p.Blooming(merged.Add(10 * 24 * time.Hour)); n != 1 {
		t.Errorf("blooming 10 days after the merge = %d, want 1", n)
	}
	if n := p.Blooming(merged.Add(31 * 24 * time.Hour)); n != 0 {
		t.Errorf("blooming 31 days after the merge = %d, want 0", n)
	}
	if n := p.Blooming(time.Time{}); n != 1 {
		t.Errorf("a zero time should count every flower, got %d", n)
	}
	isSeed := func(px pixel.RGB) bool { return tinted(px, seedColor) }
	fresh := paintAlone(p, Style{Health: 1, Now: merged.Add(24 * time.Hour)})
	seeded := paintAlone(p, Style{Health: 1, Now: merged.Add(40 * 24 * time.Hour)})
	if count(fresh, isSeed) != 0 || count(seeded, isSeed) == 0 {
		t.Errorf("seed heads: fresh %d, after 40 days %d", count(fresh, isSeed), count(seeded, isSeed))
	}
}

func TestBudsForOpenPRs(t *testing.T) {
	p := Grow("r", Shrub, pushes(80))
	now := t0.Add(100 * time.Hour)
	is := func(col pixel.RGB) func(pixel.RGB) bool { return func(px pixel.RGB) bool { return px == col } }
	fresh := paintAlone(p, Style{Health: 1, Now: now, Buds: []time.Time{now.Add(-time.Hour), now.Add(-2 * time.Hour)}})
	if n := count(fresh, is(budColor)); n != 2 {
		t.Errorf("fresh buds = %d, want 2", n)
	}
	old := paintAlone(p, Style{Health: 1, Now: now, Buds: []time.Time{now.Add(-10 * 24 * time.Hour)}})
	if count(old, is(budColor)) != 0 || count(old, is(paleBud)) != 1 {
		t.Errorf("a PR waiting 10 days should droop: bright %d, pale %d", count(old, is(budColor)), count(old, is(paleBud)))
	}
}

func TestBudsAreCappedAndFitSmallPlants(t *testing.T) {
	many := make([]time.Time, 40)
	for i := range many {
		many[i] = t0
	}
	is := func(px pixel.RGB) bool { return px == budColor }
	big := paintAlone(Grow("r", Shrub, pushes(200)), Style{Health: 1, Now: t0, Buds: many})
	if n := count(big, is); n != MaxBuds {
		t.Errorf("buds for 40 PRs = %d, want %d", n, MaxBuds)
	}
	tiny := paintAlone(Grow("r", Shrub, nil), Style{Health: 1, Now: t0, Buds: many})
	if n := count(tiny, is); n == 0 || n > MaxBuds {
		t.Errorf("a seedling should show a few buds, got %d", n)
	}
}

func TestRisingPlantsShowFreshShoots(t *testing.T) {
	p := Grow("r", Shrub, pushes(60))
	now := t0.Add(61 * time.Hour) // all of this plant's growth is recent
	calm := paintAlone(p, Style{Health: 1, Now: now}).Encode(pixel.TrueColor)
	rising := paintAlone(p, Style{Health: 1, Now: now, Rising: true}).Encode(pixel.TrueColor)
	if calm == rising {
		t.Error("a rising plant should show fresh shoots")
	}
	later := now.Add(60 * 24 * time.Hour) // nothing is recent any more
	if paintAlone(p, Style{Health: 1, Now: later, Rising: true}).Encode(pixel.TrueColor) !=
		paintAlone(p, Style{Health: 1, Now: later}).Encode(pixel.TrueColor) {
		t.Error("old growth should not show as shoots")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/garden/`
Expected: FAIL (build errors: `unknown field Now in struct literal of type Style`, `p.Blooming undefined`, `undefined: seedColor`).

- [ ] **Step 3: Give cells a birth time**

In `internal/garden/plant.go`:

1. Add `"sort"` to the imports.
2. Replace `type Cell struct { ... }` with:

```go
type Cell struct {
	Kind  CellKind
	Level uint8 // how lush a leaf is; grows once there is no room for new ones
	At    int64 // Unix seconds of the event that grew or last thickened it; 0 for the seedling
}
```

3. In `type Plant struct`, add the field `stamp int64 // the time of the event being replayed; new cells get it` after `weedSlots []int`.
4. In `Grow`, make the first statement inside `for _, e := range events {` be `		p.stamp = e.At.Unix()`.
5. Replace `func (p *Plant) set(...)` with:

```go
func (p *Plant) set(x, y int, k CellKind) {
	p.Grid[y][x] = p.cell(k)
}

// cell is a new cell of kind k, born at the event being replayed.
func (p *Plant) cell(k CellKind) Cell { return Cell{Kind: k, At: p.stamp} }
```

6. Append:

```go
// How signals age: flowers go to seed flowerLife after their merge, a PR's
// bud droops once it has waited budDroop, and on a rising plant growth
// younger than shootAge shows as fresh shoots. At most MaxBuds buds are shown.
const (
	flowerLife = 30 * 24 * time.Hour
	budDroop   = 7 * 24 * time.Hour
	shootAge   = 14 * 24 * time.Hour
	MaxBuds    = 5
)

// Blooming is how many of the plant's flowers are still in bloom at now. A
// zero now counts them all.
func (p *Plant) Blooming(now time.Time) int {
	n := 0
	for _, q := range p.cellsOf(Flower) {
		if !seedHead(p.Grid[q.y][q.x], now) {
			n++
		}
	}
	return n
}

// seedHead reports whether a flower has gone to seed by now.
func seedHead(c Cell, now time.Time) bool {
	return !now.IsZero() && c.At != 0 && now.Sub(time.Unix(c.At, 0)) > flowerLife
}

// fresh reports whether a cell grew less than shootAge before now.
func fresh(c Cell, now time.Time) bool {
	return !now.IsZero() && c.At != 0 && now.Sub(time.Unix(c.At, 0)) <= shootAge
}

// budSpots returns up to n free cells for buds: just above the plant's
// highest stems, leaves and flesh, never touching each other. The same
// plant always gives the same spots.
func (p *Plant) budSpots(n int) []pt {
	var cands []pt
	for y := 1; y < ground; y++ {
		for x := 0; x < Width; x++ {
			if !p.free(x, y) {
				continue
			}
			if below := p.Grid[y+1][x].Kind; below == Stem || below == Leaf || below == Body {
				cands = append(cands, pt{x, y})
			}
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].y != cands[j].y {
			return cands[i].y < cands[j].y
		}
		return hash01(p.Name, cands[i].x, -3) < hash01(p.Name, cands[j].x, -3)
	})
	var out []pt
	for _, c := range cands {
		if len(out) == n {
			break
		}
		touching := false
		for _, o := range out {
			if max(o.x-c.x, c.x-o.x) <= 1 && max(o.y-c.y, c.y-o.y) <= 1 {
				touching = true
				break
			}
		}
		if !touching {
			out = append(out, c)
		}
	}
	return out
}
```

- [ ] **Step 4: Let growers carry birth times**

In `internal/garden/growers.go`:

1. Replace the whole `sprout` function with:

```go
// sprout places a new cell of kind k in a free spot next to one of the
// anchors and says where. The ground row is left to the stem base and weeds.
func (b base) sprout(anchors []pt, k CellKind, upOnly bool) (pt, bool) {
	return b.sproutCell(anchors, b.p.cell(k), upOnly)
}

// sproutCell is sprout for a ready-made cell, so a moved bloom keeps its
// birth time.
func (b base) sproutCell(anchors []pt, cell Cell, upOnly bool) (pt, bool) {
	try := func(a, d pt) (pt, bool) {
		x, y := a.x+d.x, a.y+d.y
		if (upOnly && d.y > 0) || y >= ground || !b.p.free(x, y) {
			return pt{}, false
		}
		b.p.Grid[y][x] = cell
		return pt{x, y}, true
	}
	if len(anchors) == 0 {
		return pt{}, false
	}
	for i := 0; i < 24; i++ {
		if q, ok := try(anchors[b.r.Intn(len(anchors))], dirs8[b.r.Intn(len(dirs8))]); ok {
			return q, true
		}
	}
	for _, i := range b.r.Perm(len(anchors)) {
		for _, d := range dirs8 {
			if q, ok := try(anchors[i], d); ok {
				return q, true
			}
		}
	}
	return pt{}, false
}
```

2. Replace the whole `bloom` function with:

```go
// bloom places a new flower or fruit near the anchors.
func (b base) bloom(anchors []pt, k CellKind) { b.bloomCell(anchors, b.p.cell(k)) }

// bloomCell places cell near the anchors, falling back to anywhere on the
// plant, and finally to taking an existing leaf's place.
func (b base) bloomCell(anchors []pt, cell Cell) {
	if _, ok := b.sproutCell(anchors, cell, true); ok {
		return
	}
	if _, ok := b.sproutCell(b.p.cellsOf(Stem, Body, Leaf), cell, false); ok {
		return
	}
	if leaves := b.p.cellsOf(Leaf); len(leaves) > 0 {
		c := leaves[b.r.Intn(len(leaves))]
		b.p.Grid[c.y][c.x] = cell
	}
}
```

3. In `thicken`, replace `			c.Level++` with:

```go
			c.Level++
			c.At = b.p.stamp // lusher is new growth too
```

4. Replace the cactus `grow` method with:

```go
// grow turns x,y into flesh. A flower or fruit there is carried up onto the
// new growth, keeping its birth time, so merges crown the cactus instead of
// capping it.
func (c *cactus) grow(x, y int) {
	old := c.p.Grid[y][x]
	c.p.set(x, y, Body)
	if old.Kind == Flower || old.Kind == Fruit {
		c.bloomCell([]pt{{x, y}}, old)
	}
}
```

- [ ] **Step 5: Paint seed heads, buds and shoots**

In `internal/garden/paint.go`:

1. Add `"time"` to the imports.
2. Replace `type Style struct { ... }` with:

```go
// Style is how a plant is painted in one frame.
type Style struct {
	Health float64     // 1 fresh … 0 fully wilted
	Sway   float64     // horizontal offset of the plant's top row, in pixels
	Now    time.Time   // the frame's time; ages flowers and buds (zero: all fresh)
	Buds   []time.Time // when each open PR was opened; up to MaxBuds are drawn
	Rising bool        // more commits lately than before: recent growth shows as shoots
}
```

3. In the `var (...)` color block, add:

```go
	seedColor    = rgbOf(214, 196, 150)
	shootColor   = rgbOf(185, 245, 105)
	budColor     = rgbOf(235, 110, 150)
	paleBud      = rgbOf(215, 185, 175)
```

4. Add the helper:

```go
// swayAt is how far row y is shifted by the plant's sway.
func swayAt(st Style, y int) int {
	return int(math.Round(st.Sway * float64(ground-y) / float64(Height)))
}
```

5. In `Paint`, replace `		dx := int(math.Round(st.Sway * float64(ground-y) / float64(Height)))` with `		dx := swayAt(st, y)`.
6. In `Paint`, replace the flower case

```go
			case Flower:
				col = pal.flower
				if h < 0.5 {
					col = pixel.Lerp(col, deadColor, 0.6)
				}
```

with

```go
			case Flower:
				col = pal.flower
				if seedHead(cell, st.Now) {
					col = seedColor
				}
				if h < 0.5 {
					col = pixel.Lerp(col, deadColor, 0.6)
				}
```

7. In `Paint`, directly after the `switch cell.Kind { ... }` block closes and before `c.Set(baseX-center+x+dx, …)`, add:

```go
			if st.Rising && (cell.Kind == Stem || cell.Kind == Leaf || cell.Kind == Body) && fresh(cell, st.Now) {
				col = pixel.Lerp(col, shootColor, 0.55)
			}
```

8. At the end of `Paint`, after the weeds loop, add:

```go
	for i, s := range p.budSpots(min(len(st.Buds), MaxBuds)) {
		col, y := budColor, s.y
		if !st.Now.IsZero() && st.Now.Sub(st.Buds[i]) > budDroop {
			col, y = paleBud, s.y+1 // waited too long: it droops and fades
		}
		c.Set(baseX-center+s.x+swayAt(st, y), baseY-ground+y, col)
	}
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/garden/ && go test ./...`
Expected: PASS. The golden frame is unchanged, because its plots have no `Now`, buds or rising, and growth consumes the random source exactly as before.

- [ ] **Step 7: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/garden
git commit -m "garden: flowers go to seed, buds for PRs, fresh shoots when rising

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Scene: weather over each plant, and the snail

**Files:**
- Create: `internal/scene/weather.go`, `internal/scene/weather_test.go`
- Modify: `internal/scene/compose.go` (`Plot.Weather` and `Plot.Snail`; `drawPlot` draws them), `internal/scene/critters.go` (`Flowering` ignores seed heads)

**Interfaces:**
- Consumes: Task 3's `Plant.Blooming` and `Style.Now`, and the test helpers `demoPlots()`, `at()` and `painted()`.
- Produces:
  - `type Weather int` with `const Clear, Cloudy, Storm`
  - `Plot.Weather Weather`, `Plot.Snail bool`
  - Unexported colors `stormCloud`, `greyCloud`, `rainColor`, `shellColor` (used by the tests)

- [ ] **Step 1: Write the failing tests**

`internal/scene/weather_test.go`:

```go
package scene

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func frameOf(pl Plot, now time.Time) *pixel.Canvas {
	return Draw(View{Cols: BedCols, Plots: []Plot{pl}, Now: now, Seed: 1})
}

func colored(c *pixel.Canvas, col pixel.RGB) int {
	n := 0
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if c.At(x, y) == col {
				n++
			}
		}
	}
	return n
}

func TestStormRainsOnThePlant(t *testing.T) {
	pl := demoPlots()[0]
	if colored(frameOf(pl, at(12, 0)), stormCloud) != 0 {
		t.Fatal("clear weather should have no storm cloud")
	}
	pl.Weather = Storm
	now := at(12, 0)
	storm := frameOf(pl, now)
	if colored(storm, stormCloud) == 0 {
		t.Error("no storm cloud over a failing plant")
	}
	if storm.Encode(pixel.TrueColor) == frameOf(pl, now.Add(400*time.Millisecond)).Encode(pixel.TrueColor) {
		t.Error("rain did not fall")
	}
}

func TestCloudyIsASmallGreyCloud(t *testing.T) {
	pl := demoPlots()[0]
	pl.Weather = Cloudy
	grey := colored(frameOf(pl, at(12, 0)), greyCloud)
	pl.Weather = Storm
	dark := colored(frameOf(pl, at(12, 0)), stormCloud)
	if grey == 0 || grey >= dark {
		t.Errorf("pending CI cloud = %d px, storm cloud = %d px; want a small grey cloud", grey, dark)
	}
}

func TestGlassKeepsWeatherAndSnailsOut(t *testing.T) {
	var glass Plot
	for _, p := range demoPlots() {
		if p.Finished {
			glass = p
		}
	}
	glass.Weather, glass.Snail = Storm, true
	c := frameOf(glass, at(12, 0))
	if colored(c, stormCloud) != 0 || colored(c, shellColor) != 0 {
		t.Error("finished plants under glass should have no weather or snail")
	}
}

func TestSnailCrawls(t *testing.T) {
	pl := demoPlots()[0]
	if colored(frameOf(pl, at(12, 0)), shellColor) != 0 {
		t.Fatal("snail drawn without new issues")
	}
	pl.Snail = true
	a := frameOf(pl, at(12, 0))
	if colored(a, shellColor) == 0 {
		t.Fatal("no snail")
	}
	if a.Encode(pixel.TrueColor) == frameOf(pl, at(12, 0).Add(3*time.Second)).Encode(pixel.TrueColor) {
		t.Error("the snail did not move")
	}
}

func TestSeedHeadsDontAttractCritters(t *testing.T) {
	pl := demoPlots()[0] // gag-core: healthy and flowering
	if !Flowering(pl) {
		t.Fatal("test plot should be flowering")
	}
	pl.Style.Now = pl.Plant.LastTended.Add(400 * 24 * time.Hour)
	if Flowering(pl) {
		t.Error("flowers gone to seed should not count as flowering")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/scene/`
Expected: FAIL (build errors: `undefined: stormCloud`, `pl.Weather undefined`, `undefined: Storm`).

- [ ] **Step 3: Implement weather and the snail**

`internal/scene/weather.go`:

```go
package scene

import (
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// Weather is the sky over one plant, from its CI state.
type Weather int

const (
	Clear  Weather = iota // passing, or no CI
	Cloudy                // CI running
	Storm                 // CI failing
)

var (
	greyCloud   = rgb(172, 178, 190)
	stormCloud  = rgb(70, 74, 92)
	rainColor   = rgb(150, 180, 230)
	shellColor  = rgb(150, 96, 56)
	shellSpiral = rgb(205, 160, 100)
	snailBody   = rgb(176, 172, 150)
)

// rainStep is how long a raindrop takes to fall one pixel; snailCrawl is how
// long the snail takes to move one.
const rainStep, snailCrawl = 90 * time.Millisecond, 3 * time.Second

// drawWeather draws a plant's own weather in its headroom: a small grey cloud
// while CI runs, a dark cloud with rain falling to the pot when it fails.
// Raindrop positions come from the clock, so rain falls in live frames.
func drawWeather(c *pixel.Canvas, cx, oy int, w Weather, t time.Time) {
	switch w {
	case Cloudy:
		puff(c, cx, oy+1, 4, greyCloud)
	case Storm:
		puff(c, cx, oy+1, 8, stormCloud)
		fall := int64(potTop - 3)
		step := t.UnixMilli() / rainStep.Milliseconds()
		for x := cx - 7; x <= cx+7; x += 3 {
			y := oy + 3 + int(((step+int64(x)*5)%fall+fall)%fall)
			c.Blend(x, y, rainColor, 0.85)
			c.Blend(x, y+1, rainColor, 0.6)
		}
	}
}

// puff is a flat-bottomed cloud 2·half+1 px wide whose base is row y+1.
func puff(c *pixel.Canvas, cx, y, half int, col pixel.RGB) {
	for dx := -half; dx <= half; dx++ {
		c.Set(cx+dx, y+1, col)
		if dx > -half && dx < half {
			c.Set(cx+dx, y, col)
		}
		if dx >= -half/2 && dx <= half/2 {
			c.Set(cx+dx, y-1, col)
		}
	}
}

// drawSnail draws a snail crawling on the soil left of the pot: a burst of
// new issues.
func drawSnail(c *pixel.Canvas, cx, oy int, t time.Time) {
	x := cx - BedCols/2 + 1 + int(t.Unix()/int64(snailCrawl/time.Second)%3)
	y := oy + groundTop - 1
	c.Set(x, y, shellColor)
	c.Set(x+1, y, shellSpiral)
	c.Set(x, y-1, shellColor)
	c.Set(x+1, y-1, shellColor)
	c.Set(x+2, y, snailBody)
	c.Set(x+3, y, snailBody)
	c.Set(x+3, y-1, snailBody)
}
```

- [ ] **Step 4: Wire it into plots**

In `internal/scene/compose.go`:

1. Replace `type Plot struct { ... }` with:

```go
// Plot is one plant with what's needed to draw it.
type Plot struct {
	Plant        *garden.Plant
	Style        garden.Style
	Finished     bool
	Name, Status string
	Weather      Weather // the plant's own sky: CI running or failing
	Snail        bool    // a burst of new issues
}
```

2. In `drawPlot`, replace

```go
	if pl.Finished {
		DrawCloche(c, cx, oy+2, oy+potTop+PotH-1, BedCols-3)
		if v.Motion {
			DrawGlint(c, v.Now, cx, oy+2, oy+potTop+PotH-1, BedCols-3, nameSeed(pl.Name))
		}
	}
```

with

```go
	if pl.Finished {
		DrawCloche(c, cx, oy+2, oy+potTop+PotH-1, BedCols-3)
		if v.Motion {
			DrawGlint(c, v.Now, cx, oy+2, oy+potTop+PotH-1, BedCols-3, nameSeed(pl.Name))
		}
	} else {
		drawWeather(c, cx, oy, pl.Weather, v.Now)
		if pl.Snail {
			drawSnail(c, cx, oy, v.Now)
		}
	}
```

In `internal/scene/critters.go`, replace the body of `Flowering` with:

```go
	return !pl.Finished && pl.Style.Health >= 0.6 && pl.Plant.Blooming(pl.Style.Now) > 0
```

and its doc comment's first line with `// Flowering reports whether a plot attracts critters: a healthy plant in bloom`.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/scene/ && go test ./...`
Expected: PASS, with the golden frame unchanged.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/scene
git commit -m "scene: storm clouds and rain for failing CI, a snail for new issues

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Live: signals on plants and an attention-ordered ticker

**Files:**
- Modify: `internal/live/snapshot.go` (`Repo` signals, `CI`), `internal/live/ticker.go` (`Items` in attention order), `internal/live/model.go` (plots via `Plot`; storms keep the fast frame rate)
- Create: `internal/live/plot.go`, `internal/live/plot_test.go`
- Test: `internal/live/ticker_test.go` and `internal/live/model_test.go` (append)

**Interfaces:**
- Consumes: Task 3's `Style.Now/Buds/Rising` and Task 4's `scene.Weather`, `Plot.Weather`, `Plot.Snail` and `Flowering`.
- Produces:
  - `type CI int` with `const CIUnknown, CIPassing, CIPending, CIFailing`
  - `Repo` fields `Branch string`, `CI CI`, `PRs []time.Time`, `NewIssues int`, `Rising bool`
  - `func Plot(r Repo, at time.Time, decay float64) scene.Plot` (everything but sway)
  - The unexported `snailIssues = 3`
  - `Items` order: failing CI, then PRs waiting over 7 days, then wilting, then other open PRs, then new-issue bursts, with a calm line when there's nothing.

- [ ] **Step 1: Write the failing tests**

`internal/live/plot_test.go`:

```go
package live

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func TestPlotCarriesSignals(t *testing.T) {
	r := grow("x", 30, 2, t0.Add(-time.Hour))
	r.CI, r.PRs, r.NewIssues, r.Rising = CIFailing, []time.Time{t0}, 3, true
	pl := Plot(r, t0, 45)
	if pl.Weather != scene.Storm || !pl.Snail || len(pl.Style.Buds) != 1 || !pl.Style.Rising || !pl.Style.Now.Equal(t0) {
		t.Errorf("signals lost: %+v", pl)
	}
	r.CI = CIPending
	if Plot(r, t0, 45).Weather != scene.Cloudy {
		t.Error("running CI should be a grey cloud")
	}
	r.CI = CIUnknown
	if Plot(r, t0, 45).Weather != scene.Clear {
		t.Error("no CI should be clear skies")
	}
	r.CI, r.Finished = CIFailing, true
	pl = Plot(r, t0, 45)
	if pl.Weather != scene.Clear || pl.Snail || len(pl.Style.Buds) != 0 || pl.Style.Health != 1 {
		t.Errorf("finished repos rest under glass without signals: %+v", pl)
	}
}
```

Append to `internal/live/ticker_test.go`:

```go
func TestItemsFollowTheAttentionOrder(t *testing.T) {
	fresh := t0.Add(-time.Hour)
	days := func(d int) time.Time { return t0.Add(-time.Duration(d) * 24 * time.Hour) }
	ci := grow("ci", 20, 0, fresh)
	ci.CI, ci.Branch = CIFailing, "main"
	slow := grow("slow", 20, 0, fresh)
	slow.PRs = []time.Time{days(9), days(2), days(12)}
	quiet := grow("quiet", 20, 0, days(40))
	newpr := grow("newpr", 20, 0, fresh)
	newpr.PRs = []time.Time{days(1)}
	buggy := grow("buggy", 20, 0, fresh)
	buggy.NewIssues = 4
	got := Items([]Repo{buggy, newpr, quiet, slow, ci}, t0, 45, 3)
	want := []string{
		"⚡ ci: CI failing on main",
		"🌷 slow: 2 PRs waiting (12d)",
		"🥀 quiet: 40d quiet",
		"🌷 newpr: 1 PR open",
		"🐌 buggy: 4 new issues this week",
	}
	if len(got) != len(want) {
		t.Fatalf("items = %+v", got)
	}
	for i, w := range want {
		if g := got[i].Icon + " " + got[i].Text; g != w {
			t.Errorf("item %d = %q, want %q", i, g, w)
		}
	}
}

func TestTickerIconsAreTwoCellsWide(t *testing.T) {
	for _, icon := range []string{"⚡", "🌷", "🥀", "🐌", "🌱"} {
		if w := runewidth.StringWidth(icon); w != 2 {
			t.Errorf("%s is %d cells wide; ticker icons must be two-cell emoji", icon, w)
		}
	}
}
```

Append to `internal/live/model_test.go`:

```go
func TestStormsRainAtFullFrameRate(t *testing.T) {
	snap := garden3()
	snap.Repos[2].CI = CIFailing
	night := t0.Add(11 * time.Hour) // 23:00: no critters
	if m := ready(newModel(snap, nil, night), 80, 24); m.frameInterval() != fastFrame {
		t.Error("rain falls at night too: want the fast frame rate")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/live/`
Expected: FAIL (build errors: `r.CI undefined`, `undefined: CIFailing`, `undefined: Plot`).

- [ ] **Step 3: Repo signals**

In `internal/live/snapshot.go`, replace `type Repo struct { ... }` with:

```go
// Repo is one plant in the live garden, with its signals.
type Repo struct {
	Name      string
	Plant     *garden.Plant
	Finished  bool
	Branch    string      // default branch, for "CI failing on main"
	CI        CI          // CI state of the default branch's latest commit
	PRs       []time.Time // when each open, non-draft PR was opened
	NewIssues int         // issues opened in the last 7 days
	Rising    bool        // more commits in the last 14 days than in the 14 before
}

// CI is a repo's build state on its default branch.
type CI int

const (
	CIUnknown CI = iota // no checks, or not fetched yet
	CIPassing
	CIPending
	CIFailing
)
```

- [ ] **Step 4: `Plot`**

`internal/live/plot.go`:

```go
package live

import (
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

// snailIssues is how many issues opened in a week bring out the snail.
const snailIssues = 3

// Plot is how a repo looks at time at, without motion: health, blooms, buds,
// shoots, weather and snail. Finished repos rest under glass with none of
// these.
func Plot(r Repo, at time.Time, decay float64) scene.Plot {
	pl := scene.Plot{Plant: r.Plant, Finished: r.Finished, Name: r.Name,
		Status: garden.Status(r.Plant, at, r.Finished), Style: garden.Style{Health: 1, Now: at}}
	if r.Finished {
		return pl
	}
	pl.Style = garden.Style{Health: garden.Health(r.Plant, at, decay), Now: at, Buds: r.PRs, Rising: r.Rising}
	pl.Weather = weatherFor(r.CI)
	pl.Snail = r.NewIssues >= snailIssues
	return pl
}

func weatherFor(ci CI) scene.Weather {
	switch ci {
	case CIFailing:
		return scene.Storm
	case CIPending:
		return scene.Cloudy
	}
	return scene.Clear
}
```

- [ ] **Step 5: The ticker in attention order**

In `internal/live/ticker.go`:

1. Add `waitingAge = 7 * 24 * time.Hour // PRs open longer than this are waiting on you` inside the existing `const (...)` block.
2. Replace the whole `Items` function (and its doc comment) with:

```go
// Items lists what needs attention, most urgent first: failing CI, then PRs
// waiting over a week, then wilting plants (most wilted first), then other
// open PRs, then bursts of new issues. When nothing needs attention it
// returns one calm line.
func Items(repos []Repo, now time.Time, decay float64, commits7d int) []Item {
	type wilting struct {
		item   Item
		health float64
	}
	var failing, waiting, open, weeds []Item
	var ws []wilting
	for _, r := range repos {
		if r.Finished {
			continue
		}
		if r.CI == CIFailing {
			failing = append(failing, Item{"⚡", r.Name + ": CI failing" + onBranch(r.Branch)})
		}
		if n, oldest := waitingPRs(r.PRs, now); n > 0 {
			waiting = append(waiting, Item{"🌷", fmt.Sprintf("%s: %s waiting (%dd)", r.Name, plural(n, "PR"), int(oldest.Hours()/24))})
		} else if len(r.PRs) > 0 {
			open = append(open, Item{"🌷", fmt.Sprintf("%s: %s open", r.Name, plural(len(r.PRs), "PR"))})
		}
		if !r.Plant.LastTended.IsZero() {
			if h := garden.Health(r.Plant, now, decay); h < wiltThreshold {
				idle := int(now.Sub(r.Plant.LastTended).Hours() / 24)
				ws = append(ws, wilting{Item{"🥀", fmt.Sprintf("%s: %dd quiet", r.Name, idle)}, h})
			}
		}
		if r.NewIssues >= snailIssues {
			weeds = append(weeds, Item{"🐌", fmt.Sprintf("%s: %d new issues this week", r.Name, r.NewIssues)})
		}
	}
	sort.SliceStable(ws, func(i, j int) bool { return ws[i].health < ws[j].health })
	items := append(failing, waiting...)
	for _, w := range ws {
		items = append(items, w.item)
	}
	items = append(append(items, open...), weeds...)
	if len(items) == 0 {
		items = append(items, calm(commits7d))
	}
	return items
}

// waitingPRs counts PRs open longer than waitingAge, and the oldest one's age.
func waitingPRs(opened []time.Time, now time.Time) (n int, oldest time.Duration) {
	for _, at := range opened {
		if age := now.Sub(at); age > waitingAge {
			n++
			oldest = max(oldest, age)
		}
	}
	return n, oldest
}

func onBranch(branch string) string {
	if branch == "" {
		return ""
	}
	return " on " + branch
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
```

- [ ] **Step 6: The model uses `Plot`; storms keep the frame rate up**

In `internal/live/model.go`:

1. Replace the body of `plots` with:

```go
	out := make([]scene.Plot, len(m.repos))
	for i, r := range m.repos {
		pl := Plot(r, at, m.cfg.DecayDays)
		if !r.Finished { // still air under glass
			pl.Style.Sway = sway(r.Name, pl.Style.Health, at)
		}
		out[i] = pl
	}
	return out
```

2. In `busy`, replace

```go
	at := m.now()
	if scene.Darkness(at) > 0.5 {
		return false
	}
	for _, pl := range m.plots(at) {
		if scene.Flowering(pl) {
			return true
		}
	}
	return false
```

with

```go
	at := m.now()
	day := scene.Darkness(at) <= 0.5
	for _, pl := range m.plots(at) {
		if pl.Weather == scene.Storm || (day && scene.Flowering(pl)) {
			return true // rain falls, or critters fly
		}
	}
	return false
```

and update the doc comment of `frameInterval` to `// frameInterval is fast while critters fly, rain falls or the camera slides (or is about to), slow otherwise, so an idle garden costs little.`

- [ ] **Step 7: Run the tests**

Run: `go test ./internal/live/ && go test ./...`
Expected: PASS, including the v0.3 ticker tests (with no signals they read the same).

- [ ] **Step 8: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/live
git commit -m "live: signals on plants and a ticker in attention order

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: `cmd/gag`: compute signals from GitHub, show them in the demo

**Files:**
- Modify: `cmd/gag/garden.go` (`repoFor`, `ciFor`, the demo with signals, `printOnce` via `live.Plot`; `plotFor` removed)
- Test: `cmd/gag/garden_test.go` (append)

**Interfaces:**
- Consumes: Task 1's `github.Repo` fields, Task 5's `live.Repo` fields, `live.CI` and `live.Plot`, and Task 2's `snapshot(ctx, progress)`.
- Produces:
  - `func repoFor(r *github.Repo, now time.Time) live.Repo`
  - `func ciFor(state string) live.CI`
  - `demoGarden(now) []live.Repo` with signals.

- [ ] **Step 1: Write the failing tests**

Append to `cmd/gag/garden_test.go`:

```go
func TestCIForMapsGitHubStates(t *testing.T) {
	cases := map[string]live.CI{
		"SUCCESS":       live.CIPassing,
		"FAILURE":       live.CIFailing,
		"ERROR":         live.CIFailing,
		"PENDING":       live.CIPending,
		"EXPECTED":      live.CIPending,
		"":              live.CIUnknown,
		"SOMETHING_NEW": live.CIUnknown,
	}
	for state, want := range cases {
		if got := ciFor(state); got != want {
			t.Errorf("ciFor(%q) = %v, want %v", state, got, want)
		}
	}
}

func TestRepoForComputesSignals(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	day := func(d int) time.Time { return now.Add(-time.Duration(d) * 24 * time.Hour) }
	r := &github.Repo{NameWithOwner: "me/x", Branch: "main", CI: "FAILURE",
		OpenPRs: []github.OpenPR{{Number: 1, CreatedAt: day(9)}, {Number: 2, CreatedAt: day(1), Draft: true}},
		Issues: []github.Issue{{Number: 1, CreatedAt: day(1)}, {Number: 2, CreatedAt: day(2)},
			{Number: 3, CreatedAt: day(3)}, {Number: 4, CreatedAt: day(20)}},
		Commits: []github.Commit{{At: day(1)}, {At: day(2)}, {At: day(3)}, {At: day(20)}},
	}
	lr := repoFor(r, now)
	if lr.Name != "x" || lr.CI != live.CIFailing || lr.Branch != "main" {
		t.Errorf("name %q, ci %v, branch %q", lr.Name, lr.CI, lr.Branch)
	}
	if len(lr.PRs) != 1 || !lr.PRs[0].Equal(day(9)) {
		t.Errorf("PRs = %v, want only the non-draft one", lr.PRs)
	}
	if lr.NewIssues != 3 || !lr.Rising {
		t.Errorf("new issues %d (want 3), rising %v (want true: 3 commits vs 1)", lr.NewIssues, lr.Rising)
	}
}

func TestDemoShowsTheSignals(t *testing.T) {
	var storms, buds, snails, rising int
	for _, r := range demoGarden(time.Now()) {
		if r.CI == live.CIFailing {
			storms++
		}
		buds += len(r.PRs)
		if r.NewIssues >= 3 {
			snails++
		}
		if r.Rising {
			rising++
		}
	}
	if storms == 0 || buds == 0 || snails == 0 || rising == 0 {
		t.Errorf("demo should show every signal: storms %d, buds %d, snails %d, rising %d", storms, buds, snails, rising)
	}
}
```

and add `"github.com/RursusAeternum/GitAGarden/internal/live"` to that file's imports.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./cmd/gag/`
Expected: FAIL (build errors: `undefined: ciFor`, `undefined: repoFor`).

- [ ] **Step 3: Implement `repoFor` and `ciFor`**

In `cmd/gag/garden.go`, add:

```go
// repoFor grows a repo's plant and works out its signals as of now.
func repoFor(r *github.Repo, now time.Time) live.Repo {
	lr := live.Repo{Name: r.Name(), Plant: garden.Grow(r.Name(), r.Species(), r.Events()),
		Finished: r.Finished(), Branch: r.Branch, CI: ciFor(r.CI)}
	for _, pr := range r.OpenPRs {
		if !pr.Draft { // drafts aren't waiting on anyone
			lr.PRs = append(lr.PRs, pr.CreatedAt)
		}
	}
	weekAgo := now.Add(-7 * 24 * time.Hour)
	for _, is := range r.Issues {
		if is.CreatedAt.After(weekAgo) {
			lr.NewIssues++
		}
	}
	var last, before int // commits in the last 14 days, and the 14 before
	for _, c := range r.Commits {
		switch age := now.Sub(c.At); {
		case age < 14*24*time.Hour:
			last++
		case age < 28*24*time.Hour:
			before++
		}
	}
	lr.Rising = last > before
	return lr
}

// ciFor maps GitHub's statusCheckRollup state to a CI state. States GitHub
// may add later count as unknown: clear skies rather than a false alarm.
func ciFor(state string) live.CI {
	switch state {
	case "SUCCESS":
		return live.CIPassing
	case "FAILURE", "ERROR":
		return live.CIFailing
	case "PENDING", "EXPECTED":
		return live.CIPending
	}
	return live.CIUnknown
}
```

- [ ] **Step 4: Use `repoFor` and `live.Plot` everywhere**

In `cmd/gag/garden.go`:

1. In `snapshot`, replace `		snap.Repos = append(snap.Repos, live.Repo{Name: r.Name(), Plant: garden.Grow(r.Name(), r.Species(), r.Events()), Finished: r.Finished()})` with `		snap.Repos = append(snap.Repos, repoFor(r, now))`.
2. In `printOnce`, replace

```go
		for _, r := range demoGarden(now) {
			plots = append(plots, plotFor(r.Plant, r.Name, r.Finished, at, src.decay))
		}
```

with

```go
		for _, r := range demoGarden(now) {
			plots = append(plots, live.Plot(r, at, src.decay))
		}
```

and replace

```go
		for _, r := range repos {
			plots = append(plots, plotFor(garden.Grow(r.Name(), r.Species(), r.Events()), r.Name(), r.Finished(), at, src.decay))
		}
```

with

```go
		for _, r := range repos {
			plots = append(plots, live.Plot(repoFor(r, now), at, src.decay))
		}
```

3. Delete the `plotFor` function.
4. Replace `type demoRepo struct { ... }`, `var demo = ...` and `func demoGarden` with:

```go
type demoRepo struct {
	name, lang string
	events     int
	idleDays   int
	finished   bool
	ci         live.CI
	prDaysAgo  []int // open PRs, by age in days
	newIssues  int
	rising     bool
}

var demo = []demoRepo{
	{name: "gag-core", lang: "go", events: 160, ci: live.CIPassing, prDaysAgo: []int{2, 10}, rising: true},
	{name: "rustyfs", lang: "rust", events: 90, idleDays: 12, ci: live.CIFailing},
	{name: "notebook-api", lang: "python", events: 70, idleDays: 3, ci: live.CIPending, newIssues: 4},
	{name: "old-blog", lang: "go", events: 120, idleDays: 400, finished: true},
	{name: "dotfiles", lang: "shell", events: 40, idleDays: 35},
	{name: "tiny-cli", lang: "rust", events: 12, idleDays: 1, prDaysAgo: []int{1}},
	{name: "site-v2", lang: "typescript", events: 110, idleDays: 70},
	{name: "lsystem", lang: "c", events: 60, idleDays: 200, finished: true},
}

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
			Finished: r.finished, Branch: "main", CI: r.ci, NewIssues: r.newIssues, Rising: r.rising}
		for _, d := range r.prDaysAgo {
			lr.PRs = append(lr.PRs, now.Add(-time.Duration(d)*24*time.Hour))
		}
		out = append(out, lr)
	}
	return out
}
```

- [ ] **Step 5: Run the tests and look at the demo**

Run: `gofmt -l . ; go vet ./... && go test ./... && go build -o gag ./cmd/gag && GAG_COLOR=truecolor ./gag garden -demo --once > /tmp/gag-demo.txt; head -c 200 /tmp/gag-demo.txt | wc -c`
Expected: all tests PASS, and the demo prints. View `/tmp/gag-demo.txt` in a true-color terminal (`cat /tmp/gag-demo.txt`). You should see:
- rustyfs under a dark cloud with rain
- notebook-api under a small grey cloud, with a snail by its pot
- gag-core with pink buds (one pale, drooping) and lime shoots
- older flowers turned to beige seed heads

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "gag computes signals from GitHub; the demo shows every one

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: v0.3 leftovers: refresh on wake, the -simulate label, README

**Files:**
- Modify: `internal/live/model.go` (`Config.Label`, refresh on wake, wall-clock freshness), `cmd/gag/garden.go` (pass the label), `README.md`
- Test: `internal/live/model_test.go` (append)

**Interfaces:**
- Consumes: the Task 2 model.
- Produces:
  - `live.Config.Label string` (shown before the ticker's status)
  - The model starts a load when a tick finds the wall clock jumped more than `wakeGap` (1 minute).

- [ ] **Step 1: Write the failing tests**

Append to `internal/live/model_test.go`:

```go
func TestWakingUpRefreshes(t *testing.T) {
	now := t0
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return garden3(), nil },
		DecayDays: 45,
		Now:       func() time.Time { return now },
	})
	m = ready(m, 80, 24)
	m, _ = step(m, tickMsg{}) // the first tick notes the time
	now = now.Add(125 * time.Millisecond)
	if m, _ = step(m, tickMsg{}); m.loading {
		t.Fatal("an ordinary tick should not reload")
	}
	now = now.Add(9 * time.Hour) // the lid was closed overnight
	m, _ = step(m, tickMsg{})
	if !m.loading {
		t.Error("waking up should start a refresh")
	}
}

func TestLabelShowsInTheTicker(t *testing.T) {
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return garden3(), nil },
		DecayDays: 45,
		Now:       func() time.Time { return t0 },
		Label:     "simulating 30d ahead",
	})
	m = ready(m, 100, 30)
	lines := strings.Split(m.View(), "\n")
	if last := visible(lines[len(lines)-1]); !strings.Contains(last, "simulating 30d ahead") {
		t.Errorf("ticker = %q", last)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/live/`
Expected: FAIL (build error: `unknown field Label in struct literal of type Config`).

- [ ] **Step 3: Implement**

In `internal/live/model.go`:

1. Add `wakeGap = time.Minute // a longer gap between ticks means the machine slept` to the `const (...)` block.
2. In `Config`, add the field `Label string // shown before the ticker's status, e.g. "simulating 30d ahead"`.
3. In `Model`, add the field `lastTick time.Time // wall-clock time of the last frame, to notice sleep`.
4. Replace

```go
	case tickMsg:
		return m, m.tick()
```

with

```go
	case tickMsg:
		now := m.cfg.Now().Round(0) // wall clock: the monotonic one stops while the machine sleeps
		woke := !m.lastTick.IsZero() && now.Sub(m.lastTick) > wakeGap
		m.lastTick = now
		if woke && !m.loading && len(m.repos) > 0 {
			m.loading = true
			return m, tea.Batch(m.tick(), m.load(false))
		}
		return m, m.tick()
```

5. In `tickerLine`, change `	status := Status(m.snap, m.cfg.Now(), m.loading, m.loadErr != nil)` to `	status := Status(m.snap, m.cfg.Now().Round(0), m.loading, m.loadErr != nil)`. Then, after the `if m.loading && m.progress.Total > 0 { … }` block from Task 2, add:

```go
	if m.cfg.Label != "" {
		status = strings.TrimSuffix(m.cfg.Label+" · "+status, " · ")
	}
```

and add `"strings"` to the imports.

In `cmd/gag/garden.go`, in `runGarden`, change the `live.New(live.Config{...})` line to:

```go
	label := ""
	if ahead > 0 {
		label = fmt.Sprintf("simulating %s ahead", *simulate)
	}
	m := live.New(live.Config{Load: src.snapshot, Refresh: *refresh, DecayDays: *decay, Ahead: ahead, Profile: colorProfile(), Label: label})
```

- [ ] **Step 4: Update the README**

In `README.md`:

1. Replace

```
- **Cache:** histories are stored in the OS cache dir (`~/.cache/gag` on Linux,
  `~/Library/Caches/gag` on macOS). They refresh at most every `-ttl` (15m), and
  commits are fetched incrementally. If GitHub is unreachable, the garden falls
  back to the cached data.
```

with

```
- **Cache:** histories are stored in the OS cache dir (`~/.cache/gag` on Linux,
  `~/Library/Caches/gag` on macOS). Commits are fetched incrementally. `--once`
  reuses data younger than `-ttl` (15m); the live view refetches on every
  refresh and when you press `r`. If GitHub is unreachable, the garden falls
  back to your cached repos.
- **No token?** The live view shows the demo garden and tells you how to sign in.
```

2. Replace the table under `## How plants grow`:

```
| Event | Adds |
|---|---|
| push | a leaf, a stem segment, or a spine; once full, leaves get lusher |
| merged PR | a flower |
| release | fruit |
| issue opened / closed | adds / removes a weed by the pot |
| time since last tended | leaves yellow → brown → fall, flowers droop (applied at render) |
```

with

```
| Signal | In the garden |
|---|---|
| push | a leaf, a stem segment, or a spine; once full, leaves get lusher |
| merged PR | a flower, which goes to seed 30 days later |
| release | fruit |
| open PR | a pink bud (up to 5); after a week of waiting it droops and fades |
| CI on the default branch | running: a small grey cloud · failing: a storm cloud with rain |
| more commits in the last 14 days than the 14 before | bright new shoots |
| open issues | weeds by the pot; 3+ new issues in a week bring a snail |
| time since last tended | leaves yellow → brown → fall, flowers droop |

The ticker at the bottom of the live view names what needs you, most urgent
first: failing CI, PRs waiting over a week, wilting plants, other open PRs,
then bursts of new issues.
```

3. Replace the `## Layout` code block's contents with:

```
cmd/gag/            CLI: the live garden, --once prints, replay
internal/github/    GraphQL client, repo histories and signals, on-disk cache
internal/garden/    events, growth per species, the plant painter
internal/scene/     sky, clouds, weather, critters, pots, beds and layout
internal/pixel/     RGB canvas encoded as half-block terminal text
internal/live/      the live Bubble Tea view and its ticker
internal/replay/    Bubble Tea time-lapse of one plant
```

- [ ] **Step 5: Run the tests**

Run: `gofmt -l . ; go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "Refresh on wake, label -simulate in the live view, update the README

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Real-world check and v0.4.0 release gate

**Files:**
- None, unless the check turns up bugs.

- [ ] **Step 1: Static check on real data**

Run: `go build -o gag ./cmd/gag && GAG_COLOR=truecolor ./gag garden --once -ttl 0 -limit 8`
Expected, with stderr a terminal: a progress bar on stderr while it loads, then the user's garden. Any repo with failing CI has a storm, and KaineWebPage's old merges show as beige seed heads instead of a tower of fresh flowers.

- [ ] **Step 2: Hand the live view to the user**

Ask the user to run `./gag` and check:
- the progress bar on the first load
- `r` shows `refreshing ███░░░░░ 3/8` in the ticker
- storms and rain, buds, shoots and snails where their repos have them
- the ticker's attention order
- `./gag -simulate 40d` shows "simulating 40d ahead" in the ticker, with flowers gone to seed and buds drooping

Fix what they report, with a failing test first.

- [ ] **Step 3: Release gate**

Merging to `main` and tagging publish a public release. Only on the user's explicit yes:

```bash
git tag -a v0.4.0 -m "v0.4.0: the garden shows what needs you"
git push origin main v0.4.0
gh workflow run update-gag.yml -R RursusAeternum/homebrew-tap
```
