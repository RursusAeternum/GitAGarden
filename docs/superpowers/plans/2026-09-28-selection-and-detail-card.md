# Plant Selection and Detail Card Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let you pick a plant in the live view (`←/→` or a click) and open a detail card beside it (`enter`, closed with `esc`) that shows the facts behind the plant's look. Fold in the six v0.5 polish fixes.

**Architecture:**
- `live.Repo` gains a `Detail` section, filled by `cmd/gag` from GitHub data that's already fetched.
- `internal/scene` owns where things land. One `geometry` of beds and slots is shared by `Draw`, the new `PlotAt` and `OnScreen`, and the card's placement, so clicks match the drawing. `scene` also highlights the selected plant and draws the card into the pixel canvas.
- `internal/live` formats the card's lines (`CardFor`) and holds the selection, the camera hold and the input handling.

**Tech Stack:** Go 1.26, Bubble Tea v1.3.10 (mouse cell motion, `tea.MouseMsg`), go-runewidth.

**Spec:** `docs/superpowers/specs/2026-09-28-selection-and-detail-card-design.md`, on top of v0.5.0. It ships as v0.6.0.

**Plan refinements of the spec.** Each keeps the spec's intent:
1. **`View.Selected` is 1-based: `0`, the zero value, means none.** The spec says −1 for none. With 1-based numbering, every existing `scene.View{…}` literal (`--once`, replay, tests) still selects nothing without being touched.
2. **`CardFor(r, now, decay, maxRows)` has no width.** `scene` does the text cleanup (control characters dropped, wide runes replaced with `?`) and the `…` trimming when it draws, because that is where the one-column guarantee has to hold. The spec's live tests for cleanup become scene tests.
3. **PR lines put the age right-aligned** (`#12 Add sparkline      2d`) instead of `(2d)` inline, so trimming a long title never cuts the age off.
4. **The sky above the top bed counts as that bed's slots for clicks.** It is the plants' own sky: in a window taller than the beds, the top bed's sky reaches up to the top of the frame.
5. **While `?` shows the keys, the ticker's status gives way.** The new key list is 68 columns. With the status beside it, an 80-column window would cut off `q quit`.

## Global Constraints

- Module `github.com/RursusAeternum/GitAGarden`, `go 1.26.0`. Go is at `~/sdk/go/bin`: run `export PATH=$HOME/sdk/go/bin:$PATH` before any `go` command.
- No new third-party modules.
- With nothing selected and no card, `scene.Draw` produces exactly today's frame: `internal/scene/testdata/garden.golden` passes unchanged in every task. `--once` and replay never select anything.
- Every glyph the card draws is one column wide under go-runewidth: `┌─┐│└┘★✓✗●–·…` and `▁▂▃▄▅▆▇█`.
- The card is 38 columns wide, or the window width minus 2 when that's narrower, and its label column is 9 wide.
- The live frame always exactly fills the window, with or without a card (`checkSize`).
- The frame budget: a 240×65 live frame with a card open renders in under 10 ms.
- The help line reads exactly `←/→ select · enter details · r refresh · t ticker · ? help · q quit`.
- A selection clears after 60 s without a key press or left click.
- Only a left-button press counts as a click.
- After every task, `go vet ./... && go test ./...` passes and `gofmt -l .` prints nothing.
- Every commit message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Resizing with a plant selected or its card open.** The window might grow wide enough that the garden stops panning, shrink to 24×12, or come back again. The selection must survive, the camera must bring the plant back on screen, and the frame must always fit. Pinned by Task 5 `TestResizeKeepsTheSelection`.
2. **Input before the garden loads.** Arrow keys, `enter` and clicks on the loading screen do nothing, and nothing panics. Pinned by Task 5 `TestInputBeforeTheGardenLoads`.
3. **Real-world text on the card.** Emoji, CJK, tabs, stray escape codes and very long titles are cleaned or trimmed, and the frame stays exactly window-sized. Pinned by Task 3 `TestCardTextIsOneColumnWide`.
4. **Clicks at the edges.** The sky above the top bed, the margins beside the slots, a slot half off screen mid-pan, the ticker line, and inside the card each do the documented thing. Pinned by Task 2 `TestPlotAtFollowsThePanningCamera` and `TestPlotAtIsEmptyBesideTheSlots`, and Task 5 `TestClicksSelectAndClear` and `TestClickOnTheCardDoesNothing`.
5. **A machine that sleeps with a card open.** On wake the selection has timed out, the card is closed and the ambient screen is back. Pinned by Task 5 `TestLongSleepClosesTheCard`.

---

### Task 1: Card data: `live.Detail`, filled from GitHub and the demo

**Files:**
- Create: `internal/live/detail.go`, `internal/live/detail_test.go`, `cmd/gag/detail.go`, `cmd/gag/detail_test.go`
- Modify: `internal/live/snapshot.go` (`Repo.Detail`), `cmd/gag/garden.go` (`repoFor` fills `Detail`; the demo's type, list and `demoGarden`)

**Interfaces:**
- Consumes: `github.Repo` (`NameWithOwner`, `Language`, `Commits []Commit{At, Message}`, `OpenPRs []OpenPR{Number, Title, CreatedAt, Draft}`, `Issues []Issue{Number, Title, CreatedAt, ClosedAt *time.Time}`, `Releases []Release{Tag, CreatedAt}`); `garden.FakeHistory` event notes (`"issue #%d opened"`, `"issue #%d closed"`, `"released v%d.%d.0"`).
- Produces:
  - `type live.Entry struct{ Number int; Title string; At time.Time }`
  - `type live.Detail struct{ FullName, Language string; LastCommit Entry; Daily [14]int; PRs []Entry; Drafts, OpenIssues int; NewestIssue, Release Entry }`
  - `live.Repo.Detail Detail`
  - `func live.DailyCommits(times []time.Time, now time.Time) [14]int`
  - In `cmd/gag`: `detailFor(r *github.Repo, now time.Time) live.Detail` and `demoDetail(r demoRepo, events []garden.Event, now time.Time) live.Detail`.

- [ ] **Step 1: Write the failing tests**

`internal/live/detail_test.go`:

```go
package live

import (
	"testing"
	"time"
)

func TestDailyCommitsCountsLocalDays(t *testing.T) {
	loc := time.FixedZone("here", 2*60*60)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, loc)
	times := []time.Time{
		now.Add(-time.Hour),                           // today
		time.Date(2026, 6, 14, 0, 30, 0, 0, loc),      // today, just after midnight
		time.Date(2026, 6, 13, 22, 0, 0, 0, time.UTC), // midnight here: today
		time.Date(2026, 6, 13, 23, 30, 0, 0, loc),     // yesterday
		time.Date(2026, 6, 1, 9, 0, 0, 0, loc),        // 13 days ago: the first day
		time.Date(2026, 5, 31, 9, 0, 0, 0, loc),       // 14 days ago: too old
		now.Add(time.Hour),                            // the future
	}
	want := [14]int{0: 1, 12: 1, 13: 3}
	if got := DailyCommits(times, now); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
```

`cmd/gag/detail_test.go`:

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
		Commits: []github.Commit{{At: ago(50), Message: "older"}, {At: ago(2), Message: "newest"}, {At: ago(480), Message: "ancient"}},
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
		LastCommit:  live.Entry{Title: "newest", At: ago(2)},
		Daily:       [14]int{11: 1, 13: 1},
		PRs:         []live.Entry{{Number: 9, Title: "Bump deps", At: ago(240)}, {Number: 11, Title: "Add sparkline", At: ago(48)}},
		Drafts:      1,
		OpenIssues:  2,
		NewestIssue: live.Entry{Number: 31, Title: "Crash on empty repo", At: ago(48)},
		Release:     live.Entry{Title: "v0.5.0", At: ago(1)},
	}
	if got := repoFor(r, now).Detail; !reflect.DeepEqual(got, want) {
		t.Errorf("detail =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDemoReposHaveDetails(t *testing.T) {
	var prTitles, drafts, releases, issues int
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
	}
	if prTitles == 0 || drafts == 0 || releases == 0 || issues == 0 {
		t.Errorf("the demo should show every kind of detail: %d PR titles, %d drafts, %d releases, %d newest issues",
			prTitles, drafts, releases, issues)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/live/ ./cmd/gag/`
Expected: FAIL, with build errors such as `undefined: DailyCommits` and `undefined: live.Detail`.

- [ ] **Step 3: The live types**

`internal/live/detail.go`:

```go
package live

import (
	"math"
	"time"
)

// Detail is what a repo's detail card shows beyond its plant.
type Detail struct {
	FullName    string  // owner/name
	Language    string
	LastCommit  Entry   // the default branch's newest commit; zero when none
	Daily       [14]int // commits per local day, oldest first; the last is today
	PRs         []Entry // open non-draft PRs, oldest first
	Drafts      int     // open draft PRs
	OpenIssues  int
	NewestIssue Entry // the newest open issue; zero when none
	Release     Entry // the latest release, its tag as Title; zero when none
}

// Entry is one dated thing on the card: a commit, PR, issue or release.
// Number is 0 for commits and releases.
type Entry struct {
	Number int
	Title  string
	At     time.Time
}

// DailyCommits counts the commits at times per local calendar day for the
// 14 days ending today, oldest first. Older and future commits are left out.
func DailyCommits(times []time.Time, now time.Time) [14]int {
	var days [14]int
	loc := now.Location()
	midnight := func(t time.Time) time.Time {
		y, m, d := t.In(loc).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, loc)
	}
	today := midnight(now)
	for _, t := range times {
		if t.After(now) {
			continue
		}
		back := int(math.Round(today.Sub(midnight(t)).Hours() / 24)) // DST days run 23 or 25 hours
		if back < len(days) {
			days[len(days)-1-back]++
		}
	}
	return days
}
```

In `internal/live/snapshot.go`, in `type Repo struct`, add after the `Stars` field:

```go
	Detail    Detail      // what the detail card shows
```

- [ ] **Step 4: Fill it from GitHub and the demo**

`cmd/gag/detail.go`:

```go
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
```

In `cmd/gag/garden.go`:

1. In `repoFor`, change `		Finished: r.Finished(), Branch: r.Branch, CI: ciFor(r.CI), Stars: r.Stars}` to `		Finished: r.Finished(), Branch: r.Branch, CI: ciFor(r.CI), Stars: r.Stars, Detail: detailFor(r, now)}`.
2. Replace everything from `type demoRepo struct {` down to the closing `}` of `func demoGarden` with:

```go
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
```

`garden.SpeciesFor` lowercases its argument, so the capitalised language names give the same species as before.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/live/ ./cmd/gag/ && go test ./...`
Expected: PASS. `TestDemoShowsTheSignals` and the golden frame are unchanged.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/live cmd/gag
git commit -m "Card data: each repo's details from GitHub, and made-up ones for the demo

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Scene: selection highlight, `PlotAt`, and the camera helpers

**Files:**
- Create: `internal/scene/select.go`, `internal/scene/select_test.go`
- Modify: `internal/scene/draw.go` (`View.Selected`; `Draw` uses `geometry` and lights the selected slot), `internal/scene/compose.go` (`drawPlot` gets `selected`, and the name label turns white)

**Interfaces:**
- Consumes: v0.5's `View`, `Draw`, `LayoutFor`, `slots`, `PanAt`, `Sliding`, `panHold`, `panSlide`, `bedPx`, `groundTop`, `BedCols`; the test helpers `demoPlots()`, `manyPlots(n)` and `at(h, m)`.
- Produces:
  - `View.Selected int`: 1 + the selected plot's index, and 0 for none.
  - `func PlotAt(v View, col, row int) (index int, ok bool)`
  - `func OnScreen(v View) []int` and `func ReadingOrder(lay Layout, n int) []int`
  - `const SlideFor = panSlide`
  - `func PanShowing(pan float64, col, perRow, columns int) float64`
  - `func SlidePan(from, to, f float64, columns int) float64`
  - `func ResumeAt(col int) time.Duration`
  - Unexported: `geometry`, `geometryOf(v View) geometry`, `(geometry).bedTop(b int) int`, `(geometry).slots(v View, b int) []slot` and `slotLeft(cx int) int`, all of which Task 3 uses.

- [ ] **Step 1: Write the failing tests**

`internal/scene/select_test.go`:

```go
package scene

import (
	"reflect"
	"strings"
	"testing"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// litBy lists the pixels that change when plot i is selected in v.
func litBy(v View, i int) [][2]int {
	plain := Draw(v)
	v.Selected = i + 1
	lit := Draw(v)
	var out [][2]int
	for y := 0; y < plain.H; y++ {
		for x := 0; x < plain.W; x++ {
			if plain.At(x, y) != lit.At(x, y) {
				out = append(out, [2]int{x, y})
			}
		}
	}
	return out
}

func TestSelectionLightsItsGroundStrip(t *testing.T) {
	v := View{Cols: 3*BedCols + 10, Plots: demoPlots(), Now: at(14, 0), Seed: 1}
	for i := range v.Plots {
		px := litBy(v, i)
		if len(px) == 0 {
			t.Fatalf("selecting plot %d changed nothing", i)
		}
		for _, p := range px {
			if got, ok := PlotAt(v, p[0], p[1]/2); !ok || got != i {
				t.Fatalf("plot %d lit pixel %v, but PlotAt there says %d, %v", i, p, got, ok)
			}
			if p[1]%bedPx < groundTop {
				t.Fatalf("plot %d lit pixel %v, above its ground strip", i, p)
			}
		}
	}
	plain := Draw(v).Encode(pixel.TrueColor)
	v.Selected = 2
	lit := Draw(v).Encode(pixel.TrueColor)
	if white := "38;2;255;255;255m"; strings.Count(lit, white) <= strings.Count(plain, white) {
		t.Error("the selected plant's name label should turn white")
	}
}

func TestPlotAtFollowsThePanningCamera(t *testing.T) {
	v := View{Cols: 80, Rows: 23, Plots: manyPlots(8), Now: at(14, 0), Seed: 1, Pan: 2.5}
	lit := 0
	for i := range v.Plots {
		for _, p := range litBy(v, i) {
			lit++
			if got, ok := PlotAt(v, p[0], p[1]/2); !ok || got != i {
				t.Fatalf("mid-pan, plot %d lit pixel %v, but PlotAt there says %d, %v", i, p, got, ok)
			}
		}
	}
	if lit == 0 {
		t.Fatal("no plant on screen lit up")
	}
	if _, ok := PlotAt(v, 10, 23); ok {
		t.Error("a row below the frame holds no plant")
	}
}

func TestPlotAtIsEmptyBesideTheSlots(t *testing.T) {
	v := View{Cols: 3*BedCols + 10, Rows: 30, Plots: demoPlots(), Now: at(14, 0), Seed: 1} // 5-column margins
	for _, col := range []int{0, 4, 3*BedCols + 5, 3*BedCols + 9, -1, 3*BedCols + 10} {
		if i, ok := PlotAt(v, col, 20); ok {
			t.Errorf("column %d holds plot %d; want open ground", col, i)
		}
	}
	if i, ok := PlotAt(v, 5, 20); !ok || i != 0 {
		t.Errorf("column 5 = %d, %v; want the first plant", i, ok)
	}
	if i, ok := PlotAt(v, 5, 1); !ok || i != 0 {
		t.Errorf("the sky above the top bed = %d, %v; want the first plant's", i, ok)
	}
}

func TestOnScreenAndReadingOrder(t *testing.T) {
	cases := []struct {
		v    View
		want []int
	}{
		{View{Cols: 100, Plots: manyPlots(3)}, []int{0, 1, 2}},
		{View{Cols: 80, Rows: 23, Plots: manyPlots(8), Pan: 2}, []int{2, 3, 4}},
		{View{Cols: 80, Rows: 23, Plots: manyPlots(8), Pan: 7}, []int{7, 0, 1}},
		{View{Cols: 80, Plots: manyPlots(5)}, []int{0, 1, 2, 3, 4}},
	}
	for _, c := range cases {
		if got := OnScreen(c.v); !reflect.DeepEqual(got, c.want) {
			t.Errorf("OnScreen(%d cols, pan %v) = %v, want %v", c.v.Cols, c.v.Pan, got, c.want)
		}
	}
	panning := Layout{PerRow: 3, Beds: 2, Columns: 4, Overflow: true}
	if got, want := ReadingOrder(panning, 7), []int{0, 2, 4, 6, 1, 3, 5}; !reflect.DeepEqual(got, want) {
		t.Errorf("ReadingOrder(panning) = %v, want %v", got, want)
	}
	if got, want := ReadingOrder(Layout{PerRow: 3, Beds: 2, Columns: 3}, 5), []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(got, want) {
		t.Errorf("ReadingOrder(fitting) = %v, want %v", got, want)
	}
}

func TestCameraHelpers(t *testing.T) {
	for _, c := range []struct {
		pan              float64
		col, per, cols   int
		want             float64
	}{
		{0, 1, 3, 8, 0},   // already on screen
		{0, 7, 3, 8, 7},   // one step left, round the ring
		{0, 4, 3, 8, 2},   // two steps right
		{2.6, 1, 3, 8, 1}, // mid-slide: nearest from where the camera is
		{5, 0, 3, 3, 0},   // the garden fits: no panning
	} {
		if got := PanShowing(c.pan, c.col, c.per, c.cols); got != c.want {
			t.Errorf("PanShowing(%v, %d, %d, %d) = %v, want %v", c.pan, c.col, c.per, c.cols, got, c.want)
		}
	}
	for _, c := range []struct{ from, to, f, want float64 }{
		{7, 1, 1, 1}, {7, 1, 0.5, 0}, {1, 7, 0.5, 0}, {0, 3, 0, 0}, {0, 3, 2, 3},
	} {
		if got := SlidePan(c.from, c.to, c.f, 8); got != c.want {
			t.Errorf("SlidePan(%v → %v, %v) = %v, want %v", c.from, c.to, c.f, got, c.want)
		}
	}
	for col := 0; col < 8; col++ {
		if got := PanAt(ResumeAt(col), 8); got != float64(col) || Sliding(ResumeAt(col)) {
			t.Errorf("PanAt(ResumeAt(%d)) = %v, sliding %v; want resting on %d", col, got, Sliding(ResumeAt(col)), col)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/scene/`
Expected: FAIL, with build errors such as `unknown field Selected in struct literal of type View` and `undefined: PlotAt`.

- [ ] **Step 3: Geometry, hit testing and the camera helpers**

`internal/scene/select.go`:

```go
package scene

import (
	"math"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// geometry is where a View's beds and plots land. Draw, PlotAt, OnScreen
// and the card's placement all use it, so what you click is what was drawn.
type geometry struct {
	lay  Layout
	cols int
	h    int // canvas height in pixels
	top  int // pixel row of bed 0's top; negative when a short window crops it
}

func geometryOf(v View) geometry {
	cols := max(v.Cols, 1)
	lay := LayoutFor(cols, v.Rows, len(v.Plots))
	h := lay.Beds * bedPx
	if v.Rows > 0 {
		h = v.Rows * 2
	}
	return geometry{lay: lay, cols: cols, h: h, top: h - lay.Beds*bedPx}
}

// bedTop is the pixel row where bed b starts.
func (g geometry) bedTop(b int) int { return g.top + b*bedPx }

// slots lists the plots bed b shows and the columns they're centered on.
func (g geometry) slots(v View, b int) []slot { return slots(g.lay, b, len(v.Plots), g.cols, v.Pan) }

// slotLeft is the first terminal column of the slot centered on cx.
func slotLeft(cx int) int { return cx - BedCols/2 }

// PlotAt is the plot drawn at terminal cell (col, row) of v's frame: the
// whole column of its slot, from the sky down to its labels. The top bed's
// sky reaches the top of the frame. ok is false on open ground beside the
// slots and outside the frame.
func PlotAt(v View, col, row int) (index int, ok bool) {
	g := geometryOf(v)
	y := row * 2
	if col < 0 || col >= g.cols || y < 0 || y >= g.h {
		return 0, false
	}
	for b := 0; b < g.lay.Beds; b++ {
		oy := g.bedTop(b)
		if y >= oy+bedPx || (y < oy && b > 0) {
			continue
		}
		for _, s := range g.slots(v, b) {
			if left := slotLeft(s.cx); col >= left && col < left+BedCols {
				return s.index, true
			}
		}
		return 0, false
	}
	return 0, false
}

// OnScreen lists the plots whose whole slot is inside v's frame, in reading
// order: bed by bed, left to right.
func OnScreen(v View) []int {
	g := geometryOf(v)
	var out []int
	for b := 0; b < g.lay.Beds; b++ {
		for _, s := range g.slots(v, b) {
			if left := slotLeft(s.cx); left >= 0 && left+BedCols <= g.cols {
				out = append(out, s.index)
			}
		}
	}
	return out
}

// ReadingOrder lists all n plots of a layout in reading order: along each
// row of beds, then the next. A panning garden's rows run across all its
// columns, not only the visible ones.
func ReadingOrder(lay Layout, n int) []int {
	if !lay.Overflow {
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	var out []int
	for b := 0; b < lay.Beds; b++ {
		for j := 0; j < lay.Columns; j++ {
			if i := j*lay.Beds + b; i < n {
				out = append(out, i)
			}
		}
	}
	return out
}

// SlideFor is how long the camera takes to slide to a new position.
const SlideFor = panSlide

// PanShowing is the camera position, in whole plot columns, nearest to pan
// at which column col is on screen, when perRow columns fit side by side in
// a garden of columns. A garden that fits returns 0.
func PanShowing(pan float64, col, perRow, columns int) float64 {
	if columns <= perRow {
		return 0
	}
	p := wrap(int(math.Round(pan)), columns)
	if wrap(col-p, columns) < perRow {
		return float64(p)
	}
	right := wrap(col-perRow+1, columns) // col becomes the rightmost column on screen
	left := col                          // col becomes the leftmost
	if ringDist(pan, float64(right), columns) <= ringDist(pan, float64(left), columns) {
		return float64(right)
	}
	return float64(left)
}

// SlidePan is the camera f of the way (0 to 1, eased) through a slide from
// from to to, the short way round a garden of columns.
func SlidePan(from, to, f float64, columns int) float64 {
	if columns <= 0 {
		return 0
	}
	n := float64(columns)
	d := math.Mod(to-from, n)
	switch {
	case d > n/2:
		d -= n
	case d < -n/2:
		d += n
	}
	f = math.Max(0, math.Min(1, f))
	pos := math.Mod(from+d*f*f*(3-2*f), n)
	if pos < 0 {
		pos += n
	}
	return pos
}

// ResumeAt is the time into automatic panning at which PanAt has just come
// to rest on column col. A camera handing back to automatic panning offsets
// its clock so that the moment it resumes is ResumeAt of its column.
func ResumeAt(col int) time.Duration { return time.Duration(col)*panHold + panSlide }

// wrap is a mod n, from 0 to n-1 even for negative a.
func wrap(a, n int) int { return (a%n + n) % n }

// ringDist is the distance between two positions on a ring of n columns.
func ringDist(a, b float64, n int) float64 {
	d := math.Abs(math.Mod(a-b, float64(n)))
	return math.Min(d, float64(n)-d)
}

// groundLit is the warm light on the selected plant's ground strip.
var groundLit = rgb(236, 196, 128)

// lightGround brightens pixel rows [y0, y1) of the slot centered on cx.
func lightGround(c *pixel.Canvas, cx, y0, y1 int) {
	for y := max(y0, 0); y < min(y1, c.H); y++ {
		for x := max(slotLeft(cx), 0); x < min(slotLeft(cx)+BedCols, c.W); x++ {
			c.Set(x, y, pixel.Lerp(c.At(x, y), groundLit, 0.35))
		}
	}
}
```

- [ ] **Step 4: Draw the selection**

In `internal/scene/draw.go`:

1. In `type View struct`, add after the `Shooting` field:

```go
	Selected   int        // 1 + the selected plot's index; 0, the zero value, selects none
```

2. In `Draw`, replace

```go
	cols := max(v.Cols, 1)
	lay := LayoutFor(cols, v.Rows, len(v.Plots))
	h := lay.Beds * bedPx
	if v.Rows > 0 {
		h = v.Rows * 2
	}
	c := pixel.New(cols, h)
	top := h - lay.Beds*bedPx // negative when the top is cropped
	var hosts []Host
	for b := 0; b < lay.Beds; b++ {
		oy := top + b*bedPx
```

with

```go
	g := geometryOf(v)
	c := pixel.New(g.cols, g.h)
	var hosts []Host
	for b := 0; b < g.lay.Beds; b++ {
		oy := g.bedTop(b)
```

3. In `Draw`, replace

```go
		for _, s := range slots(lay, b, len(v.Plots), cols, v.Pan) {
			pl := v.Plots[s.index]
			drawPlot(c, v, pl, s.cx, oy)
```

with

```go
		for _, s := range g.slots(v, b) {
			pl := v.Plots[s.index]
			selected := v.Selected == s.index+1
			if selected {
				lightGround(c, s.cx, oy+groundTop, oy+bedPx)
			}
			drawPlot(c, v, pl, s.cx, oy, selected)
```

In `internal/scene/compose.go`:

1. In the `var (` block that holds `nameColor`, add `	selectedLabel = rgb(255, 255, 255)`.
2. Change `func drawPlot(c *pixel.Canvas, v View, pl Plot, cx, oy int) {` to `func drawPlot(c *pixel.Canvas, v View, pl Plot, cx, oy int, selected bool) {`.
3. Replace `	label(c, cx, labelRow, pl.Name, nameColor)` with:

```go
	nameFg := nameColor
	if selected {
		nameFg = selectedLabel
	}
	label(c, cx, labelRow, pl.Name, nameFg)
```

- [ ] **Step 5: Run the tests**

Run: `gofmt -w internal/scene && go test ./internal/scene/ && go test ./...`
Expected: PASS, and `TestComposeGolden` passes unchanged.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/scene
git commit -m "scene: select a plant, find the plant under a click, and camera helpers

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Scene: the detail card

**Files:**
- Create: `internal/scene/card.go`, `internal/scene/card_test.go`
- Modify: `internal/scene/draw.go` (`View.Card`; `Draw` draws the card last)

**Interfaces:**
- Consumes: Task 2's `geometryOf`, `(geometry).bedTop`, `(geometry).slots`, `slotLeft` and `View.Selected`; `pixel.Canvas.Rect` and `Text`; the test helpers `demoPlots()`, `manyPlots(n)`, `at(h, m)` and `visible(s)`.
- Produces:
  - `type Card struct{ Title, Note string; Lines []CardLine }`
  - `type CardLine struct{ Label string; Sub bool; Value, Right string; Tone Tone }`
  - `type Tone int` with `TonePlain`, `ToneGood`, `ToneBad` and `ToneGold`
  - `View.Card *Card`
  - `func CardRect(v View) (x, y, w, h int, ok bool)`

- [ ] **Step 1: Write the failing tests**

`internal/scene/card_test.go`:

```go
package scene

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func sampleCard() *Card {
	return &Card{Title: "gag-core", Note: "Go · shrub", Lines: []CardLine{
		{Value: "RursusAeternum/gag-core", Right: "★ 31"},
		{Label: "state", Value: "thriving · tended 2h ago"},
		{Label: "last", Value: "Make storms readable, keep CI errors local"},
		{Label: "CI", Value: "✓ passing on main", Tone: ToneGood},
		{Label: "PRs", Value: "2 open · oldest 10d"},
		{Sub: true, Value: "#12 Add sparkline", Right: "2d"},
		{Label: "release", Value: "v0.5.0 · 1h ago", Tone: ToneGold},
	}}
}

// frameLines is v's frame as plain text rows.
func frameLines(v View) []string {
	return strings.Split(visible(Draw(v).Encode(pixel.TrueColor)), "\n")
}

func TestCardStaysInsideTheWindow(t *testing.T) {
	for _, size := range [][2]int{{80, 23}, {24, 11}, {240, 64}} {
		plots := manyPlots(8)
		for i := range plots {
			v := View{Cols: size[0], Rows: size[1], Plots: plots, Now: at(14, 0), Seed: 1, Selected: i + 1, Card: sampleCard()}
			x, y, w, h, ok := CardRect(v)
			if !ok || x < 0 || y < 0 || x+w > size[0] || y+h > size[1] || w < 6 || h < 3 {
				t.Fatalf("%dx%d, plot %d: card at %d,%d size %dx%d", size[0], size[1], i, x, y, w, h)
			}
			lines := frameLines(v)
			if len(lines) != size[1] {
				t.Fatalf("%dx%d: %d rows", size[0], size[1], len(lines))
			}
			for n, l := range lines {
				if got := runewidth.StringWidth(l); got != size[0] {
					t.Fatalf("%dx%d, plot %d: row %d is %d wide", size[0], size[1], i, n, got)
				}
			}
			top, bottom := []rune(lines[y]), []rune(lines[y+h-1])
			if top[x] != '┌' || top[x+w-1] != '┐' || bottom[x] != '└' || bottom[x+w-1] != '┘' {
				t.Fatalf("%dx%d, plot %d: no box at %d,%d %dx%d:\n%s", size[0], size[1], i, x, y, w, h, strings.Join(lines, "\n"))
			}
		}
	}
}

func TestCardSitsBesideThePlant(t *testing.T) {
	v := View{Cols: 120, Plots: demoPlots(), Now: at(14, 0), Seed: 1, Card: sampleCard()} // slots 21-46, 47-72, 73-98
	v.Selected = 1
	if x, _, _, _, _ := CardRect(v); x < 47 {
		t.Errorf("the first plant's card starts at column %d, over the plant", x)
	}
	v.Selected = 3
	if x, _, w, _, _ := CardRect(v); x+w > 73 {
		t.Errorf("the last plant's card ends at column %d, over the plant", x+w)
	}
	v.Cols, v.Selected = 80, 2 // neither side of the middle plant has room
	if x, _, w, _, _ := CardRect(v); x != (80-w)/2 {
		t.Errorf("the card should be centred when no side fits; it starts at column %d", x)
	}
	two := View{Cols: 3 * BedCols, Plots: manyPlots(5), Now: at(14, 0), Seed: 1, Card: sampleCard(), Selected: 4}
	if _, y, _, _, _ := CardRect(two); y != BedRows {
		t.Errorf("a card for a plant in the second bed starts at row %d, want %d", y, BedRows)
	}
}

func TestCardTextIsOneColumnWide(t *testing.T) {
	card := &Card{Title: "日本語のリポジトリ🌱", Note: "Go · shrub", Lines: []CardLine{
		{Label: "last", Value: "fix:\tthe 🐛 in 中文 \x1b[31mred\x1b[0m"},
		{Label: "PRs", Value: strings.Repeat("a very long title ", 10), Right: "10d"},
		{Label: "a-very-long-label", Value: "x", Right: strings.Repeat("r", 60)},
	}}
	v := View{Cols: 80, Rows: 23, Plots: demoPlots(), Now: at(14, 0), Seed: 1, Selected: 1, Card: card}
	for n, l := range frameLines(v) {
		for _, r := range l {
			if runewidth.RuneWidth(r) != 1 {
				t.Fatalf("row %d holds %q, %d columns wide", n, r, runewidth.RuneWidth(r))
			}
		}
		if w := runewidth.StringWidth(l); w != 80 {
			t.Fatalf("row %d is %d wide", n, w)
		}
	}
}

func TestTallCardsAreCut(t *testing.T) {
	card := &Card{Title: "t"}
	for i := 0; i < 30; i++ {
		card.Lines = append(card.Lines, CardLine{Label: "x", Value: "y"})
	}
	v := View{Cols: 80, Rows: 11, Plots: demoPlots(), Now: at(14, 0), Seed: 1, Selected: 1, Card: card}
	if _, y, _, h, _ := CardRect(v); y != 0 || h != 11 {
		t.Errorf("a card taller than the window is at row %d, %d tall; want 0 and 11", y, h)
	}
	if lines := frameLines(v); !strings.Contains(lines[10], "└") {
		t.Error("a cut card keeps its bottom border")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/scene/`
Expected: FAIL, with build errors such as `undefined: Card` and `unknown field Card in struct literal of type View`.

- [ ] **Step 3: The card**

`internal/scene/card.go`:

```go
package scene

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// Card is a detail card: a title row over labelled lines, drawn in a box
// beside the selected plant.
type Card struct {
	Title, Note string // the title row's left and right: the repo, its language and species
	Lines       []CardLine
}

// CardLine is one row of a card.
type CardLine struct {
	Label string // the label column; "" on a line without one
	Sub   bool   // indented to the value column, continuing the line above
	Value string
	Right string // right-aligned after the value, e.g. an age
	Tone  Tone   // the value's colour
}

// Tone colours a card value.
type Tone int

const (
	TonePlain Tone = iota
	ToneGood       // CI passing
	ToneBad        // CI failing
	ToneGold       // the latest release
)

const (
	cardMaxW   = 38 // columns
	cardLabelW = 9  // the label column
)

var (
	cardBG     = rgb(28, 26, 23) // the ticker's background
	cardBorder = rgb(110, 104, 94)
	cardLabel  = rgb(150, 142, 128)
	cardText   = rgb(226, 220, 206)
	cardGood   = rgb(130, 210, 120)
	cardBad    = rgb(235, 100, 90)
	cardGold   = rgb(240, 200, 90)
)

func (t Tone) color() pixel.RGB {
	switch t {
	case ToneGood:
		return cardGood
	case ToneBad:
		return cardBad
	case ToneGold:
		return cardGold
	}
	return cardText
}

// CardRect is where v's card is drawn, in terminal cells. It sits beside the
// selected plant's slot on the side with more room, level with the top of
// its row of beds, and is centred when neither side fits. It always stays
// inside the frame. ok is false when v has no card.
func CardRect(v View) (x, y, w, h int, ok bool) {
	if v.Card == nil {
		return 0, 0, 0, 0, false
	}
	g := geometryOf(v)
	rows := g.h / 2
	w = max(6, min(cardMaxW, g.cols-2))
	h = min(len(v.Card.Lines)+2, rows)
	x = (g.cols - w) / 2
	for b := 0; b < g.lay.Beds; b++ {
		for _, s := range g.slots(v, b) {
			if s.index+1 != v.Selected {
				continue
			}
			left, right := slotLeft(s.cx), slotLeft(s.cx)+BedCols
			roomL, roomR := left, g.cols-right
			switch {
			case roomR >= w && roomR >= roomL:
				x = right
			case roomL >= w:
				x = left - w
			}
			y = max(0, g.bedTop(b)/2)
		}
	}
	x = max(0, min(x, g.cols-w))
	y = max(0, min(y, rows-h))
	return x, y, w, h, true
}

// drawCard draws v's card over the frame, at CardRect.
func drawCard(c *pixel.Canvas, v View) {
	x, y, w, h, ok := CardRect(v)
	if !ok || h < 2 {
		return
	}
	c.Rect(x, 2*y, w, 2*h, cardBG)
	room := w - 4
	note := cut(cardRunes(v.Card.Note), room/2)
	noteText := ""
	if len(note) > 0 {
		room -= len(note) + 1 // a space before the note
		noteText = " " + string(note)
	}
	title := cut(cardRunes(v.Card.Title), max(0, room-2)) // then a space and at least one dash
	cardRow(c, x, y,
		seg{"┌ ", cardBorder}, seg{string(title), cardText},
		seg{" " + strings.Repeat("─", max(0, room-len(title)-1)), cardBorder},
		seg{noteText, cardLabel}, seg{" ┐", cardBorder})
	lines := v.Card.Lines
	if len(lines) > h-2 {
		lines = lines[:h-2]
	}
	for i, ln := range lines {
		drawCardLine(c, x, y+1+i, w, ln)
	}
	c.Text(x, y+h-1, "└"+strings.Repeat("─", w-2)+"┘", cardBorder)
}

// drawCardLine draws one line of a w-wide card on terminal row row.
func drawCardLine(c *pixel.Canvas, x, row, w int, ln CardLine) {
	room := w - 4
	var label []rune
	if ln.Label != "" || ln.Sub {
		n := min(cardLabelW, room)
		label = pad(cut(cardRunes(ln.Label), n-1), n)
	}
	room -= len(label)
	right := cut(cardRunes(ln.Right), room)
	vroom := room - len(right)
	if len(right) > 0 {
		vroom-- // a space before the right part
	}
	value := cut(cardRunes(ln.Value), max(0, vroom))
	cardRow(c, x, row,
		seg{"│ ", cardBorder}, seg{string(label), cardLabel}, seg{string(value), ln.Tone.color()},
		seg{strings.Repeat(" ", max(0, room-len(value)-len(right))), cardText},
		seg{string(right), cardText}, seg{" │", cardBorder})
}

// seg is a run of card text in one colour.
type seg struct {
	text string
	col  pixel.RGB
}

// cardRow writes segs one after another along terminal row row, from
// column x.
func cardRow(c *pixel.Canvas, x, row int, segs ...seg) {
	for _, s := range segs {
		c.Text(x, row, s.text, s.col)
		x += utf8.RuneCountInString(s.text)
	}
}

// cardRunes cleans text for the card. Control characters go (a tab becomes
// a space), and runes that don't take exactly one terminal column become
// '?', so they can't shift the row.
func cardRunes(s string) []rune {
	var out []rune
	for _, r := range s {
		switch {
		case r == '\t':
			out = append(out, ' ')
		case unicode.IsControl(r):
		case runewidth.RuneWidth(r) != 1:
			out = append(out, '?')
		default:
			out = append(out, r)
		}
	}
	return out
}

// cut shortens r to n runes, ending in '…' when it had to cut.
func cut(r []rune, n int) []rune {
	if len(r) <= n {
		return r
	}
	if n <= 0 {
		return nil
	}
	return append(r[:n-1:n-1], '…')
}

// pad fills r with spaces up to n runes.
func pad(r []rune, n int) []rune {
	for len(r) < n {
		r = append(r, ' ')
	}
	return r
}
```

In `internal/scene/draw.go`:

1. In `type View struct`, add after `Selected`:

```go
	Card       *Card      // the selected plot's detail card; nil for none
```

2. At the end of `Draw`, replace

```go
	if v.Motion {
		DrawCritters(c, v.Now, hosts, v.Seed)
	}
	return c
}
```

with

```go
	if v.Motion {
		DrawCritters(c, v.Now, hosts, v.Seed)
	}
	if v.Card != nil {
		drawCard(c, v)
	}
	return c
}
```

- [ ] **Step 4: Run the tests**

Run: `gofmt -w internal/scene && go test ./internal/scene/ && go test ./...`
Expected: PASS, and the golden frame is unchanged.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/scene
git commit -m "scene: the detail card, beside the selected plant

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Live: `CardFor`, the card's lines

**Files:**
- Create: `internal/live/card.go`, `internal/live/card_test.go`

**Interfaces:**
- Consumes:
  - Task 1's `Detail` and `Entry`, and Task 3's `scene.Card`, `scene.CardLine`, `scene.ToneGood`, `scene.ToneBad` and `scene.ToneGold`.
  - From v0.5: `ago(d)`, `plural(n, noun)`, `wiltThreshold`, `garden.Health`, and `Repo`'s `Name`, `Plant`, `Finished`, `Branch`, `CI` and `Stars`.
  - The test helpers `grow`, `t0`.
- Produces:
  - `func CardFor(r Repo, now time.Time, decay float64, maxRows int) scene.Card`
  - `func Sparkline(days [14]int) string`

- [ ] **Step 1: Write the failing tests**

`internal/live/card_test.go`:

```go
package live

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func fullRepo() Repo {
	r := grow("gag-core", 60, 4, t0.Add(-2*time.Hour))
	r.Branch, r.CI, r.Stars = "main", CIPassing, 31
	r.Detail = Detail{FullName: "RursusAeternum/gag-core", Language: "Go",
		LastCommit: Entry{Title: "Make storms readable", At: t0.Add(-2 * time.Hour)},
		Daily:      [14]int{0, 1, 2, 4, 1, 0, 0, 1, 3, 4, 1, 0, 2, 4},
		PRs: []Entry{
			{Number: 12, Title: "Add sparkline", At: t0.Add(-2 * 24 * time.Hour)},
			{Number: 9, Title: "Bump deps", At: t0.Add(-10 * 24 * time.Hour)},
			{Number: 15, Title: "Fix typo", At: t0.Add(-time.Hour)},
		},
		Drafts: 1, OpenIssues: 4,
		NewestIssue: Entry{Number: 31, Title: "Crash on empty repo", At: t0.Add(-2 * 24 * time.Hour)},
		Release:     Entry{Title: "v0.5.0", At: t0.Add(-time.Hour)}}
	return r
}

func TestCardForAFullRepo(t *testing.T) {
	c := CardFor(fullRepo(), t0, 45, 40)
	if c.Title != "gag-core" || c.Note != "Go · shrub" {
		t.Errorf("title row = %q / %q", c.Title, c.Note)
	}
	want := []scene.CardLine{
		{Value: "RursusAeternum/gag-core", Right: "★ 31"},
		{Label: "state", Value: "thriving · tended 2h ago"},
		{Label: "last", Value: "Make storms readable"},
		{Label: "14 days", Value: "·▂▄█▂··▂▆█▂·▄█ 23 commits"},
		{Label: "CI", Value: "✓ passing on main", Tone: scene.ToneGood},
		{Label: "PRs", Value: "3 open · oldest 10d · +1 draft"},
		{Sub: true, Value: "#9 Bump deps", Right: "10d"},
		{Sub: true, Value: "#12 Add sparkline", Right: "2d"},
		{Sub: true, Value: "+1 more"},
		{Label: "issues", Value: "4 open · newest 2d ago"},
		{Sub: true, Value: "#31 Crash on empty repo"},
		{Label: "release", Value: "v0.5.0 · 1h ago", Tone: scene.ToneGold},
	}
	if !reflect.DeepEqual(c.Lines, want) {
		t.Errorf("lines:\n%+v\nwant\n%+v", c.Lines, want)
	}
}

func TestCardForSaysWhenThereIsNothing(t *testing.T) {
	c := CardFor(grow("seed", 1, 0, t0.Add(-time.Hour)), t0, 45, 40)
	want := []scene.CardLine{
		{Value: "seed"},
		{Label: "state", Value: "thriving · tended 1h ago"},
		{Label: "last", Value: "no commits yet"},
		{Label: "14 days", Value: "·············· 0 commits"},
		{Label: "CI", Value: "– no checks"},
		{Label: "PRs", Value: "none open"},
		{Label: "issues", Value: "none open"},
		{Label: "release", Value: "none yet"},
	}
	if c.Note != "shrub" || !reflect.DeepEqual(c.Lines, want) {
		t.Errorf("note %q, lines:\n%+v\nwant\n%+v", c.Note, c.Lines, want)
	}
}

func TestCardStates(t *testing.T) {
	wilting := grow("quiet", 40, 0, t0.Add(-40*24*time.Hour))
	done := grow("old", 30, 0, t0.Add(-90*24*time.Hour))
	done.Finished = true
	fresh := Repo{Name: "new", Plant: garden.Grow("new", garden.Shrub, nil)}
	failing := grow("red", 10, 0, t0)
	failing.CI, failing.Branch = CIFailing, "main"
	for _, c := range []struct {
		r    Repo
		line int
		want scene.CardLine
	}{
		{wilting, 1, scene.CardLine{Label: "state", Value: "wilting · 40d quiet"}},
		{done, 1, scene.CardLine{Label: "state", Value: "under glass · tended 90d ago"}},
		{fresh, 1, scene.CardLine{Label: "state", Value: "no activity yet"}},
		{failing, 4, scene.CardLine{Label: "CI", Value: "✗ failing on main", Tone: scene.ToneBad}},
	} {
		if got := CardFor(c.r, t0, 45, 40).Lines[c.line]; got != c.want {
			t.Errorf("%s: %+v, want %+v", c.r.Name, got, c.want)
		}
	}
}

func TestCardDropsLinesInOrder(t *testing.T) {
	labels := func(c scene.Card) string {
		var s []string
		for _, l := range c.Lines {
			switch {
			case l.Sub:
				s = append(s, "+")
			case l.Label == "":
				s = append(s, "name")
			default:
				s = append(s, l.Label)
			}
		}
		return strings.Join(s, " ")
	}
	for rows, want := range map[int]string{
		14: "name state last 14 days CI PRs + + + issues + release",
		13: "name state last 14 days CI PRs + + issues + release",
		11: "name state last 14 days CI PRs issues + release",
		10: "name state last 14 days CI PRs issues release",
		9:  "name state last 14 days CI PRs issues",
		8:  "name state last CI PRs issues",
		7:  "state last CI PRs issues",
		6:  "state CI PRs issues",
		5:  "state CI PRs",
		4:  "state CI",
		3:  "state CI",
	} {
		if got := labels(CardFor(fullRepo(), t0, 45, rows)); got != want {
			t.Errorf("%d rows: %s\nwant %s", rows, got, want)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/live/`
Expected: FAIL (build error `undefined: CardFor`).

- [ ] **Step 3: Implement**

`internal/live/card.go`:

```go
package live

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

// How a card sheds lines when the window is too short: the highest rank goes
// first, its last line first. Lines ranked keep always stay.
const (
	keep = iota
	dropPRs // the PR summary: dropped last
	dropIssues
	dropLast
	dropName // the owner/name line
	dropDays // the 14-day sparkline
	dropRelease
	dropIssueTitle
	dropPRTitles // PR title lines and "+N more": dropped first
)

type rankedLine struct {
	scene.CardLine
	rank int
}

// CardFor is r's detail card at time now, at most maxRows terminal rows tall
// with its border. scene cleans and trims the text when it draws the card.
func CardFor(r Repo, now time.Time, decay float64, maxRows int) scene.Card {
	if r.Plant == nil {
		r.Plant = &garden.Plant{}
	}
	d := r.Detail
	card := scene.Card{Title: r.Name, Note: string(r.Plant.Species)}
	if d.Language != "" {
		card.Note = strings.TrimSuffix(d.Language+" · "+card.Note, " · ")
	}
	name := scene.CardLine{Value: d.FullName}
	if name.Value == "" {
		name.Value = r.Name
	}
	if r.Stars > 0 {
		name.Right = fmt.Sprintf("★ %d", r.Stars)
	}
	last := d.LastCommit.Title
	if last == "" {
		last = "no commits yet"
	}
	lines := []rankedLine{
		{name, dropName},
		{scene.CardLine{Label: "state", Value: stateOf(r, now, decay)}, keep},
		{scene.CardLine{Label: "last", Value: last}, dropLast},
		{scene.CardLine{Label: "14 days", Value: Sparkline(d.Daily) + " " + plural(total(d.Daily), "commit")}, dropDays},
		{ciLine(r), keep},
	}
	lines = append(lines, prLines(d, now)...)
	lines = append(lines, issueLines(d, now)...)
	rel := scene.CardLine{Label: "release", Value: "none yet"}
	if d.Release.Title != "" {
		rel.Value, rel.Tone = d.Release.Title+" · "+ago(now.Sub(d.Release.At)), scene.ToneGold
	}
	lines = append(lines, rankedLine{rel, dropRelease})
	for len(lines)+2 > maxRows && drop(&lines) {
	}
	for _, l := range lines {
		card.Lines = append(card.Lines, l.CardLine)
	}
	return card
}

// stateOf is the card's state line: thriving, wilting or under glass.
func stateOf(r Repo, now time.Time, decay float64) string {
	p := r.Plant
	switch {
	case r.Finished && p.LastTended.IsZero():
		return "under glass"
	case r.Finished:
		return "under glass · tended " + ago(now.Sub(p.LastTended))
	case p.LastTended.IsZero():
		return "no activity yet"
	case garden.Health(p, now, decay) < wiltThreshold:
		return fmt.Sprintf("wilting · %dd quiet", int(now.Sub(p.LastTended).Hours()/24))
	}
	return "thriving · tended " + ago(now.Sub(p.LastTended))
}

func ciLine(r Repo) scene.CardLine {
	on := ""
	if r.Branch != "" {
		on = " on " + r.Branch
	}
	l := scene.CardLine{Label: "CI"}
	switch r.CI {
	case CIPassing:
		l.Value, l.Tone = "✓ passing"+on, scene.ToneGood
	case CIFailing:
		l.Value, l.Tone = "✗ failing"+on, scene.ToneBad
	case CIPending:
		l.Value = "● running" + on
	default:
		l.Value = "– no checks"
	}
	return l
}

// prLines are the PR summary and up to two PRs, oldest first.
func prLines(d Detail, now time.Time) []rankedLine {
	prs := slices.Clone(d.PRs)
	sort.SliceStable(prs, func(i, j int) bool { return prs[i].At.Before(prs[j].At) })
	sum := scene.CardLine{Label: "PRs", Value: "none open"}
	if len(prs) > 0 {
		sum.Value = fmt.Sprintf("%d open · oldest %s", len(prs), age(now.Sub(prs[0].At)))
	}
	if d.Drafts > 0 {
		sum.Value += fmt.Sprintf(" · +%d draft", d.Drafts)
	}
	out := []rankedLine{{sum, dropPRs}}
	for i, pr := range prs {
		if i == 2 {
			out = append(out, rankedLine{scene.CardLine{Sub: true, Value: fmt.Sprintf("+%d more", len(prs)-2)}, dropPRTitles})
			break
		}
		out = append(out, rankedLine{scene.CardLine{Sub: true, Value: fmt.Sprintf("#%d %s", pr.Number, pr.Title),
			Right: age(now.Sub(pr.At))}, dropPRTitles})
	}
	return out
}

// issueLines are the issue summary and the newest open issue.
func issueLines(d Detail, now time.Time) []rankedLine {
	l := scene.CardLine{Label: "issues", Value: "none open"}
	if d.OpenIssues == 0 {
		return []rankedLine{{l, dropIssues}}
	}
	l.Value = fmt.Sprintf("%d open", d.OpenIssues)
	if d.NewestIssue.Number == 0 {
		return []rankedLine{{l, dropIssues}}
	}
	l.Value += " · newest " + ago(now.Sub(d.NewestIssue.At))
	title := scene.CardLine{Sub: true, Value: fmt.Sprintf("#%d %s", d.NewestIssue.Number, d.NewestIssue.Title)}
	return []rankedLine{{l, dropIssues}, {title, dropIssueTitle}}
}

// drop removes the last of the highest-ranked lines that may go, and
// reports false when only lines that always stay are left.
func drop(lines *[]rankedLine) bool {
	top, at := keep, -1
	for i, l := range *lines {
		if l.rank > keep && l.rank >= top {
			top, at = l.rank, i
		}
	}
	if at < 0 {
		return false
	}
	*lines = append((*lines)[:at], (*lines)[at+1:]...)
	return true
}

// Sparkline draws daily counts as block heights scaled to the busiest day;
// a day without commits is a dot.
func Sparkline(days [14]int) string {
	blocks := []rune("▁▂▃▄▅▆▇█")
	top := 0
	for _, n := range days {
		top = max(top, n)
	}
	var b strings.Builder
	for _, n := range days {
		if n <= 0 {
			b.WriteRune('·')
			continue
		}
		b.WriteRune(blocks[(n*len(blocks)-1)/top])
	}
	return b.String()
}

func total(days [14]int) int {
	n := 0
	for _, d := range days {
		n += d
	}
	return n
}

// age is how long ago, in the ticker's short form: 2h, 10d, just now.
func age(d time.Duration) string { return strings.TrimSuffix(ago(d), " ago") }
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/live/ && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/live
git commit -m "live: the detail card's lines, and which go first when space is short

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Live: selection, the mouse, the card in the frame

**Files:**
- Create: `internal/live/select.go`, `internal/live/select_test.go`
- Modify: `internal/live/model.go` (fields, keys, mouse, tick, resize, refresh, `sceneView`, `View`, `busy`, help line), `cmd/gag/garden.go` (mouse on), `README.md` (keys)

**Interfaces:**
- Consumes:
  - Task 2's `scene.PlotAt`, `scene.OnScreen`, `scene.ReadingOrder`, `scene.PanShowing`, `scene.SlidePan`, `scene.ResumeAt`, `scene.SlideFor` and `View.Selected`.
  - Task 3's `scene.CardRect` and `View.Card`, and Task 4's `CardFor`.
  - Bubble Tea's `tea.MouseMsg{X, Y, Button, Action}`, `tea.MouseButtonLeft`, `tea.MouseActionPress`, `tea.WithMouseCellMotion()`, and key strings `"left"`, `"right"`, `"enter"` and `"esc"`.
  - The test helpers `ready`, `newModel`, `step`, `garden3`, `grow`, `checkSize`, `visible` and `t0`.
- Produces:
  - `Model.sel selection`, `Model.cam camera`
  - `(Model).selected() int`, `(Model).sceneView(at time.Time) scene.View`, `(Model).pan(scene.Layout) (float64, bool)` and `(Model).layout() scene.Layout`
  - `const idleClear = time.Minute`

- [ ] **Step 1: Write the failing tests**

`internal/live/select_test.go`:

```go
package live

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func arrow(k tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: k} }

func click(col, row int) tea.MouseMsg {
	return tea.MouseMsg{X: col, Y: row, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
}

// eight is a garden that pans at 80×24: three plants fit side by side.
func eight() Snapshot {
	var s Snapshot
	for i := 0; i < 8; i++ {
		s.Repos = append(s.Repos, grow(fmt.Sprintf("p%d", i), 20, 0, t0.Add(-time.Hour)))
	}
	s.FetchedAt = t0
	return s
}

func clocked(snap Snapshot, now *time.Time) Model {
	return New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return snap, nil },
		DecayDays: 45,
		Now:       func() time.Time { return *now },
	})
}

// In garden3 at 100×30 the slots are columns 11-36 (bloom), 37-62 (quiet)
// and 63-88 (seed); the bed fills rows 5-28 and the ticker is row 29.

func TestArrowsStepThroughThePlants(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	var got []string
	for _, k := range []tea.KeyType{tea.KeyRight, tea.KeyRight, tea.KeyRight, tea.KeyRight, tea.KeyLeft, tea.KeyLeft} {
		m, _ = step(m, arrow(k))
		got = append(got, m.sel.name)
	}
	if want := "bloom quiet seed bloom seed quiet"; strings.Join(got, " ") != want {
		t.Errorf("→→→→←← selected %q, want %q", strings.Join(got, " "), want)
	}
	m = ready(newModel(garden3(), nil, t0), 100, 30)
	if m, _ = step(m, arrow(tea.KeyLeft)); m.sel.name != "seed" {
		t.Errorf("← with nothing selected picked %q, want the last plant on screen", m.sel.name)
	}
}

func TestClicksSelectAndClear(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	for _, c := range []struct {
		col, row int
		want     string
	}{
		{24, 15, "bloom"}, {50, 2, "quiet"}, {80, 28, "seed"}, // a plant, the sky above the bed, a label
		{5, 15, ""},                  // open ground beside the plants
		{50, 15, "quiet"}, {50, 29, ""}, // the ticker
	} {
		m, _ = step(m, click(c.col, c.row))
		if m.sel.name != c.want {
			t.Errorf("click at %d,%d selected %q, want %q", c.col, c.row, m.sel.name, c.want)
		}
	}
	m, _ = step(m, tea.MouseMsg{X: 24, Y: 15, Button: tea.MouseButtonRight, Action: tea.MouseActionPress})
	if m.sel.name != "" {
		t.Error("a right click should do nothing")
	}
}

func TestEnterOpensTheCardAndEscBacksOut(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	if m, _ = step(m, arrow(tea.KeyEnter)); m.sel.card {
		t.Error("enter with nothing selected opened a card")
	}
	m, _ = step(m, click(24, 15))
	m, _ = step(m, arrow(tea.KeyEnter))
	if !strings.Contains(visible(m.View()), "┌ bloom") {
		t.Fatalf("enter should open bloom's card:\n%s", visible(m.View()))
	}
	checkSize(t, m.View(), 100, 30)
	m, _ = step(m, arrow(tea.KeyRight))
	if !strings.Contains(visible(m.View()), "┌ quiet") {
		t.Error("→ should move the open card to quiet")
	}
	m, cmd := step(m, arrow(tea.KeyEscape))
	if cmd != nil || m.sel.card || m.sel.name != "quiet" {
		t.Errorf("the first esc should only close the card: card %v, selected %q", m.sel.card, m.sel.name)
	}
	m, cmd = step(m, arrow(tea.KeyEscape))
	if cmd != nil || m.sel.name != "" {
		t.Errorf("the second esc should only clear the selection, selected %q", m.sel.name)
	}
	_, cmd = step(m, arrow(tea.KeyEscape))
	if cmd == nil {
		t.Fatal("esc with nothing selected should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("esc should send tea.QuitMsg")
	}
}

func TestClickOnTheCardDoesNothing(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	m, _ = step(m, click(24, 15))
	m, _ = step(m, arrow(tea.KeyEnter))
	x, y, _, _, ok := scene.CardRect(m.sceneView(m.now()))
	if !ok {
		t.Fatal("no card on screen")
	}
	m, _ = step(m, click(x+2, y+1))
	if m.sel.name != "bloom" || !m.sel.card {
		t.Errorf("a click on the card changed the selection to %q, card %v", m.sel.name, m.sel.card)
	}
}

func TestSelectionClearsAfterAMinute(t *testing.T) {
	now := t0
	m := ready(clocked(garden3(), &now), 100, 30)
	m, _ = step(m, click(24, 15))
	m, _ = step(m, arrow(tea.KeyEnter))
	now = now.Add(59 * time.Second)
	m, _ = step(m, tickMsg{})
	if m.sel.name != "bloom" || !m.sel.card {
		t.Fatal("the selection cleared before a minute was up")
	}
	now = now.Add(2 * time.Second)
	m, _ = step(m, tickMsg{})
	if m.sel.name != "" || m.sel.card {
		t.Error("a minute without input should clear the selection and close the card")
	}
}

func TestSelectionFollowsItsRepo(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	m, _ = step(m, click(50, 15)) // quiet
	more := garden3()
	more.Repos = append([]Repo{grow("newcomer", 5, 0, t0)}, more.Repos...)
	m, _ = step(m, loadedMsg{snap: more})
	if i := m.selected(); i < 0 || m.repos[i].Name != "quiet" {
		t.Errorf("after a refresh the selection is %d, want quiet", i)
	}
	gone := garden3()
	gone.Repos = append(gone.Repos[:1], gone.Repos[2:]...)
	m, _ = step(m, loadedMsg{snap: gone})
	if m.sel.name != "" {
		t.Error("the selection should clear when its repo leaves the garden")
	}
}

func TestCameraHoldsOnTheSelection(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	panNow := func() float64 { p, _ := m.pan(m.layout()); return p }
	m, _ = step(m, arrow(tea.KeyRight)) // p0, on screen
	m, _ = step(m, arrow(tea.KeyLeft))  // wraps to p7, off screen
	if m.sel.name != "p7" {
		t.Fatalf("selected %q, want p7", m.sel.name)
	}
	if m.frameInterval() != fastFrame {
		t.Error("the camera's slide needs the fast frame rate")
	}
	now = now.Add(scene.SlideFor)
	if p := panNow(); p != 7 {
		t.Errorf("after the slide the camera is at %v, want 7 (p7 on the left)", p)
	}
	now = now.Add(50 * time.Second) // longer than an automatic pan's hold
	if p := panNow(); p != 7 {
		t.Errorf("a selection should hold the camera; it moved to %v", p)
	}
	m, _ = step(m, arrow(tea.KeyEscape))
	if p := panNow(); p != 7 {
		t.Errorf("automatic panning should resume from 7, not jump to %v", p)
	}
	now = now.Add(20 * time.Second)
	if p := panNow(); p != 7 {
		t.Errorf("20 s after resuming the camera is at %v; want it still resting on 7", p)
	}
	now = now.Add(15 * time.Second)
	if p := panNow(); p == 7 {
		t.Error("automatic panning should carry on after the hold")
	}
}

func TestFrameFitsWithACardOpen(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {24, 12}, {240, 65}} {
		m := ready(newModel(eight(), nil, t0), size[0], size[1])
		m, _ = step(m, arrow(tea.KeyRight))
		m, _ = step(m, arrow(tea.KeyEnter))
		if !m.sel.card {
			t.Fatalf("%dx%d: no card open", size[0], size[1])
		}
		checkSize(t, m.View(), size[0], size[1])
	}
}

func TestResizeKeepsTheSelection(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	m, _ = step(m, arrow(tea.KeyLeft)) // the last plant on screen: p2
	m, _ = step(m, arrow(tea.KeyEnter))
	for _, size := range [][2]int{{240, 65}, {24, 12}, {80, 24}} {
		m, _ = step(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		now = now.Add(scene.SlideFor)
		checkSize(t, m.View(), size[0], size[1])
		if m.sel.name != "p2" || !m.sel.card {
			t.Fatalf("%dx%d: selection %q, card %v", size[0], size[1], m.sel.name, m.sel.card)
		}
	}
	if on := scene.OnScreen(m.sceneView(m.now())); !slices.Contains(on, 2) {
		t.Errorf("back at 80×24 the plants on screen are %v; want p2 among them", on)
	}
}

func TestHelpShowsEveryKey(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 80, 24)
	m, _ = step(m, key("?"))
	lines := strings.Split(m.View(), "\n")
	last := visible(lines[len(lines)-1])
	for _, k := range []string{"←/→ select", "enter details", "r refresh", "t ticker", "? help", "q quit"} {
		if !strings.Contains(last, k) {
			t.Errorf("the help line %q lacks %q", last, k)
		}
	}
}

func TestInputBeforeTheGardenLoads(t *testing.T) {
	m := New(Config{Now: func() time.Time { return t0 }})
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	for _, msg := range []tea.Msg{arrow(tea.KeyRight), arrow(tea.KeyLeft), arrow(tea.KeyEnter), click(10, 10)} {
		m, _ = step(m, msg)
	}
	if m.sel.name != "" || m.sel.card {
		t.Errorf("input on the loading screen selected %q", m.sel.name)
	}
	checkSize(t, m.View(), 80, 24)
}

func TestLongSleepClosesTheCard(t *testing.T) {
	now := t0
	m := ready(clocked(garden3(), &now), 100, 30)
	m, _ = step(m, click(24, 15))
	m, _ = step(m, arrow(tea.KeyEnter))
	now = now.Add(10 * time.Hour) // the laptop slept with the card open
	m, _ = step(m, tickMsg{})
	if m.sel.name != "" || m.sel.card {
		t.Error("after a long sleep the card should be closed and the selection gone")
	}
	checkSize(t, m.View(), 100, 30)
}

func BenchmarkLiveFrameWithCard(b *testing.B) {
	snap := garden3()
	for i := 0; len(snap.Repos) < 12; i++ {
		r := snap.Repos[i%3]
		r.Name += strings.Repeat("x", i+1)
		snap.Repos = append(snap.Repos, r)
	}
	m := ready(newModel(snap, nil, t0), 240, 65)
	m, _ = step(m, arrow(tea.KeyRight))
	m, _ = step(m, arrow(tea.KeyEnter))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/live/`
Expected: FAIL, with build errors such as `m.sel undefined` and `m.sceneView undefined`.

- [ ] **Step 3: Selection and the camera**

`internal/live/select.go`:

```go
package live

import (
	"math"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

// idleClear is how long a selection lasts without a key press or click.
const idleClear = time.Minute

// selection is the plant picked with the arrow keys or a click, by repo
// name, and whether its detail card is open.
type selection struct {
	name      string // "" for none
	card      bool
	lastInput time.Time // wall time of the last key press or click
}

// camera controls panning when the garden is wider than the window.
// Automatic panning runs on the session clock minus shift. A selection holds
// the camera, sliding it from → to when the selected plant is off screen.
type camera struct {
	shift     time.Duration
	held      bool
	from, to  float64
	slideFrom time.Time // wall time the hold's slide started
}

// selected is the index of the selected repo, or -1.
func (m Model) selected() int {
	if m.sel.name == "" {
		return -1
	}
	for i, r := range m.repos {
		if r.Name == m.sel.name {
			return i
		}
	}
	return -1
}

// choose selects repo i; an open card follows it.
func (m *Model) choose(i int) {
	m.sel.name = m.repos[i].Name
	m.holdOn(i)
}

// unselect clears the selection and closes the card, handing the camera
// back to automatic panning.
func (m *Model) unselect() {
	m.sel.name, m.sel.card = "", false
	m.release()
}

// stepSelection moves the selection dir (+1 or -1) plants in reading order,
// wrapping. With nothing selected it picks the first or last plant on
// screen.
func (m *Model) stepSelection(dir int) {
	if len(m.repos) == 0 || m.cols == 0 {
		return
	}
	order := scene.ReadingOrder(m.layout(), len(m.repos))
	cur := m.selected()
	if cur < 0 {
		on := scene.OnScreen(m.sceneView(m.now()))
		if len(on) == 0 {
			on = order
		}
		if dir > 0 {
			m.choose(on[0])
		} else {
			m.choose(on[len(on)-1])
		}
		return
	}
	for k, i := range order {
		if i == cur {
			m.choose(order[(k+dir+len(order))%len(order)])
			return
		}
	}
}

// click handles a left click at terminal cell (col, row). A plant selects
// it, the card ignores it, and anything else clears the selection.
func (m *Model) click(col, row int) {
	if len(m.repos) == 0 || m.cols == 0 {
		return
	}
	v := m.sceneView(m.now())
	if x, y, w, h, ok := scene.CardRect(v); ok && col >= x && col < x+w && row >= y && row < y+h {
		return
	}
	if row < m.gardenRows() {
		if i, ok := scene.PlotAt(v, col, row); ok {
			m.choose(i)
			return
		}
	}
	m.unselect()
}

// layout is how the garden fits the window right now.
func (m Model) layout() scene.Layout {
	return scene.LayoutFor(m.cols, m.gardenRows(), len(m.repos))
}

// pan is where the camera is now, and whether it's sliding or about to.
func (m Model) pan(lay scene.Layout) (float64, bool) {
	if !lay.Overflow {
		return 0, false
	}
	if !m.cam.held {
		e := m.elapsed() - m.cam.shift
		return scene.PanAt(e, lay.Columns), scene.Sliding(e) || scene.Sliding(e+slowFrame)
	}
	f := float64(m.cfg.Now().Sub(m.cam.slideFrom)) / float64(scene.SlideFor)
	if f >= 1 {
		return m.cam.to, false
	}
	return scene.SlidePan(m.cam.from, m.cam.to, f, lay.Columns), true
}

// holdOn stops automatic panning and, when plant i is off screen, slides the
// camera to the nearest position that shows it.
func (m *Model) holdOn(i int) {
	lay := m.layout()
	if !lay.Overflow {
		return
	}
	cur, _ := m.pan(lay)
	to := scene.PanShowing(cur, i/lay.Beds, lay.PerRow, lay.Columns)
	start := m.cfg.Now()
	if to == cur {
		start = start.Add(-scene.SlideFor) // already there: no slide
	}
	m.cam = camera{shift: m.cam.shift, held: true, from: cur, to: to, slideFrom: start}
}

// release hands the camera back to automatic panning, resuming from the
// column it rests on.
func (m *Model) release() {
	if !m.cam.held {
		return
	}
	col := 0
	if lay := m.layout(); lay.Overflow {
		p, _ := m.pan(lay)
		col = int(math.Round(p)) % lay.Columns
	}
	m.cam = camera{shift: m.elapsed() - scene.ResumeAt(col)}
}
```

- [ ] **Step 4: Wire it into the model**

In `internal/live/model.go`:

1. Change the `helpLine` constant to `helpLine = "←/→ select · enter details · r refresh · t ticker · ? help · q quit"`. In `tickerLine`, change `		return fit(" "+helpLine, status+" ", m.cols)` to `		return fit(" "+helpLine, "", m.cols) // the keys need the whole line`.
2. In `type Model struct`, add after the `rotFrom` field:

```go
	sel        selection        // the selected plant and its card
	cam        camera           // panning, held while a plant is selected
```

3. Replace

```go
	case tea.WindowSizeMsg:
		m.cols, m.rows = msg.Width, msg.Height
```

with

```go
	case tea.WindowSizeMsg:
		m.cols, m.rows = msg.Width, msg.Height
		if i := m.selected(); i >= 0 {
			m.holdOn(i) // the layout changed: keep the selection on screen
		}
```

4. In `case tickMsg:`, directly after its first statement `		m.prune()`, add:

```go
		if m.sel.name != "" && m.cfg.Now().Sub(m.sel.lastInput) >= idleClear {
			m.unselect()
		}
```

5. In `case loadedMsg:`, replace

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
			if m.sel.name != "" && m.selected() < 0 {
				m.unselect() // its repo left the garden
			}
```

6. Replace

```go
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
```

with

```go
	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
			m.sel.lastInput = m.cfg.Now()
			m.click(msg.X, msg.Y)
		}
	case tea.KeyMsg:
		m.sel.lastInput = m.cfg.Now()
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			switch {
			case m.sel.card:
				m.sel.card = false
			case m.selected() >= 0:
				m.unselect()
			default:
				return m, tea.Quit
			}
		case "left":
			m.stepSelection(-1)
		case "right":
			m.stepSelection(+1)
		case "enter":
			if m.selected() >= 0 {
				m.sel.card = !m.sel.card
			}
```

7. In `busy`, replace

```go
	lay := scene.LayoutFor(m.cols, m.gardenRows(), len(m.repos))
	if lay.Overflow && (scene.Sliding(m.elapsed()) || scene.Sliding(m.elapsed()+slowFrame)) {
		return true
	}
```

with

```go
	if _, sliding := m.pan(m.layout()); sliding {
		return true // the camera slides, or is about to
	}
```

8. In `View`, replace

```go
	at := m.now()
	rows := m.gardenRows()
	plots := m.plots(at)
	v := scene.View{Cols: m.cols, Rows: rows, Plots: plots, Now: at, Seed: 1, Motion: true,
		Sky: m.cfg.Sky, StarTotal: m.starTotal(), Shooting: m.shots}
	if lay := scene.LayoutFor(m.cols, rows, len(plots)); lay.Overflow {
		v.Pan = scene.PanAt(m.elapsed(), lay.Columns)
	}
	out := scene.Draw(v).Encode(m.cfg.Profile)
```

with

```go
	at := m.now()
	out := scene.Draw(m.sceneView(at)).Encode(m.cfg.Profile)
```

and add, directly above `func (m Model) View() string {`:

```go
// sceneView is the frame at time at: the plants, sky, camera, selection and
// card. View draws it; arrow keys and clicks read it, so they act on what's
// on screen.
func (m Model) sceneView(at time.Time) scene.View {
	rows := m.gardenRows()
	v := scene.View{Cols: m.cols, Rows: rows, Plots: m.plots(at), Now: at, Seed: 1, Motion: true,
		Sky: m.cfg.Sky, StarTotal: m.starTotal(), Shooting: m.shots}
	v.Pan, _ = m.pan(m.layout())
	if i := m.selected(); i >= 0 {
		v.Selected = i + 1
		if m.sel.card {
			card := CardFor(m.repos[i], at, m.cfg.DecayDays, rows)
			v.Card = &card
		}
	}
	return v
}
```

- [ ] **Step 5: Turn on the mouse, and document the keys**

In `cmd/gag/garden.go`, in `runGarden`, change `	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()` to `	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()`.

In `README.md`, replace the line

```
Live keys: `q` quit · `r` refresh now · `t` hide/show the ticker · `?` help.
```

with

```
Live keys: `←/→` select a plant · `enter` open its detail card · `esc` close ·
`r` refresh now · `t` hide/show the ticker · `?` help · `q` quit. A click
selects a plant too. While GAG runs it takes the mouse: hold ⌥ Option
(Alt on Linux) to select text.
```

- [ ] **Step 6: Run the tests and the frame budget**

Run: `gofmt -w internal/live && go test ./internal/live/ && go test -race ./internal/live/ && go test ./... && go test ./internal/live/ -bench LiveFrame -run '^$'`
Expected:
- PASS.
- `BenchmarkLiveFrameWithCard` reports under 10 ms per operation, i.e. below `10000000 ns/op`.

- [ ] **Step 7: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/live cmd/gag README.md
git commit -m "Select a plant with the arrow keys or a click; enter opens its card

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Polish: the v0.5 deferred fixes

**Files:**
- Modify:
  - `internal/config/config.go`: decay rejects `nan` and `inf`.
  - `internal/live/model.go`: shooting stars queue behind waiting ones and start one idle frame later; notes merge.
  - `cmd/gag/garden.go`: `onceView`.
  - `cmd/gag/configcmd.go` and `cmd/gag/main.go`: `runConfig(w)`.
  - `docs/superpowers/specs/2026-09-28-star-sky-and-config-design.md`: §1 wording.
- Test: `internal/config/config_test.go`, `internal/live/model_test.go`, `internal/replay/model_test.go` (append to each); `cmd/gag/configcmd_test.go` (create).

**Interfaces:**
- Consumes:
  - v0.5's `noticeNewStars`, `starNote`, `shotGap`, `slowFrame`, `noteFor` and the test helper `starred`.
  - Task 5's `sceneView`.
  - `replay` test helper `newTestModel`.
- Produces: `onceView(src source, now, at time.Time, sky scene.SkyMode, cols int) (scene.View, error)`, `runConfig(w io.Writer) error` and `(*Model).addNote(repo string, n int, wall time.Time)`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/config/config_test.go`:

```go
func TestDecayMustBeAFiniteNumber(t *testing.T) {
	for _, v := range []string{"nan", "NaN", "inf", "+Inf", "-inf"} {
		c, warns := Parse(strings.NewReader("decay = " + v))
		if len(warns) != 1 || c.Decay != 45 {
			t.Errorf("decay = %s: decay %v, warnings %v; want one warning and 45", v, c.Decay, warns)
		}
	}
}
```

Append to `internal/live/model_test.go`:

```go
func TestShootingStarsQueueUp(t *testing.T) {
	now := t0
	cur := starred(garden3(), 1, 1, 1)
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return cur, nil },
		DecayDays: 45,
		Now:       func() time.Time { return now },
	})
	m = ready(m, 100, 30)
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 3, 1, 1)})
	if first := m.shots[0].Start; first.Before(t0.Add(slowFrame)) {
		t.Errorf("the first shooting star starts %v after its refresh; want it to wait one idle frame", first.Sub(t0))
	}
	now = now.Add(time.Second)
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 5, 1, 1)})
	if len(m.shots) != 4 {
		t.Fatalf("shots = %d, want 4", len(m.shots))
	}
	for i := 1; i < len(m.shots); i++ {
		if gap := m.shots[i].Start.Sub(m.shots[i-1].Start); gap < shotGap {
			t.Errorf("shots %d and %d start %v apart; want at least %v", i-1, i, gap, shotGap)
		}
	}
}

func TestStarsForTheSameRepoShareANote(t *testing.T) {
	now := t0
	cur := starred(garden3(), 1, 1, 1)
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return cur, nil },
		DecayDays: 45,
		Now:       func() time.Time { return now },
	})
	m = ready(m, 100, 30)
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 2, 1, 1)})
	now = now.Add(5 * time.Second)
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 4, 1, 1)})
	if len(m.notes) != 1 || m.notes[0].n != 3 {
		t.Fatalf("notes = %+v; want one note of 3 new stars on bloom", m.notes)
	}
	if m.notes[0].until != now.Add(noteFor) {
		t.Error("new stars should restart the note's minute")
	}
	lines := strings.Split(m.View(), "\n")
	if last := visible(lines[len(lines)-1]); !strings.Contains(last, "⭐ 3 new stars on bloom") {
		t.Errorf("ticker = %q", last)
	}
}

func TestSkySettingReachesTheLiveView(t *testing.T) {
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return starred(garden3(), 5, 0, 1), nil },
		DecayDays: 45,
		Now:       func() time.Time { return t0 },
		Sky:       scene.SkyStars,
	})
	m = ready(m, 100, 30)
	if v := m.sceneView(m.now()); v.Sky != scene.SkyStars || v.StarTotal != 6 {
		t.Errorf("the live frame has sky %v with %d stars; want the star sky with 6", v.Sky, v.StarTotal)
	}
}
```

Append to `internal/replay/model_test.go`, and add `"github.com/RursusAeternum/GitAGarden/internal/scene"` to its imports:

```go
func TestReplayDrawsTheConfiguredSky(t *testing.T) {
	night := time.Date(2026, 6, 1, 23, 0, 0, 0, time.Local)
	frame := func(stars int) string {
		m := newTestModel()
		m.cfg.Now = func() time.Time { return night }
		m.cfg.Sky, m.cfg.Stars = scene.SkyStars, stars
		return m.View()
	}
	if frame(0) == frame(40) {
		t.Error("replay ignores the star sky setting")
	}
}
```

`cmd/gag/configcmd_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func TestGagConfigShowsTheFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "gag", "config")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("sky = stars\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := runConfig(&b); err != nil {
		t.Fatal(err)
	}
	if want := "config: " + path + "\nsky      stars   (file)\n"; !strings.HasPrefix(b.String(), want) {
		t.Errorf("gag config printed\n%s\nwant it to start\n%s", b.String(), want)
	}
}

func TestOnceDrawsTheConfiguredSky(t *testing.T) {
	now := time.Now()
	v, err := onceView(source{demo: true, decay: 45}, now, now, scene.SkyStars, 100)
	if err != nil {
		t.Fatal(err)
	}
	if v.Sky != scene.SkyStars || v.StarTotal != 48 || v.Cols != 100 || len(v.Plots) != len(demo) {
		t.Errorf("--once view: sky %v, %d stars, %d cols, %d plots", v.Sky, v.StarTotal, v.Cols, len(v.Plots))
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config/ ./internal/live/ ./internal/replay/ ./cmd/gag/`
Expected:
- **config:** FAIL: `TestDecayMustBeAFiniteNumber` shows `nan` and `inf` accepted.
- **live:** FAIL: `TestShootingStarsQueueUp` (the first shot starts at +0s, then the gaps are out of order) and `TestStarsForTheSameRepoShareANote` (two notes).
- **cmd/gag:** FAIL, with build errors: `too many arguments in call to runConfig` and `undefined: onceView`.
- **Wiring pins already passing:** `TestSkySettingReachesTheLiveView` and `TestReplayDrawsTheConfiguredSky` PASS, because they pin wiring that already exists. Prove each can fail:
  1. Temporarily delete `Sky: m.cfg.Sky, ` from `sceneView` in `internal/live/model.go`, and `Sky: m.cfg.Sky, StarTotal: m.cfg.Stars` from the `scene.Draw` call in `internal/replay/model.go`.
  2. Run `go test ./internal/live/ -run TestSkySettingReachesTheLiveView` and `go test ./internal/replay/ -run TestReplayDrawsTheConfiguredSky`. Both FAIL.
  3. Restore both lines exactly.

- [ ] **Step 3: Decay**

In `internal/config/config.go`, add `"math"` to the imports and change `		if err != nil || f <= 0 {` (in `case "decay":`) to `		if err != nil || !(f > 0) || math.IsInf(f, 0) {`.

- [ ] **Step 4: Shooting stars queue up, and notes merge**

In `internal/live/model.go`, in `noticeNewStars`:

1. Replace

```go
		wall, at, shots := m.cfg.Now(), m.now(), 0
```

with

```go
		wall, at, shots := m.cfg.Now(), m.now(), 0
		next := at.Add(slowFrame) // by then the fast frames have started
		for _, s := range m.shots {
			if t := s.Start.Add(shotGap); t.After(next) {
				next = t // queue behind the shooting stars still waiting
			}
		}
```

2. Replace

```go
			m.notes = append(m.notes, starNote{repo: r.Name, n: gained, until: wall.Add(noteFor)})
```

with

```go
			m.addNote(r.Name, gained, wall)
```

3. Replace

```go
				m.shots = append(m.shots, scene.Shooting{Start: at.Add(time.Duration(shots) * shotGap), Seed: at.UnixMilli() + int64(len(m.shots))})
```

with

```go
				m.shots = append(m.shots, scene.Shooting{Start: next.Add(time.Duration(shots) * shotGap), Seed: at.UnixMilli() + int64(len(m.shots))})
```

4. Add after `noticeNewStars`, and add `"slices"` to the file's imports:

```go
// addNote tells the ticker repo got n new stars. While that repo's note is
// still showing, the stars are added to it and its minute starts over.
func (m *Model) addNote(repo string, n int, wall time.Time) {
	notes := slices.Clone(m.notes)
	for i := range notes {
		if notes[i].repo == repo && wall.Before(notes[i].until) {
			notes[i].n += n
			notes[i].until = wall.Add(noteFor)
			m.notes = notes
			return
		}
	}
	m.notes = append(notes, starNote{repo: repo, n: n, until: wall.Add(noteFor)})
}
```

- [ ] **Step 5: Testable `--once` and `gag config`**

In `cmd/gag/garden.go`, replace the whole `printOnce` function with:

```go
// printOnce prints one static frame: for pipes, scripts and -once.
func printOnce(src source, ahead time.Duration, simulate string, sky scene.SkyMode) error {
	now := time.Now()
	at := now.Add(ahead) // the moment the garden is drawn at
	v, err := onceView(src, now, at, sky, termWidth())
	if err != nil {
		return err
	}
	header := ""
	if ahead > 0 {
		header = fmt.Sprintf("simulating %s ahead: %s, nothing tended\n\n", simulate, at.Format("2006-01-02"))
	}
	fmt.Print(header + scene.Draw(v).Encode(colorProfile()) + "\n")
	return nil
}

// onceView is the static frame --once prints: the garden loaded at now,
// drawn at time at, cols wide.
func onceView(src source, now, at time.Time, sky scene.SkyMode, cols int) (scene.View, error) {
	var plots []scene.Plot
	stars := 0
	if src.demo {
		for _, r := range demoGarden(now) {
			plots = append(plots, live.Plot(r, at, src.decay))
			stars += r.Stars
		}
	} else {
		log, progress := logf, (func(done, total int, current string))(nil)
		if term.IsTerminal(int(os.Stderr.Fd())) {
			log, progress = nil, stderrProgress(os.Stderr) // a bar instead of "fetching …" lines
		}
		repos, offline, err := loadRepos(context.Background(), src.names, src.owner, src.limit, src.ttl, log, progress)
		if err != nil {
			return scene.View{}, err
		}
		if offline {
			logf("GitHub unreachable or partly stale; showing cached data")
		}
		for _, r := range repos {
			plots = append(plots, live.Plot(repoFor(r, now), at, src.decay))
			stars += r.Stars
		}
	}
	return scene.View{Cols: cols, Plots: plots, Now: at, Seed: 1, Sky: sky, StarTotal: stars}, nil
}
```

Replace `cmd/gag/configcmd.go` with:

```go
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/RursusAeternum/GitAGarden/internal/config"
)

// runConfig writes where the config file lives and the settings in effect.
func runConfig(w io.Writer) error {
	path := config.Path()
	_, err := os.Stat(path)
	_, werr := fmt.Fprint(w, loadConfig().Describe(path, err == nil))
	return werr
}
```

In `cmd/gag/main.go`, change `		err = runConfig()` to `		err = runConfig(os.Stdout)`.

- [ ] **Step 6: The v0.5 spec's wording**

In `docs/superpowers/specs/2026-09-28-star-sky-and-config-design.md`, replace

```
- Store it as `github.Repo.Stars` (JSON `stars`) in the cache. A repo served
  from cache within the TTL keeps its cached count. Caches from older
  versions load with 0 until the next listing.
```

with

```
- Store it as `github.Repo.Stars` (JSON `stars`) in the cache. A repo served
  from cache within the TTL still takes the fresh count from the listing;
  only offline loads use cached counts. Caches from older versions load
  with 0 until the next listing.
```

- [ ] **Step 7: Run everything**

Run: `gofmt -l . ; go vet ./... && go test ./... && go test -race ./internal/live/`
Expected: PASS, with no gofmt output.

- [ ] **Step 8: Commit**

```bash
git add internal cmd docs/superpowers/specs/2026-09-28-star-sky-and-config-design.md
git commit -m "Polish: finite decay, queued shooting stars, merged star notes, pinned sky wiring

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Real-world check and the v0.6.0 release gate

**Files:**
- None, unless the check turns up bugs.

- [ ] **Step 1: The demo, by hand**

Run: `go build -o gag ./cmd/gag && GAG_COLOR=truecolor ./gag garden --once -demo | head -5`
Expected: the demo garden prints, with nothing selected (`--once` never selects).

- [ ] **Step 2: Hand it to the user**

Ask the user to run `./gag garden -demo`, then `./gag` on their real garden. They should try:
- `→`, `←`, `enter` and `esc` (twice, then a third time to quit)
- a click on a plant, then on the sky beside the plants
- resizing the window with a card open
- leaving the card open for a minute, after which it closes on its own

Fix anything they report, each with a failing test first.

- [ ] **Step 3: Release gate**

Merging to `main` and tagging publish a public release. Do this only on the user's explicit yes:

```bash
git tag -a v0.6.0 -m "v0.6.0: pick a plant, read its card"
git push origin main v0.6.0
gh run watch "$(gh run list -R RursusAeternum/GitAGarden -w release -L 1 --json databaseId --jq '.[0].databaseId')" -R RursusAeternum/GitAGarden --exit-status
gh workflow run update-gag.yml -R RursusAeternum/homebrew-tap
```

If `brew upgrade gag` is slow here, formulae.brew.sh may be slow again. The workaround is to run `git -C "$(brew --repo rursusaeternum/tap)" pull`, then `HOMEBREW_NO_AUTO_UPDATE=1 brew upgrade --cask rursusaeternum/tap/gag`.
