# Compact Plants and Recent History Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every plant grows as a skeleton from its repo's whole history plus a canopy from recent activity, so no plant fills its plot. GAG can fetch only the totals and the last 90 days: `history = recent`, the new default. `history = full` brings back full fetching.

**Architecture:**
- **`internal/garden`** grows each plant in two layers (`GrowAt`). `Grow(name, species, events)` stays as the helper for a whole history.
  - The skeleton grows from `Totals`, in structural steps drawn from the plant's own random stream.
  - The canopy grows from the events inside its windows. Each leaf, flower and fruit is placed from its own event, inside a species silhouette.
- **`internal/github`** fetches GitHub's totals for every repo. In `RecentHistory` mode it fetches only recent commits, merges, issues and releases. The cache marks histories that are incomplete.
- **`internal/config`** gains the `history` setting.
- **`cmd/gag`** passes the setting to the fetch, grows plants from totals, and makes `gag replay` fetch whole histories.
- **`internal/live`** stops a jump in a plant's counters from looking like a merge.

**Tech Stack:** Go 1.26, the GitHub GraphQL API (`totalCount`, `orderBy`, `filterBy`), no new modules.

**Spec:** `docs/superpowers/specs/2026-09-30-compact-plants-design.md`, on top of main (v0.7.1). It ships as v0.8.0.

**Prototype.** All code in this plan was prototyped and its tests run before the plan was written. The first gardens were rendered to check the look:
- the growth rules held
- the whole suite passed apart from the golden frame, which Task 1 regenerates
- growing 50,000 events took 1.4 ms

**Plan refinements of the spec.** Each one keeps the spec's intent:
1. **Names.** The spec's `Grow(name, species, totals, events, at)` is `GrowAt`. `Grow(name, species, events)` keeps its name as the helper that counts totals from a whole history and grows the plant as of its newest event. About 40 tests call it.
2. **The rosette's base is skeleton.** The spec lists "the width of its base" as the rosette's structure. So the dense base grows with the skeleton. The rosette's canopy leaves go in the ring of slots around the base, so a busy rosette spreads wider.
3. **One leaf pool.** The leaf budget is a fixed pool: the first half of the species' leaf spots, in the plant's own order.
   - The dormant third is the start of that pool, and is always in leaf.
   - Recent pushes land anywhere in the pool. A push that lands on a dormant leaf makes it recent; a second push on the same cell makes it lusher.
   - So a busy plant is the same plant, lusher, and every leaf stays in the pool.
4. **Fruit hangs among the pool's cells,** not only on those in leaf, so it never moves when a leaf ages out.
5. **A merge reaction needs a newer merge.** When a repo's totals first arrive, its merge counter jumps. That happens on the first fetch after upgrading from v0.7. v0.7's rule would call the jump "🌸 merged a PR · +2498 more", so a merge reaction now also needs `LastMerge` to be newer.
6. **The shrub's shape was tuned by eye** against rendered plants. Task 6's hands-on check is where these numbers get their final say.
   - Its stems grow inside a widening cone: `|x − center| ≤ 2(ground − y)/5 + 2`.
   - Its crown is an ellipse around the stems, never more than a row above the highest stem and clear of the lowest three rows.
   - The step counts at full size are shrub 34, cactus 44 and rosette 60, rounded down along the log curve.

## Global Constraints

- Module `github.com/RursusAeternum/GitAGarden`, `go 1.26.0`. Go is at `~/sdk/go/bin`: run `export PATH=$HOME/sdk/go/bin:$PATH` before any `go` command.
- No new third-party modules.
- The windows are fixed:
  - leaves come from pushes of the last **90 days**
  - flowers come from merges of the last **30 days**, and stay as seed heads until **90 days**
  - fruit comes from releases of the past year, up to **four**
- The leaf budget is half the silhouette's cells, rounded down. A **third** of it is always in leaf. Flowers and seed heads fill at most a **quarter** of it.
- The setting is `history = recent` (the default) or `history = full`, in the config file only. Any other value warns and falls back to `recent`.
- `full` and `recent` grow identical plants from the same repo.
- Growing a 50,000-event plant takes under 2 ms.
- `internal/scene/testdata/garden.golden` is regenerated once, in Task 1, and passes unchanged in every later task.
- `cmd/gag/demo.go` must not import `internal/github`: the browser build reaches it.
- After every task, `go vet ./... && go test ./...` passes and `gofmt -l .` prints nothing.
- Every commit message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Upgrading from a v0.7 cache.** The first fetch with totals makes a plant's counters jump by thousands of commits and merges. That must play no reactions or notes. Pinned by Task 4 `TestTotalsArrivingAreNoNews`.
2. **Totals lower than the events,** after a rewritten history or from a stale count. A plant never shrinks below what its events count. Pinned by Task 1 `TestTotalsNeverShrinkBelowTheEvents`.
3. **Events in any order.** The demo world and some tests list merges after pushes, so the plant must not depend on event order. Pinned by Task 1 `TestEventOrderDoesNotMatter`.
4. **An empty repo in `recent` mode,** with no default branch and no commits. It must fetch cleanly and grow as a seedling. Pinned by Task 2 `TestRecentEmptyRepo`.
5. **Real GitHub accepts the new queries:** the totals aliases, `filterBy:{since}` on closed issues, and `UPDATED_AT` ordering of merged PRs. Only a run against GitHub can tell. Pinned by Task 6, step 1.

---

### Task 1: Garden: skeleton and canopy

**Files:**
- Create: `internal/garden/canopy.go`, `internal/garden/compact_test.go`
- Modify:
  - `internal/garden/growers.go`: full replacement
  - `internal/garden/plant.go`: `Grow` becomes a wrapper
  - `internal/scene/testdata/garden.golden`: regenerated

**Interfaces:**
- Consumes:
  - v0.2's `Plant`, `Event`, `FakeHistory`, `growthFrac`, `seedlingHeight`, `hash01` and `seedOf`
  - the test variable `t0` from `plant_test.go`
- Produces:
  - `type Totals struct{ Pushes, Merges, Releases, OpenIssues int }`
  - `func TotalsOf(events []Event, at time.Time) Totals`
  - `func GrowAt(name string, sp Species, t Totals, events []Event, at time.Time) *Plant`
  - `func Grow(name string, sp Species, events []Event) *Plant`, its signature unchanged
  - unexported, for tests:
    - `skeleton(name, sp, t) (*Plant, habit)`
    - the `habit` interface: `step`, `leafSpots` and `flowerSpots`
    - `leafWindow`, `flowerWindow`, `fruitWindow`, `maxFruit`, `fullSteps`, `stepsFor` and `stepTop`

- [ ] **Step 1: Write the failing tests**

`internal/garden/compact_test.go`:

```go
package garden

import (
	"math/rand"
	"testing"
	"time"
)

const day = 24 * time.Hour

func kinds(p *Plant, ks ...CellKind) int { return len(p.cellsOf(ks...)) }

// spread is n events of kind k spread evenly over span, ending at end.
func spread(k EventKind, n int, span time.Duration, end time.Time) []Event {
	ev := make([]Event, n)
	for i := range ev {
		ev[i] = Event{Kind: k, At: end.Add(-span + time.Duration(i+1)*span/time.Duration(n))}
	}
	return ev
}

func TestCanopyKeepsToItsBudget(t *testing.T) {
	for _, sp := range AllSpecies {
		for _, n := range []int{10, 500, 3000, 50000} {
			ev := FakeHistory("budget", n, t0)
			at := ev[len(ev)-1].At
			sk, h := skeleton("budget", sp, TotalsOf(ev, at))
			budget := len(h.leafSpots()) / 2
			p := GrowAt("budget", sp, Totals{}, ev, at)
			if leaves := kinds(p, Leaf) - kinds(sk, Leaf); leaves > budget {
				t.Errorf("%s, %d events: %d leaves, over the budget of %d", sp, n, leaves, budget)
			}
			if f := kinds(p, Flower); f > budget/4 {
				t.Errorf("%s, %d events: %d flowers, over a quarter of the budget (%d)", sp, n, f, budget/4)
			}
			if f := kinds(p, Fruit); f > maxFruit {
				t.Errorf("%s, %d events: %d fruit, want at most %d", sp, n, f, maxFruit)
			}
			if filled := kinds(p, Stem, Body, Leaf, Flower, Fruit); filled > Width*Height/4 {
				t.Errorf("%s, %d events: %d of %d cells filled; a plant never fills its plot", sp, n, filled, Width*Height)
			}
		}
	}
}

func TestBigShrubIsACrownOnATrunk(t *testing.T) {
	p := Grow("gag-core", Shrub, FakeHistory("gag-core", 3000, t0))
	for y := ground - 2; y <= ground; y++ {
		for x := 0; x < Width; x++ {
			if k := p.Grid[y][x].Kind; k != Empty && k != Stem {
				t.Fatalf("cell %d,%d of the lower trunk is kind %d; want bare stem", x, y, k)
			}
		}
	}
	left, right := Width, 0
	for _, q := range p.cellsOf(Leaf, Flower, Fruit) {
		left, right = min(left, q.x), max(right, q.x)
	}
	if right-left >= Width-2 {
		t.Errorf("the crown spans columns %d to %d: the whole plot, a rectangle again", left, right)
	}
}

func TestSkeletonOnlyEverGrows(t *testing.T) {
	for _, sp := range AllSpecies {
		for n := 0; n < 2000; n += 37 {
			a, _ := skeleton("grows", sp, Totals{Pushes: n})
			b, _ := skeleton("grows", sp, Totals{Pushes: n + 100})
			for y := 0; y < Height; y++ {
				for x := 0; x < Width; x++ {
					// Nothing grown goes; stems and flesh stay what they are. A
					// rosette's stalk may rise through a leaf of its base.
					was, is := a.Grid[y][x].Kind, b.Grid[y][x].Kind
					if was != Empty && (is == Empty || (was != Leaf && is != was)) {
						t.Fatalf("%s: cell %d,%d of the %d-push skeleton is gone at %d pushes", sp, x, y, n, n+100)
					}
				}
			}
		}
	}
}

func TestCanopyStaysPut(t *testing.T) {
	for _, sp := range AllSpecies {
		ev := FakeHistory("put", 3000, t0)
		at := ev[len(ev)-1].At
		later := at.Add(7 * day) // no new events
		a, b := GrowAt("put", sp, Totals{}, ev, at), GrowAt("put", sp, Totals{}, ev, later)
		for y := 0; y < Height; y++ {
			for x := 0; x < Width; x++ {
				was, is := a.Grid[y][x], b.Grid[y][x]
				if was.Kind == is.Kind {
					continue
				}
				window := leafWindow
				if was.Kind == Fruit {
					window = fruitWindow
				}
				if was.At == 0 || later.Sub(time.Unix(was.At, 0)) <= window {
					t.Errorf("%s: cell %d,%d went from kind %d to %d, though its event is still in its window", sp, x, y, was.Kind, is.Kind)
				}
			}
		}
	}
}

func TestQuietPlantKeepsItsDormantLeaves(t *testing.T) {
	for _, sp := range AllSpecies {
		ev := spread(Push, 400, 200*day, t0)
		last := ev[len(ev)-1].At
		sk, h := skeleton("quiet", sp, TotalsOf(ev, last))
		pool := len(h.leafSpots()) / 2
		quiet := GrowAt("quiet", sp, Totals{}, ev, last.Add(365*day))
		quieter := GrowAt("quiet", sp, Totals{}, ev, last.Add(3*365*day))
		if leaves := kinds(quiet, Leaf) - kinds(sk, Leaf); leaves != pool/3 {
			t.Errorf("%s: a year quiet, %d canopy leaves; want the dormant third, %d", sp, leaves, pool/3)
		}
		if quiet.Grid != quieter.Grid {
			t.Errorf("%s: two years later the dormant leaves moved", sp)
		}
	}
}

func TestRecentHistoryGrowsTheSamePlant(t *testing.T) {
	for _, sp := range AllSpecies {
		all := FakeHistory("same", 3000, t0)
		at := all[len(all)-1].At.Add(day)
		var recent []Event
		for _, e := range all {
			if at.Sub(e.At) <= 365*day {
				recent = append(recent, e)
			}
		}
		a := GrowAt("same", sp, Totals{}, all, at)
		b := GrowAt("same", sp, TotalsOf(all, at), recent, at)
		if a.Grid != b.Grid || !a.LastTended.Equal(b.LastTended) || a.Pushes != b.Pushes || a.OpenIssues != b.OpenIssues {
			t.Errorf("%s: grown from the whole history and from totals plus the recent part, the plants differ", sp)
		}
	}
}

func TestFlowersAndFruitAreRecent(t *testing.T) {
	now := t0.Add(1000 * day)
	ev := spread(Push, 200, 900*day, now)
	ev = append(ev, spread(Merge, 20, 60*day, now.Add(-100*day))...) // all older than 90 days
	ev = append(ev, spread(Release, 10, 10*day, now.Add(-400*day))...)
	for _, sp := range AllSpecies {
		if p := GrowAt("old", sp, Totals{}, ev, now); kinds(p, Flower) != 0 || kinds(p, Fruit) != 0 {
			t.Errorf("%s: %d flowers and %d fruit from merges and releases long gone", sp, kinds(p, Flower), kinds(p, Fruit))
		}
	}
	fresh := append(append([]Event(nil), ev...), spread(Release, 6, 60*day, now)...)
	fresh = append(fresh, spread(Merge, 2, 10*day, now)...)
	for _, sp := range AllSpecies {
		p := GrowAt("fresh", sp, Totals{}, fresh, now)
		if kinds(p, Fruit) != maxFruit || kinds(p, Flower) != 2 {
			t.Errorf("%s: %d fruit and %d flowers; want %d fruit from this year's six releases and 2 flowers", sp, kinds(p, Fruit), kinds(p, Flower), maxFruit)
		}
	}
}

func TestTotalsNeverShrinkBelowTheEvents(t *testing.T) {
	ev := spread(Push, 300, 30*day, t0)
	p := GrowAt("rewritten", Shrub, Totals{Pushes: 5}, ev, t0) // totals older than the cache, or a rewritten history
	if p.Pushes != 300 {
		t.Errorf("pushes = %d; want the 300 the events count", p.Pushes)
	}
	if q := GrowAt("rewritten", Shrub, Totals{Pushes: 900}, ev, t0); q.Pushes != 900 {
		t.Errorf("pushes = %d; want GitHub's 900", q.Pushes)
	}
}

func TestEventOrderDoesNotMatter(t *testing.T) {
	ev := FakeHistory("order", 800, t0)
	shuffled := append([]Event(nil), ev...)
	rand.New(rand.NewSource(1)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	for _, sp := range AllSpecies {
		if a, b := Grow("order", sp, ev), Grow("order", sp, shuffled); a.Grid != b.Grid || !a.LastTended.Equal(b.LastTended) {
			t.Errorf("%s: the same events in another order grew another plant", sp)
		}
	}
}

func BenchmarkGrow50kEvents(b *testing.B) {
	ev := FakeHistory("bench", 50000, t0)
	at := ev[len(ev)-1].At
	for i := 0; i < b.N; i++ {
		GrowAt("bench", Shrub, Totals{}, ev, at)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/garden/`
Expected: FAIL, with build errors such as `undefined: skeleton`, `undefined: GrowAt` and `undefined: TotalsOf`.

- [ ] **Step 3: The species' habits**

Replace `internal/garden/growers.go` with:

```go
package garden

import (
	"math/rand"
	"sort"
	"strings"
)

type Species string

const (
	Shrub   Species = "shrub"
	Cactus  Species = "cactus"
	Rosette Species = "rosette"
)

var AllSpecies = []Species{Shrub, Cactus, Rosette}

// SpeciesFor picks a species from a repo's primary language, so the garden
// reads at a glance.
func SpeciesFor(language string) Species {
	switch strings.ToLower(language) {
	case "rust", "c", "c++", "c#", "java", "zig":
		return Cactus
	case "python", "javascript", "typescript", "ruby":
		return Rosette
	}
	return Shrub
}

func (s Species) Next() Species {
	for i, sp := range AllSpecies {
		if sp == s {
			return AllSpecies[(i+1)%len(AllSpecies)]
		}
	}
	return AllSpecies[0]
}

const maxLevel = 2

// A habit is how a species grows. Its skeleton grows one structural step at
// a time, from the plant's own random stream; the canopy then goes into the
// spots it offers, all inside the species' silhouette.
type habit interface {
	// step grows the skeleton by one structural step, no higher than row top.
	step(top int)
	// leafSpots are the cells leaves may take, in the plant's own order.
	leafSpots() []pt
	// flowerSpots are where flowers open.
	flowerSpots() []pt
}

// fullSteps is how many structural steps a species takes to reach full
// size, at about 500 pushes.
var fullSteps = map[Species]int{Shrub: 34, Cactus: 44, Rosette: 60}

func newHabit(sp Species, p *Plant, r *rand.Rand) habit {
	b := base{p: p, r: r}
	switch sp {
	case Cactus:
		return newCactus(b)
	case Rosette:
		return newRosette(b)
	}
	return newShrub(b)
}

type base struct {
	p *Plant
	r *rand.Rand
}

var dirs8 = []pt{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}

// near lists the empty cells above the ground row within Chebyshev distance
// d of a cell of one of kinds, top to bottom, left to right.
func (b base) near(d int, kinds ...CellKind) []pt {
	var out []pt
	for y := 0; y < ground; y++ {
		for x := 0; x < Width; x++ {
			if !b.p.free(x, y) {
				continue
			}
		search:
			for dy := -d; dy <= d; dy++ {
				for dx := -d; dx <= d; dx++ {
					if b.p.inBounds(x+dx, y+dy) {
						for _, k := range kinds {
							if b.p.Grid[y+dy][x+dx].Kind == k {
								out = append(out, pt{x, y})
								break search
							}
						}
					}
				}
			}
		}
	}
	return out
}

// shuffled is ps in an order of the plant's own: the same plant always gives
// the same order, whatever else happens.
func (b base) shuffled(ps []pt, salt int) []pt {
	out := append([]pt(nil), ps...)
	sort.SliceStable(out, func(i, j int) bool {
		return hash01(b.p.Name, out[i].x+salt*100, out[i].y) < hash01(b.p.Name, out[j].x+salt*100, out[j].y)
	})
	return out
}

// ---- shrub: branching woody stems; leaves cluster around them in a
// rounded crown ----

type tip struct{ x, y, lean int }

type shrub struct {
	base
	tips []tip
}

func newShrub(b base) *shrub {
	for y := ground; y > ground-3; y-- { // seedling: a short stem
		b.p.set(center, y, Stem)
	}
	return &shrub{base: b, tips: []tip{{center, ground - 2, 0}}}
}

// step extends one branch tip upwards, sometimes forking. A tip held back
// only by the height limit stays alive for later; one boxed in is dropped.
func (s *shrub) step(top int) {
	var dead []int
	defer func() {
		sort.Sort(sort.Reverse(sort.IntSlice(dead)))
		for _, i := range dead {
			s.tips = append(s.tips[:i], s.tips[i+1:]...)
		}
	}()
	for _, i := range s.r.Perm(len(s.tips)) {
		t := &s.tips[i]
		leans := []int{t.lean, -1, 0, 1}
		s.r.Shuffle(3, func(a, b int) { leans[a+1], leans[b+1] = leans[b+1], leans[a+1] })
		for _, dx := range leans {
			x, y := t.x+dx, t.y-1
			if y < top || !s.p.free(x, y) || abs(x-center) > 2*(ground-y)/5+2 { // a widening cone: bushy, not sprawling
				continue
			}
			s.p.set(x, y, Stem)
			t.x, t.y = x, y
			if s.r.Float64() < 0.25 {
				t.lean = dx
			}
			if len(s.tips) < 7 && y < ground-4 && s.r.Float64() < 0.25 {
				s.tips = append(s.tips, tip{x, y, []int{-1, 1}[s.r.Intn(2)]})
			}
			return
		}
		if t.y-1 >= top {
			dead = append(dead, i)
		}
	}
}

// crown is the shrub's rounded crown: an ellipse around its stems, clear
// of the bare lower trunk. in reports whether a cell lies inside it.
func (s *shrub) crown(stems []pt) (in func(x, y int) bool) {
	top, left, right := ground, center, center
	for _, q := range stems {
		top, left, right = min(top, q.y), min(left, q.x), max(right, q.x)
	}
	bottom := ground - 3
	if top >= bottom {
		return func(x, y int) bool { return y >= top-1 && y < ground }
	}
	cy, ry := float64(top+bottom)/2, float64(bottom-top)/2+2
	cx, rx := float64(left+right)/2, float64(right-left)/2+4
	return func(x, y int) bool { // never more than a row above the highest stem
		dx, dy := (float64(x)-cx)/rx, (float64(y)-cy)/ry
		return y >= top-1 && dx*dx+dy*dy <= 1
	}
}

// Shrub leaves take the crown nearest the branches first, so foliage
// gathers around them and the crown's edge stays airy.
func (s *shrub) leafSpots() []pt {
	stems := s.p.cellsOf(Stem)
	inCrown := s.crown(stems)
	type spot struct {
		p   pt
		key float64
	}
	var in []spot
	for y := 0; y < ground-2; y++ {
		for x := 0; x < Width; x++ {
			if !s.p.free(x, y) || !inCrown(x, y) {
				continue
			}
			d := Width
			for _, q := range stems {
				d = min(d, max(abs(q.x-x), abs(q.y-y)))
			}
			in = append(in, spot{pt{x, y}, float64(d) + 0.9*hash01(s.p.Name, x+100, y)})
		}
	}
	sort.SliceStable(in, func(i, j int) bool { return in[i].key < in[j].key })
	out := make([]pt, len(in))
	for i, sp := range in {
		out[i] = sp.p
	}
	return out
}

func abs(v int) int { return max(v, -v) }

func (s *shrub) flowerSpots() []pt {
	var out []pt
	for _, t := range s.tips {
		for _, d := range dirs8 {
			if x, y := t.x+d.x, t.y+d.y; s.p.free(x, y) && y < ground {
				out = append(out, pt{x, y})
			}
		}
	}
	if len(out) == 0 {
		out = s.leafSpots()
	}
	return s.shuffled(dedup(out), 2)
}

// ---- cactus: a three-pixel trunk that grows tall, then sprouts two-pixel
// arms; spines stand around its flesh ----

type cactus struct {
	base
	top  int
	arms []tip // lean is the side (-1 left, 1 right)
}

func newCactus(b base) *cactus {
	c := &cactus{base: b, top: ground - 1}
	c.row(ground) // seedling: a stubby two-row trunk
	c.row(ground - 1)
	return c
}

func (c *cactus) row(y int) {
	for dx := -1; dx <= 1; dx++ {
		c.p.set(center+dx, y, Body)
	}
}

func (c *cactus) step(top int) {
	f := c.r.Float64()
	switch {
	case f < 0.55 && c.growBody(top):
	case f < 0.70 && c.sproutArm():
	case c.growArm():
	case c.growBody(top):
	}
}

func (c *cactus) growBody(top int) bool {
	y := c.top - 1
	if y < max(top, 1) {
		return false
	}
	for dx := -1; dx <= 1; dx++ {
		if !c.p.free(center+dx, y) {
			return false
		}
	}
	c.top = y
	c.row(y)
	return true
}

func (c *cactus) sproutArm() bool {
	if len(c.arms) >= 2 || ground-c.top < 9 {
		return false
	}
	side := []int{-1, 1}[c.r.Intn(2)]
	if len(c.arms) == 1 {
		side = -c.arms[0].lean
	}
	y := c.top + 3 + c.r.Intn(ground-c.top-7) // between top+3 and ground-5
	for dx := 2; dx <= 4; dx++ {
		if !c.p.free(center+dx*side, y) {
			return false
		}
	}
	for dx := 2; dx <= 4; dx++ {
		c.p.set(center+dx*side, y, Body)
	}
	c.arms = append(c.arms, tip{x: center + 3*side, y: y, lean: side})
	return true
}

// growArm raises an arm by a row. Arms are two pixels wide and stay below
// the trunk's top.
func (c *cactus) growArm() bool {
	for _, i := range c.r.Perm(len(c.arms)) {
		a := &c.arms[i]
		y := a.y - 1
		if y <= c.top+1 || !c.p.free(a.x, y) || !c.p.free(a.x+a.lean, y) {
			continue
		}
		a.y = y
		c.p.set(a.x, y, Body)
		c.p.set(a.x+a.lean, y, Body)
		return true
	}
	return false
}

// Spines stand beside the flesh, never above the crown of the trunk or an
// arm, where flowers go.
func (c *cactus) leafSpots() []pt {
	var out []pt
	for _, q := range c.near(1, Body) {
		if q.y > c.top {
			out = append(out, q)
		}
	}
	return c.shuffled(out, 1)
}

func (c *cactus) flowerSpots() []pt {
	var out []pt
	for dx := -1; dx <= 1; dx++ {
		if c.p.free(center+dx, c.top-1) {
			out = append(out, pt{center + dx, c.top - 1})
		}
	}
	for _, a := range c.arms {
		for _, x := range []int{a.x, a.x + a.lean} {
			if c.p.free(x, a.y-1) {
				out = append(out, pt{x, a.y - 1})
			}
		}
	}
	return c.shuffled(out, 2)
}

// ---- rosette: a low succulent that fills out from the middle, and sends
// up a flowering stalk ----

type rosette struct {
	base
	slots []pt // the rosette's cells, innermost first
	width int  // how many of them its base fills
	stalk int
}

// baseShare is how much of the full rosette its base fills at full size;
// the canopy's leaves go in the ring around it.
const baseShare = 0.6

func newRosette(b base) *rosette {
	type slot struct {
		p   pt
		key float64
	}
	var ss []slot
	for y := ground - 8; y <= ground; y++ {
		for x := 0; x < Width; x++ {
			dx, dy := float64(x-center)/10.5, float64(ground-y)/8.5
			if e := dx*dx + dy*dy; e <= 1 {
				ss = append(ss, slot{pt{x, y}, e + b.r.Float64()*0.2})
			}
		}
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].key < ss[j].key })
	g := &rosette{base: b}
	for _, s := range ss {
		g.slots = append(g.slots, s.p)
	}
	g.widen(3) // seedling
	return g
}

// widen fills the base out to its first n slots.
func (g *rosette) widen(n int) {
	for ; g.width < min(n, len(g.slots)); g.width++ {
		if s := g.slots[g.width]; g.p.free(s.x, s.y) {
			g.p.set(s.x, s.y, Leaf)
		}
	}
}

// step widens the base, and every fourth step or so raises the stalk from
// its middle.
func (g *rosette) step(top int) {
	full := int(baseShare * float64(len(g.slots)))
	g.widen(g.width + max(1, full/fullSteps[Rosette]))
	if g.r.Intn(4) == 0 && g.stalk < maxStalk {
		if y := ground - 1 - g.stalk; y >= max(top, 2) {
			g.p.set(center, y, Stem) // through the base's leaves
			g.stalk++
		}
	}
}

// maxStalk is how tall a rosette's stalk grows, from the ground.
const maxStalk = 16

// The rosette's canopy is the ring of slots around its base, innermost
// first: a busy rosette spreads wider.
func (g *rosette) leafSpots() []pt {
	var out []pt
	for _, s := range g.slots[g.width:] {
		if g.p.free(s.x, s.y) {
			out = append(out, s)
		}
	}
	return out
}

func (g *rosette) flowerSpots() []pt {
	var out []pt
	if g.stalk > 0 {
		for y := ground - g.stalk; y <= ground-g.stalk+2; y++ {
			for _, x := range []int{center - 1, center + 1} {
				if g.p.free(x, y) {
					out = append(out, pt{x, y})
				}
			}
		}
		if y := ground - 1 - g.stalk; g.p.free(center, y) {
			out = append(out, pt{center, y})
		}
	}
	if len(out) == 0 {
		out = g.leafSpots()
	}
	return g.shuffled(out, 2)
}

func dedup(ps []pt) []pt {
	seen := map[pt]bool{}
	var out []pt
	for _, q := range ps {
		if !seen[q] {
			seen[q] = true
			out = append(out, q)
		}
	}
	return out
}

// stepsFor is how many structural steps a skeleton takes at a push count:
// the same log curve as its height.
func stepsFor(sp Species, pushes int) int {
	return int(float64(fullSteps[sp]) * growthFrac(pushes))
}

// stepTop is the highest row the skeleton may reach at step k of n: the
// height limit rises with the steps taken, as it rises with pushes.
func stepTop(k, n int) int {
	return ground - (seedlingHeight + k*(Height-2-seedlingHeight)/max(n, 1))
}
```

- [ ] **Step 4: Totals, the skeleton and the canopy**

`internal/garden/canopy.go`:

```go
package garden

import (
	"hash/fnv"
	"math/rand"
	"sort"
	"time"
)

// Totals counts a repo's whole history. A plant's skeleton grows from them,
// so it needs no more than the totals; its canopy grows from recent events.
type Totals struct {
	Pushes, Merges, Releases, OpenIssues int
}

// TotalsOf counts events at or before at; a zero at counts them all.
func TotalsOf(events []Event, at time.Time) Totals {
	var t Totals
	for _, e := range events {
		if !at.IsZero() && e.At.After(at) {
			continue
		}
		switch e.Kind {
		case Push:
			t.Pushes++
		case Merge:
			t.Merges++
		case Release:
			t.Releases++
		case IssueOpened:
			t.OpenIssues++
		case IssueClosed:
			if t.OpenIssues > 0 {
				t.OpenIssues--
			}
		}
	}
	return t
}

// How long a canopy remembers: leaves come from pushes of the last
// leafWindow, flowers from merges of the last flowerWindow (in bloom for
// flowerLife, then gone to seed), fruit from up to maxFruit releases of the
// last fruitWindow.
const (
	leafWindow   = 90 * 24 * time.Hour
	flowerWindow = 90 * 24 * time.Hour
	fruitWindow  = 365 * 24 * time.Hour
	maxFruit     = 4
)

// GrowAt grows a plant as of at: its skeleton from the totals, its canopy
// from the events inside its windows. events may be the whole history or
// only the recent part, in any order; the larger of t and what events count
// is used. A zero at means the time of the newest event.
func GrowAt(name string, sp Species, t Totals, events []Event, at time.Time) *Plant {
	if at.IsZero() {
		for _, e := range events {
			if e.At.After(at) {
				at = e.At
			}
		}
	}
	c := TotalsOf(events, at)
	t = Totals{max(t.Pushes, c.Pushes), max(t.Merges, c.Merges), max(t.Releases, c.Releases), max(t.OpenIssues, c.OpenIssues)}

	p, h := skeleton(name, sp, t)
	for _, e := range events {
		if !e.At.After(at) && e.Kind.Tends() && e.At.After(p.LastTended) {
			p.LastTended = e.At
		}
	}
	growCanopy(p, h, events, at)
	return p
}

// skeleton grows a plant's structure from its totals, and returns it with
// its habit, ready for a canopy.
func skeleton(name string, sp Species, t Totals) (*Plant, habit) {
	r := rand.New(rand.NewSource(seedOf(name)))
	p := &Plant{Name: name, Species: sp, Pushes: t.Pushes, Merges: t.Merges, Releases: t.Releases, OpenIssues: t.OpenIssues}
	for x := 0; x < Width; x++ {
		if x < center-1 || x > center+1 {
			p.weedSlots = append(p.weedSlots, x)
		}
	}
	r.Shuffle(len(p.weedSlots), func(i, j int) {
		p.weedSlots[i], p.weedSlots[j] = p.weedSlots[j], p.weedSlots[i]
	})
	h := newHabit(sp, p, r)
	for k := 0; k < stepsFor(sp, t.Pushes); k++ {
		h.step(stepTop(k, fullSteps[sp]))
	}
	return p, h
}

// growCanopy puts the leaves, flowers and fruit on a grown skeleton.
func growCanopy(p *Plant, h habit, events []Event, at time.Time) {
	spots := h.leafSpots()
	pool := spots[:len(spots)/2] // the leaf budget: half the silhouette
	for _, q := range pool[:len(pool)/3] {
		p.Grid[q.y][q.x] = Cell{Kind: Leaf} // dormant leaves, from the plant's own order
	}
	if len(pool) > 0 {
		for _, e := range events {
			if e.Kind != Push || e.At.After(at) || at.Sub(e.At) > leafWindow {
				continue
			}
			k := eventHash(p.Name, e)
			for _, i := range []uint64{k, k >> 20} { // a cluster of up to two leaves
				q := pool[i%uint64(len(pool))]
				cell := &p.Grid[q.y][q.x]
				if cell.At != 0 && cell.Level < maxLevel { // a second recent push: lusher
					cell.Level++
				}
				cell.Kind, cell.At = Leaf, max(cell.At, e.At.Unix()) // the newest push it grew from
			}
		}
	}
	place(p, h.flowerSpots(), events, at, Merge, flowerWindow, len(pool)/4, Flower)
	place(p, base{p: p}.shuffled(pool, 3), events, at, Release, fruitWindow, maxFruit, Fruit) // fruit hangs among the leaves
}

// place puts one cell of kind k per event of kind ek inside window, newest
// first and at most limit of them, each on a spot picked by the event's own
// hash; a taken spot sends it to the next.
func place(p *Plant, spots []pt, events []Event, at time.Time, ek EventKind, window time.Duration, limit int, k CellKind) {
	if len(spots) == 0 {
		return
	}
	var in []Event
	for _, e := range events {
		if e.Kind == ek && !e.At.After(at) && at.Sub(e.At) <= window {
			in = append(in, e)
		}
	}
	sort.SliceStable(in, func(i, j int) bool { return in[i].At.After(in[j].At) })
	for n, e := range in {
		if n == limit {
			return
		}
		h := eventHash(p.Name, e)
		for try := uint64(0); try < uint64(len(spots)); try++ {
			q := spots[(h+try)%uint64(len(spots))]
			if cell := &p.Grid[q.y][q.x]; cell.Kind != Flower && cell.Kind != Fruit {
				*cell = Cell{Kind: k, At: e.At.Unix()}
				break
			}
		}
	}
}

// eventHash seeds where an event's growth goes: the same event on the same
// plant always lands in the same place.
func eventHash(name string, e Event) uint64 {
	h := fnv.New64a()
	h.Write([]byte(name))
	var b [9]byte
	u := uint64(e.At.UnixNano())
	for i := 0; i < 8; i++ {
		b[i] = byte(u >> (8 * i))
	}
	b[8] = byte(e.Kind)
	h.Write(b[:])
	return h.Sum64()
}
```

In `internal/garden/plant.go`, replace the whole `Grow` function, from its comment `// Grow builds a plant from scratch by replaying events in order.` to its closing brace, with:

```go
// Grow grows a plant from a repo's whole history, as of its last event.
// The same name, species and events always produce the same plant.
func Grow(name string, sp Species, events []Event) *Plant {
	return GrowAt(name, sp, Totals{}, events, time.Time{})
}
```

Then delete the now unused `"math/rand"` import from `plant.go`.

- [ ] **Step 5: Run the garden tests**

Run: `gofmt -w internal/garden && go vet ./internal/garden/ && go test ./internal/garden/`
Expected: PASS. That covers the new tests and every existing garden test, including `TestTinyReposAreSeedlings` and `TestPlantsGrowWithTheLogOfPushes`.

- [ ] **Step 6: Regenerate the golden frame, once**

Run: `go test ./... 2>&1 | grep -v '^ok'`
Expected: only `TestComposeGolden` fails, because the demo plants in it now grow the new way.

Run: `go test ./internal/scene/ -run TestComposeGolden -update && go test ./...`
Expected: PASS.

- [ ] **Step 7: The frame budget**

Run: `go test ./internal/garden/ -bench Grow50kEvents -run '^$'`
Expected: `BenchmarkGrow50kEvents` reports under `2000000 ns/op`.

- [ ] **Step 8: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/garden internal/scene/testdata/garden.golden
git commit -m "garden: grow a skeleton from the totals and a canopy from recent activity

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: GitHub: totals and recent history

**Files:**
- Create: `internal/github/recent_test.go`
- Modify: `internal/github/repo.go`, `internal/github/client.go`, `internal/github/sync.go`, `internal/github/agents_test.go`

**Interfaces:**
- Consumes:
  - Task 1's `garden.Totals`
  - the test helper `fakeGitHub` and the constant `emptyConn`, from `sync_test.go`
- Produces:
  - `type Totals struct{ Commits, Merged, Releases, OpenIssues int }`, with JSON keys `commits`, `merged`, `releases` and `openIssues`
  - `Repo.Totals *Totals` (JSON `totals,omitempty`)
  - `Repo.History string` (JSON `history,omitempty`); `"recent"` marks an incomplete history
  - `(*Repo).Complete() bool` and `(*Repo).GardenTotals() garden.Totals`
  - `type HistoryMode int`, with `FullHistory` (the zero value) and `RecentHistory`
  - `const RecentWindow = 90 * 24 * time.Hour`
  - `Client.History HistoryMode`
  - unexported:
    - `(*Client).totals`
    - the generic `pages[T]`
    - `commitsSince(ctx, owner, name, since, pages)`

- [ ] **Step 1: Write the failing tests**

`internal/github/recent_test.go`:

```go
package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
)

type gqlCall struct {
	Query     string
	Variables map[string]any
}

// recordingGitHub is fakeGitHub that also keeps every call, with its
// variables, for the test to inspect.
func recordingGitHub(t *testing.T, answer func(gqlCall) string) (*Client, func() []gqlCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []gqlCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body gqlCall
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		mu.Lock()
		calls = append(calls, body)
		mu.Unlock()
		io.WriteString(w, answer(body))
	}))
	t.Cleanup(srv.Close)
	return &Client{token: "test", hc: srv.Client(), url: srv.URL}, func() []gqlCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]gqlCall(nil), calls...)
	}
}

const totalsAnswer = `{"data":{"repository":{"defaultBranchRef":{"target":{"history":{"totalCount":4200}}},
	"merged":{"totalCount":900},"releases":{"totalCount":35},"openIssues":{"totalCount":120}}}}`

func conn(nodes string) string {
	return `{"data":{"repository":{"conn":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[` + nodes + `]}}}}`
}

func history(commits string) string {
	return `{"data":{"repository":{"defaultBranchRef":{"target":{"history":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[` + commits + `]}}}}}}`
}

func ts(d time.Duration) string { return time.Now().Add(-d).UTC().Format(time.RFC3339) }

const day = 24 * time.Hour

func TestRecentFetchesTotalsAndRecentHistory(t *testing.T) {
	c, calls := recordingGitHub(t, func(call gqlCall) string {
		q := call.Query
		switch {
		case strings.Contains(q, "totalCount"):
			return totalsAnswer
		case strings.Contains(q, "history("):
			return history(`{"committedDate":"` + ts(2*day) + `","messageHeadline":"recent","authors":{"nodes":[]}}`)
		case strings.Contains(q, "states:MERGED"):
			return conn(`{"number":41,"title":"in the window","mergedAt":"` + ts(3*day) + `","updatedAt":"` + ts(3*day) + `"},
				{"number":12,"title":"merged long ago, commented on lately","mergedAt":"` + ts(400*day) + `","updatedAt":"` + ts(5*day) + `"},
				{"number":9,"title":"past the window","mergedAt":"` + ts(200*day) + `","updatedAt":"` + ts(100*day) + `"}`)
		case strings.Contains(q, "states:CLOSED"):
			return conn(`{"number":30,"title":"closed lately","createdAt":"` + ts(300*day) + `","closedAt":"` + ts(4*day) + `"},
				{"number":20,"title":"closed long ago, edited lately","createdAt":"` + ts(300*day) + `","closedAt":"` + ts(200*day) + `"}`)
		case strings.Contains(q, "issues(") && strings.Contains(q, "states:OPEN"):
			return conn(`{"number":31,"title":"open","createdAt":"` + ts(1*day) + `","closedAt":null}`)
		case strings.Contains(q, "releases("):
			return conn(`{"tagName":"v2.0.0","createdAt":"` + ts(700*day) + `","isDraft":false}`)
		}
		return emptyConn
	})
	c.History = RecentHistory
	r := &Repo{NameWithOwner: "me/big"}
	if err := c.fetch(context.Background(), r, nil); err != nil {
		t.Fatal(err)
	}
	if want := (Totals{Commits: 4200, Merged: 900, Releases: 35, OpenIssues: 120}); r.Totals == nil || *r.Totals != want {
		t.Errorf("totals = %+v, want %+v", r.Totals, want)
	}
	if r.History != "recent" || r.Complete() {
		t.Errorf("history %q: a recent fetch is not complete", r.History)
	}
	if len(r.Commits) != 1 || len(r.PRs) != 1 || r.PRs[0].Number != 41 {
		t.Errorf("commits %d, PRs %+v; want the one commit and #41", len(r.Commits), r.PRs)
	}
	if len(r.Issues) != 2 || r.Issues[0].Number != 31 || r.Issues[1].Number != 30 {
		t.Errorf("issues %+v; want open #31 and #30 closed lately", r.Issues)
	}
	if len(r.Releases) != 1 || r.Releases[0].Tag != "v2.0.0" {
		t.Errorf("releases %+v; want the newest, however old", r.Releases)
	}
	for _, call := range calls() {
		q := call.Query
		switch {
		case strings.Contains(q, "history("):
			since, _ := call.Variables["since"].(string)
			if at, err := time.Parse(time.RFC3339, since); err != nil || time.Since(at) > RecentWindow+day || time.Since(at) < RecentWindow-day {
				t.Errorf("commits since %q; want about 90 days ago", since)
			}
		case strings.Contains(q, "direction:ASC") && !strings.Contains(q, "pullRequests(first:100,after:$after,states:OPEN"): // open PRs: as before
			t.Errorf("a recent fetch should not page through whole histories: %s", q)
		case strings.Contains(q, "releases(") && !strings.Contains(q, "first:10,"):
			t.Errorf("releases: want the ten newest: %s", q)
		}
	}
}

func TestRecentQuietRepoKeepsItsNewestCommit(t *testing.T) {
	c, _ := recordingGitHub(t, func(call gqlCall) string {
		switch q := call.Query; {
		case strings.Contains(q, "totalCount"):
			return totalsAnswer
		case strings.Contains(q, "history("):
			if call.Variables["since"] != nil {
				return history(``) // nothing in the last 90 days
			}
			return history(`{"committedDate":"` + ts(300*day) + `","messageHeadline":"last one","authors":{"nodes":[]}}`)
		}
		return emptyConn
	})
	c.History = RecentHistory
	r := &Repo{NameWithOwner: "me/quiet"}
	if err := c.fetch(context.Background(), r, nil); err != nil {
		t.Fatal(err)
	}
	if len(r.Commits) != 1 || r.Commits[0].Message != "last one" {
		t.Errorf("commits %+v; want the newest one even outside the window", r.Commits)
	}
}

func TestCompleteCacheServesRecent(t *testing.T) {
	newest := time.Now().Add(-10 * day).UTC().Truncate(time.Second)
	cached := &Repo{NameWithOwner: "me/x", Commits: []Commit{{At: newest.Add(-900 * day)}, {At: newest}}} // from before v0.8: complete
	c, calls := recordingGitHub(t, func(call gqlCall) string {
		if strings.Contains(call.Query, "totalCount") {
			return totalsAnswer
		}
		if strings.Contains(call.Query, "history(") {
			return history(``)
		}
		return emptyConn
	})
	c.History = RecentHistory
	r := &Repo{NameWithOwner: "me/x"}
	if err := c.fetch(context.Background(), r, cached); err != nil {
		t.Fatal(err)
	}
	if !r.Complete() || len(r.Commits) != 2 {
		t.Errorf("history %q with %d commits: a complete cache stays complete", r.History, len(r.Commits))
	}
	for _, call := range calls() {
		if strings.Contains(call.Query, "history(") && call.Variables["since"] != newest.Add(time.Second).Format(time.RFC3339) {
			t.Errorf("commits since %v; want only those after the newest cached one", call.Variables["since"])
		}
	}
}

func TestFullFetchesWhatARecentCacheLacks(t *testing.T) {
	cached := &Repo{NameWithOwner: "me/x", History: "recent", FetchedAt: time.Now(), Commits: []Commit{{At: time.Now().Add(-day)}}}
	c, calls := recordingGitHub(t, func(call gqlCall) string {
		if strings.Contains(call.Query, "totalCount") {
			return totalsAnswer
		}
		if strings.Contains(call.Query, "history(") {
			return history(`{"committedDate":"` + ts(day) + `","messageHeadline":"a","authors":{"nodes":[]}},
				{"committedDate":"` + ts(900*day) + `","messageHeadline":"b","authors":{"nodes":[]}}`)
		}
		return emptyConn
	})
	s := &Store{path: filepath.Join(t.TempDir(), "repos.json"), Repos: map[string]*Repo{"me/x": cached}}
	repos, err := Sync(context.Background(), c, s, []*Repo{{NameWithOwner: "me/x"}}, time.Hour, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fetched := len(calls()) > 0; !fetched || !repos[0].Complete() || len(repos[0].Commits) != 2 {
		t.Errorf("fetched %v, history %q, %d commits: full history must fetch past a fresh recent cache",
			len(calls()) > 0, repos[0].History, len(repos[0].Commits))
	}
	for _, call := range calls() {
		if strings.Contains(call.Query, "history(") && call.Variables["since"] != nil {
			t.Errorf("commits since %v; want all of them", call.Variables["since"])
		}
	}
}

func TestOldCachesHaveNoTotals(t *testing.T) {
	old := `{"repos":{"me/x":{"nameWithOwner":"me/x","commits":[{"at":"2026-01-01T00:00:00Z","msg":"a"}],"fetchedAt":"2026-09-01T00:00:00Z"}}}`
	var s Store
	if err := json.Unmarshal([]byte(old), &s); err != nil {
		t.Fatal(err)
	}
	r := s.Repos["me/x"]
	if r.Totals != nil || r.GardenTotals() != (garden.Totals{}) || !r.Complete() {
		t.Errorf("a v0.7 cache: totals %+v, complete %v; want none, and complete", r.Totals, r.Complete())
	}
}

func TestRecentEmptyRepo(t *testing.T) {
	c, _ := recordingGitHub(t, func(call gqlCall) string {
		switch q := call.Query; {
		case strings.Contains(q, "totalCount"):
			return `{"data":{"repository":{"defaultBranchRef":null,"merged":{"totalCount":0},"releases":{"totalCount":0},"openIssues":{"totalCount":0}}}}`
		case strings.Contains(q, "history("):
			return `{"data":{"repository":{"defaultBranchRef":null}}}`
		}
		return emptyConn
	})
	c.History = RecentHistory
	r := &Repo{NameWithOwner: "me/empty"}
	if err := c.fetch(context.Background(), r, nil); err != nil {
		t.Fatalf("an empty repo should fetch cleanly: %v", err)
	}
	if r.Totals == nil || r.Totals.Commits != 0 || len(r.Commits) != 0 || r.History != "recent" {
		t.Errorf("totals %+v, %d commits, history %q", r.Totals, len(r.Commits), r.History)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/github/`
Expected: FAIL, with build errors such as `undefined: RecentHistory`, `c.History undefined` and `r.Totals undefined`.

- [ ] **Step 3: The repo's totals and history mark**

In `internal/github/repo.go`, replace

```go
	CI            string    `json:"ci,omitempty"`     // statusCheckRollup state of its latest commit
	FetchedAt     time.Time `json:"fetchedAt"`
}
```

with

```go
	CI            string    `json:"ci,omitempty"`     // statusCheckRollup state of its latest commit
	Totals        *Totals   `json:"totals,omitempty"`  // GitHub's counts; nil in caches from before v0.8
	History       string    `json:"history,omitempty"` // "recent" when only recent history was fetched
	FetchedAt     time.Time `json:"fetchedAt"`
}

// Totals are GitHub's counts for a repo's whole history.
type Totals struct {
	Commits    int `json:"commits"` // on the default branch
	Merged     int `json:"merged"`  // merged PRs
	Releases   int `json:"releases"`
	OpenIssues int `json:"openIssues"`
}

// HistoryMode is how much of each repo's history Sync fetches.
type HistoryMode int

const (
	FullHistory   HistoryMode = iota // everything, as far as the page caps reach
	RecentHistory                    // the totals, and the last RecentWindow in detail
)

// RecentWindow is how far back RecentHistory fetches commits, merged PRs
// and closed issues.
const RecentWindow = 90 * 24 * time.Hour

// recentReleases is how many of the newest releases RecentHistory fetches,
// whatever their age.
const recentReleases = 10

// Complete reports whether r holds its whole history rather than only the
// recent part. Caches from before v0.8 always fetched everything.
func (r *Repo) Complete() bool { return r.History != "recent" }

// GardenTotals are r's totals as a plant grows from them; zero when GitHub's
// counts are unknown, and the plant counts its events instead.
func (r *Repo) GardenTotals() garden.Totals {
	if r.Totals == nil {
		return garden.Totals{}
	}
	return garden.Totals{Pushes: r.Totals.Commits, Merges: r.Totals.Merged, Releases: r.Totals.Releases, OpenIssues: r.Totals.OpenIssues}
}
```

In `internal/github/client.go`, replace

```go
	url   string // GraphQL endpoint; tests point it at a fake server
}
```

with

```go
	url   string // GraphQL endpoint; tests point it at a fake server

	History HistoryMode // how much history Sync fetches; FullHistory by default
}
```

- [ ] **Step 4: The fetch**

In `internal/github/sync.go`:

1. Replace the `commitsSince` comment and signature with:

```go
// commitsSince fetches default-branch commits newer than since (all of them
// when since is zero), newest first, in at most pages pages of 100.
func (c *Client) commitsSince(ctx context.Context, owner, name string, since time.Time, pages int) ([]Commit, error) {
```

   In its body, replace `for page := 0; page < maxCommitPages; page++ {` with `for page := 0; page < pages; page++ {`.

2. Replace the whole `connection` function, from `// connection pages through a repository connection that supports` to its closing brace, with:

```go
// connection pages through a repository connection that supports
// orderBy CREATED_AT, oldest first. args are extra connection arguments.
func connection[T any](ctx context.Context, c *Client, owner, name, field, args, nodeFields string) ([]T, error) {
	return pages[T](ctx, c, owner, name, field, args+",orderBy:{field:CREATED_AT,direction:ASC}", nodeFields, 100, maxOtherPages, nil)
}

// pages pages through a repository connection, first nodes a page and at
// most max pages. args are its arguments, orderBy included. A non-nil keep
// ends it at the first node it rejects: the connection is ordered so that
// every node after that one is unwanted too.
func pages[T any](ctx context.Context, c *Client, owner, name, field, args, nodeFields string, first, max int, keep func(T) bool) ([]T, error) {
	q := fmt.Sprintf(`query($owner:String!,$name:String!,$after:String){repository(owner:$owner,name:$name){conn: %s(first:%d,after:$after%s){pageInfo{hasNextPage endCursor} nodes{%s}}}}`,
		field, first, args, nodeFields)
	vars := map[string]any{"owner": owner, "name": name, "after": nil}
	var all []T
	for page := 0; page < max; page++ {
		var out struct {
			Repository struct {
				Conn struct {
					PageInfo pageInfo
					Nodes    []T
				}
			}
		}
		if err := c.query(ctx, q, vars, &out); err != nil {
			return nil, err
		}
		for _, n := range out.Repository.Conn.Nodes {
			if keep != nil && !keep(n) {
				return all, nil
			}
			all = append(all, n)
		}
		if !out.Repository.Conn.PageInfo.HasNextPage {
			break
		}
		vars["after"] = out.Repository.Conn.PageInfo.EndCursor
	}
	return all, nil
}

// totals fetches GitHub's counts for a repo's whole history.
func (c *Client) totals(ctx context.Context, owner, name string) (*Totals, error) {
	q := `query($owner:String!,$name:String!){repository(owner:$owner,name:$name){defaultBranchRef{target{... on Commit{history{totalCount}}}} merged: pullRequests(states:MERGED){totalCount} releases{totalCount} openIssues: issues(states:OPEN){totalCount}}}`
	type count struct{ TotalCount int }
	var out struct {
		Repository struct {
			DefaultBranchRef *struct {
				Target struct{ History count }
			}
			Merged, Releases, OpenIssues count
		}
	}
	if err := c.query(ctx, q, map[string]any{"owner": owner, "name": name}, &out); err != nil {
		return nil, err
	}
	rep := out.Repository
	t := &Totals{Merged: rep.Merged.TotalCount, Releases: rep.Releases.TotalCount, OpenIssues: rep.OpenIssues.TotalCount}
	if rep.DefaultBranchRef != nil {
		t.Commits = rep.DefaultBranchRef.Target.History.TotalCount
	}
	return t, nil
}
```

3. Replace the start of `fetch`, from `// fetch fills in r's history. Commits are fetched incrementally on top of` down to, but not including, the line `	type openPRNode struct {`, with:

```go
// fetch fills in r's totals and history. Commits are fetched incrementally
// on top of cached ones. With FullHistory, PRs, issues and releases are
// refetched whole, which also picks up issues closed since last time; with
// RecentHistory only the recent ones are, newest first.
func (c *Client) fetch(ctx context.Context, r *Repo, cached *Repo) error {
	owner, name, _ := strings.Cut(r.NameWithOwner, "/")
	recent := c.History == RecentHistory
	cutoff := time.Now().Add(-RecentWindow)

	totals, err := c.totals(ctx, owner, name)
	if err != nil {
		return fmt.Errorf("totals: %w", err)
	}
	r.Totals = totals

	complete := !recent
	var since time.Time
	if cached != nil && (recent || cached.Complete()) { // full history can't build on a recent cache
		r.Commits, complete = cached.Commits, cached.Complete()
		for _, cm := range cached.Commits {
			if cm.At.After(since) {
				since = cm.At
			}
		}
	}
	if since.IsZero() && recent {
		since = cutoff
	}
	newer, err := c.commitsSince(ctx, owner, name, since, maxCommitPages)
	if err != nil {
		return fmt.Errorf("commits: %w", err)
	}
	r.Commits = append(r.Commits, newer...)
	if recent && len(r.Commits) == 0 { // a quiet repo: its newest commits still tell when it was tended
		if r.Commits, err = c.commitsSince(ctx, owner, name, time.Time{}, 1); err != nil {
			return fmt.Errorf("commits: %w", err)
		}
	}
	r.History = "full"
	if !complete {
		r.History = "recent"
	}

	type releaseNode struct {
		TagName   string
		CreatedAt time.Time
		IsDraft   bool
	}
	var prs []PR
	var issues []Issue
	var rels []releaseNode
	if recent {
		type prNode struct {
			PR
			UpdatedAt time.Time
		}
		var nodes []prNode
		nodes, err = pages(ctx, c, owner, name, "pullRequests", ",states:MERGED,orderBy:{field:UPDATED_AT,direction:DESC}",
			"number title mergedAt updatedAt", 100, maxOtherPages, func(n prNode) bool { return !n.UpdatedAt.Before(cutoff) })
		if err != nil {
			return fmt.Errorf("pull requests: %w", err)
		}
		for _, n := range nodes {
			if !n.MergedAt.Before(cutoff) {
				prs = append(prs, n.PR)
			}
		}
		if issues, err = pages[Issue](ctx, c, owner, name, "issues", ",states:OPEN,orderBy:{field:CREATED_AT,direction:DESC}",
			"number title createdAt closedAt", 100, 1, nil); err != nil {
			return fmt.Errorf("issues: %w", err)
		}
		closed, err := pages[Issue](ctx, c, owner, name, "issues",
			fmt.Sprintf(",states:CLOSED,filterBy:{since:%q},orderBy:{field:UPDATED_AT,direction:DESC}", cutoff.UTC().Format(time.RFC3339)),
			"number title createdAt closedAt", 100, maxOtherPages, nil)
		if err != nil {
			return fmt.Errorf("issues: %w", err)
		}
		for _, is := range closed {
			if is.ClosedAt != nil && !is.ClosedAt.Before(cutoff) {
				issues = append(issues, is)
			}
		}
		if rels, err = pages[releaseNode](ctx, c, owner, name, "releases", ",orderBy:{field:CREATED_AT,direction:DESC}",
			"tagName createdAt isDraft", recentReleases, 1, nil); err != nil {
			return fmt.Errorf("releases: %w", err)
		}
	} else {
		if prs, err = connection[PR](ctx, c, owner, name, "pullRequests", ",states:MERGED", "number title mergedAt"); err != nil {
			return fmt.Errorf("pull requests: %w", err)
		}
		if issues, err = connection[Issue](ctx, c, owner, name, "issues", "", "number title createdAt closedAt"); err != nil {
			return fmt.Errorf("issues: %w", err)
		}
		if rels, err = connection[releaseNode](ctx, c, owner, name, "releases", "", "tagName createdAt isDraft"); err != nil {
			return fmt.Errorf("releases: %w", err)
		}
	}
	r.PRs, r.Issues, r.Releases = prs, issues, nil
	for _, rel := range rels {
		if !rel.IsDraft {
			r.Releases = append(r.Releases, Release{Tag: rel.TagName, CreatedAt: rel.CreatedAt})
		}
	}
```

4. In `Sync`, replace

```go
		if cached != nil && time.Since(cached.FetchedAt) < ttl {
			m.Commits, m.PRs, m.Issues, m.Releases, m.FetchedAt = cached.Commits, cached.PRs, cached.Issues, cached.Releases, cached.FetchedAt
			m.OpenPRs, m.Branch, m.CI = cached.OpenPRs, cached.Branch, cached.CI
```

   with

```go
		if cached != nil && time.Since(cached.FetchedAt) < ttl && (cached.Complete() || c == nil || c.History == RecentHistory) {
			m.Commits, m.PRs, m.Issues, m.Releases, m.FetchedAt = cached.Commits, cached.PRs, cached.Issues, cached.Releases, cached.FetchedAt
			m.OpenPRs, m.Branch, m.CI = cached.OpenPRs, cached.Branch, cached.CI
			m.Totals, m.History = cached.Totals, cached.History
```

In `internal/github/agents_test.go`, replace `c.commitsSince(context.Background(), "me", "x", time.Time{})` with `c.commitsSince(context.Background(), "me", "x", time.Time{}, maxCommitPages)`.

- [ ] **Step 5: Run the tests**

Run: `gofmt -w internal/github && go vet ./internal/github/ && go test -race ./internal/github/ && go test ./...`
Expected: PASS. The existing sync tests still pass for two reasons:
- a client's zero `History` is `FullHistory`
- their fakes answer the totals query with an empty connection, which reads as zero totals

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/github
git commit -m "github: fetch GitHub's totals, and in recent mode only the last 90 days in detail

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Config: the history setting

**Files:**
- Modify: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `Config.History string`, which is `"recent"` or `"full"`; the key `"history"` last in `Keys`.

- [ ] **Step 1: Write the failing tests**

In `internal/config/config_test.go`:

1. In `TestParse`, change the input line
   `src := "﻿# my garden\r\nsky = Stars  # opt in\r\n\r\nLIMIT=12\nrepos = me/a, other/b\nuser = octocat\nrefresh = 2m\ndecay = 30\n"`
   to
   `src := "﻿# my garden\r\nsky = Stars  # opt in\r\n\r\nLIMIT=12\nrepos = me/a, other/b\nuser = octocat\nrefresh = 2m\ndecay = 30\nhistory = FULL\n"`

   Then change its `want` to:

```go
	want := Config{Sky: "stars", Limit: 12, Repos: []string{"me/a", "other/b"}, User: "octocat",
		Refresh: 2 * time.Minute, Decay: 30, History: "full"}
```

2. In `TestDescribe`, add the line `"history  recent  (default)",` after `"decay    45      (default)",`.
3. Append:

```go
func TestHistoryIsRecentOrFull(t *testing.T) {
	if c := Defaults(); c.History != "recent" {
		t.Errorf("default history = %q, want recent", c.History)
	}
	c, warns := Parse(strings.NewReader("history = everything\n"))
	if c.History != "recent" || len(warns) != 1 || !strings.Contains(warns[0].String(), "isn't recent or full") {
		t.Errorf("history %q, warnings %v; want the default and one warning", c.History, warns)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config/`
Expected: FAIL, with the build error `unknown field History in struct literal of type Config`.

- [ ] **Step 3: Implement**

In `internal/config/config.go`:

1. In `type Config struct`, add after `Decay   float64`:

```go
	History string            // "recent" or "full": how much history GAG fetches
```

2. Change `Keys` to `var Keys = []string{"sky", "limit", "repos", "user", "refresh", "decay", "history"}`.
3. In `Defaults`, change the literal to `Config{Sky: "random", Limit: 8, Refresh: 5 * time.Minute, Decay: 45, History: "recent", From: map[string]Source{}}`.
4. In `set`, add before `default:`:

```go
	case "history":
		switch v := strings.ToLower(val); v {
		case "recent", "full":
			c.History = v
			return ""
		}
		return fmt.Sprintf("history %q isn't recent or full; using %s", val, c.History)
```

5. In `value`, add after the `case "decay":` branch:

```go
	case "history":
		return c.History
```

- [ ] **Step 4: Run the tests**

Run: `gofmt -w internal/config && go test ./internal/config/ && go test ./...`
Expected: PASS. `TestGagConfigShowsTheFile` still passes, because it checks only the start of the output.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/config
git commit -m "config: history = recent or full

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Live: a jump in counters is not a merge

**Files:**
- Modify: `internal/live/changes.go`, `internal/live/changes_test.go`

**Interfaces:**
- Consumes: the test helper `baseRepo()` from `changes_test.go`.
- Produces: nothing new.

- [ ] **Step 1: Write the failing test**

Append to `internal/live/changes_test.go`:

```go
func TestTotalsArrivingAreNoNews(t *testing.T) {
	before := baseRepo() // its counters counted from a cache without totals
	after := before
	grown := *before.Plant
	grown.Pushes, grown.Merges, grown.OpenIssues = 4200, 2500, 120 // GitHub's totals, from the next fetch
	after.Plant = &grown
	if got := ChangesBetween([]Repo{before}, []Repo{after}); len(got) != 0 {
		t.Errorf("counters that jump without new commits, merges or issues: %+v", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/live/ -run TestTotalsArrivingAreNoNews`
Expected: FAIL: `counters that jump without new commits, merges or issues: [{Repo:gag-core Kind:1 Icon:🌸 Text:gag-core: merged a PR · +2498 more …`.

- [ ] **Step 3: Implement**

In `internal/live/changes.go`, in `changesOf`, replace `	if merges > 0 {` with:

```go
	if merges > 0 && d.LastMerge.At.After(bd.LastMerge.At) { // a counter can jump without a merge: when totals first arrive
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/live/ && go test ./...`
Expected: PASS, including every v0.7 merge test: each of them sets a newer `LastMerge`.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/live
git commit -m "live: a merge reaction needs a newer merge, not just a higher count

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: The garden, the card, replay and the README use it

**Files:**
- Create: `cmd/gag/history_test.go`
- Modify:
  - `cmd/gag/garden.go`
  - `cmd/gag/main.go`
  - `cmd/gag/detail.go`
  - `cmd/gag/demo.go`
  - `cmd/gag/detail_test.go`
  - `cmd/gag/garden_test.go`
  - `internal/replay/model.go`
  - `README.md`

**Interfaces:**
- Consumes:
  - Task 1's `garden.GrowAt` and `garden.Totals`
  - Task 2's `github.HistoryMode`, `FullHistory`, `RecentHistory`, `RecentWindow`, `Repo.Totals`, `Repo.History`, `(*Repo).GardenTotals()` and `Client.History`
  - Task 3's `config.Config.History`
- Produces:
  - `repoFor(r *github.Repo, now, at time.Time) live.Repo`
  - `loadRepos(ctx, names, owner, limit, ttl, mode github.HistoryMode, log, progress)`
  - `historyMode(string) github.HistoryMode`
  - `const replayHistory`
  - `settings.history`, and `source.history` and `source.ahead`

- [ ] **Step 1: Write the failing tests**

`cmd/gag/history_test.go`:

```go
package main

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/config"
	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/github"
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./cmd/gag/`
Expected: FAIL, with build errors such as `too many arguments in call to repoFor`, `got.history undefined` and `undefined: replayHistory`.

- [ ] **Step 3: Grow from totals, at the right time**

In `cmd/gag/garden.go`:

1. In `runGarden`, replace the `src := source{…}` line with:

```go
	src := source{names: opts.repos, owner: opts.user, limit: opts.limit, ttl: *ttl, demo: *demoFlag, decay: opts.decay,
		history: opts.history, ahead: ahead, state: &sourceState{}}
```

2. In `type source struct`, add after the `state` field:

```go

	history github.HistoryMode // how much history to fetch
	ahead   time.Duration      // -simulate: plants grow as of this far ahead
```

3. In `snapshot`:
   - replace `loadRepos(ctx, s.names, s.owner, s.limit, ttl, nil, report)` with `loadRepos(ctx, s.names, s.owner, s.limit, ttl, s.history, nil, report)`
   - replace `repoFor(r, now)` with `repoFor(r, now, now.Add(s.ahead))`
4. In `onceView`:
   - replace `loadRepos(context.Background(), src.names, src.owner, src.limit, src.ttl, log, progress)` with `loadRepos(context.Background(), src.names, src.owner, src.limit, src.ttl, src.history, log, progress)`
   - replace `live.Plot(repoFor(r, now), at, src.decay)` with `live.Plot(repoFor(r, now, at), at, src.decay)`
5. Replace the `repoFor` comment and its first two lines with:

```go
// repoFor works out a repo's signals as of now, and grows its plant as of
// at: now, or later under -simulate.
func repoFor(r *github.Repo, now, at time.Time) live.Repo {
	lr := live.Repo{Name: r.Name(), Plant: garden.GrowAt(r.Name(), r.Species(), r.GardenTotals(), r.Events(), at),
```

6. In `type settings struct`, add after `decay   float64`:

```go
	history github.HistoryMode // set only in the file
```

7. In `merge`:
   - change the comment's last sentence to `The sky and history are set only in the file.`
   - add `history: historyMode(file.History)` to the `settings{…}` literal, after `decay: file.Decay`
8. Add before `func skyMode`:

```go
// historyMode is the config file's history setting as a fetch mode.
func historyMode(s string) github.HistoryMode {
	if s == "full" {
		return github.FullHistory
	}
	return github.RecentHistory
}
```

In `cmd/gag/main.go`:

1. Replace the `loadRepos` comment and signature with:

```go
// loadRepos returns synced repos: the named ones, or the limit most recently
// pushed by owner (your own repos when owner is empty), with as much history
// as mode fetches. If GitHub is unreachable it falls back to the local cache
// and reports offline. log gets progress lines ("fetching owner/repo"); nil
// discards them. progress, if not nil, hears (0, 0, "") while repos are
// listed, then Sync's per-repo progress.
func loadRepos(ctx context.Context, names []string, owner string, limit int, ttl time.Duration, mode github.HistoryMode, log func(string), progress func(done, total int, current string)) ([]*github.Repo, bool, error) {
```

2. In `loadRepos`, directly after the `github.NewClient()` error check, add `	c.History = mode`.
3. Add before `func runReplay`:

```go
// replayHistory is what replay fetches: a repo's whole life, whatever the
// history setting.
const replayHistory = github.FullHistory
```

4. In `runReplay`, replace `loadRepos(context.Background(), []string{*repo}, "", 1, *ttl, logf, nil)` with `loadRepos(context.Background(), []string{*repo}, "", 1, *ttl, replayHistory, logf, nil)`.

In `cmd/gag/detail.go`, in `detailFor`, add before the line `	for _, rel := range r.Releases {`:

```go
	if r.Totals != nil && r.Totals.OpenIssues > d.OpenIssues { // recent history lists only the newest open issues
		d.OpenIssues = r.Totals.OpenIssues
	}
```

In `cmd/gag/demo.go`, in `(*demoWorld).garden`, replace `garden.Grow(r.name, garden.SpeciesFor(r.lang), s.events)` with `garden.GrowAt(r.name, garden.SpeciesFor(r.lang), garden.Totals{}, s.events, now)`.

In `internal/replay/model.go`, in `View`, replace `p := garden.Grow(m.cfg.Name, m.cfg.Species, events)` with `p := garden.GrowAt(m.cfg.Name, m.cfg.Species, garden.Totals{}, events, m.clock)`.

The existing tests call `repoFor` with two arguments:
- In `cmd/gag/detail_test.go`, replace `repoFor(r, now).Detail` with `repoFor(r, now, now).Detail`.
- In `cmd/gag/garden_test.go`, replace `lr := repoFor(r, now)` with `lr := repoFor(r, now, now)`.
- In `cmd/gag/garden_test.go`, replace the body of `TestRepoForCarriesStars` with:

```go
	if now := time.Now(); repoFor(&github.Repo{NameWithOwner: "me/x", Stars: 42}, now, now).Stars != 42 {
		t.Error("the stars should reach the plant")
	}
```

- [ ] **Step 4: The README**

In `README.md`:

1. In the config file example, add after the line `# decay = 30`:

```
# history = full     # fetch whole histories (default: recent)
```

2. After the paragraph that starts `With \`sky = stars\``, add:

```markdown
`history` sets how much of each repo GAG downloads.
- `recent`, the default, fetches GitHub's totals plus the last 90 days in
  detail, so even big accounts load quickly.
- `full` fetches every commit, PR, issue and release, as before v0.8.

Plants look the same either way. `gag replay -repo` always fetches the
whole history.
```

3. Replace the section `## How plants grow`, from its heading down to, but not including, the paragraph that starts `The ticker at the bottom of the live view`, with:

```markdown
## How plants grow

`plant = GrowAt(repo name, species, totals, events, time)`. A plant grows
in two layers, both seeded by its name, so the same repo always grows the
same plant. Nothing is stored between runs.

- **Its skeleton** grows from the repo's totals, so a long-lived project is
  a big plant:
  - the stems, trunk and arms
  - the rosette's base and stalk

  It gets bigger with the log of all the repo's commits, reaching full size
  at about 500. It never rearranges itself as more commits arrive.
- **Its canopy** grows from recent activity, inside the species' silhouette,
  with air between the leaves. Even a huge repo stays a plant, not a block.

| Signal | In the garden |
|---|---|
| all commits | the plant's size and structure |
| pushes in the last 90 days | leaves, up to half the plant's silhouette; a third always stays in leaf |
| merged PR in the last 30 days | a flower, which goes to seed and drops after 90 days |
| release in the past year | fruit, up to 4 |
| open PR | a pink bud (up to 5); after a week of waiting it droops and fades |
| CI on the default branch | running: a small grey cloud · failing: a storm cloud with rain |
| more commits in the last 14 days than the 14 before | bright new shoots |
| open issues | weeds by the pot; 3+ new issues in a week bring a snail |
| time since last tended | leaves yellow → brown → fall, flowers droop |

```

- [ ] **Step 5: Run everything, including the browser build**

Run: `gofmt -l . ; go vet ./... && go test ./... && go test -race ./internal/live/`
Expected: PASS.

Build the browser demo, as `web/build.sh` does without the npm step:

Run: `WORK=$(mktemp -d) && cp -R "$(go list -m -f '{{.Dir}}' github.com/charmbracelet/bubbletea)" "$WORK/bubbletea" && chmod -R u+w "$WORK/bubbletea" && cp web/_bubbletea/*.go "$WORK/bubbletea/" && cp go.mod go.sum "$WORK/" && go mod edit -replace "github.com/charmbracelet/bubbletea=$WORK/bubbletea" "$WORK/go.mod" && GOOS=js GOARCH=wasm go vet -modfile="$WORK/go.mod" ./cmd/gag/ && GOOS=js GOARCH=wasm go build -modfile="$WORK/go.mod" -trimpath -ldflags "-s -w" -o "$WORK/gag.wasm" ./cmd/gag && ls -lh "$WORK/gag.wasm"; rm -rf "$WORK"`
Expected: `vet` prints nothing, and the build writes a `gag.wasm` of about 7.5 MB.

Also check that `grep -n 'internal/github' cmd/gag/demo.go` prints nothing.

- [ ] **Step 6: Commit**

```bash
git add cmd/gag internal/replay README.md
git commit -m "Plants grow from totals; history = recent fetches less; replay fetches everything

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Real-world check and the v0.8.0 release gate

**Files:**
- None, unless the check turns up bugs.

- [ ] **Step 1: Against real GitHub**

This is Review Focus 5. Run `go build -o gag ./cmd/gag`, then run each of these and check it completes without an error:
- `./gag garden --once` with no `history` setting. That is `recent`.
- `./gag garden --once -user charmbracelet`. It must be much faster than v0.7.1's first load of that account.
- `./gag replay -repo <one of your repos>`.
- `./gag garden --once` with `history = full` in `~/.config/gag/config`, then with the setting removed again.

A GraphQL error in any of them means a query the fake server accepted but GitHub doesn't. Fix it, with a test that pins the corrected query.

- [ ] **Step 2: Hand it to the user**

Ask the user to look at:
- their own garden, `./gag`
- a big one, `./gag garden -user charmbracelet`
- the demo, `./gag garden -demo`

No plant should fill its plot, and big repos should look like big, mature plants. The shrub's shape numbers (refinement 6) are theirs to adjust. Fix anything they report, each with a failing test first.

- [ ] **Step 3: Release gate**

Merging to `main` and tagging publish a public release. Do this only on the user's explicit yes:

```bash
git tag -a v0.8.0 -m "v0.8.0: compact plants and recent history"
git push origin main v0.8.0
gh run watch "$(gh run list -R RursusAeternum/GitAGarden -w release -L 1 --json databaseId --jq '.[0].databaseId')" -R RursusAeternum/GitAGarden --exit-status
gh workflow run update-gag.yml -R RursusAeternum/homebrew-tap
```
