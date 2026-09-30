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

var sweepNames = []string{"gag-core", "rustyfs", "notebook-api", "dotfiles", "tiny-cli", "site-v2", "lsystem", "old-blog",
	"garden", "bubbletea", "lipgloss", "fzf", "vim-plug", "api", "web", "cli", "docs", "infra", "tools", "sandbox"}

func TestFlowersStayPutAsTimePasses(t *testing.T) {
	now := t0.Add(400 * day)
	ev := append(spread(Push, 300, 300*day, now), spread(Merge, 13, 60*day, now)...)
	later := now.Add(20 * day) // no new events; every merge is still inside the window
	for _, sp := range AllSpecies {
		for _, name := range sweepNames {
			a, b := GrowAt(name, sp, Totals{}, ev, now), GrowAt(name, sp, Totals{}, ev, later)
			for y := 0; y < Height; y++ {
				for x := 0; x < Width; x++ {
					if a.Grid[y][x].Kind == Flower && b.Grid[y][x].Kind != Flower {
						t.Errorf("%s %s: the flower at %d,%d moved while its merge is still in the window", sp, name, x, y)
					}
				}
			}
			if sp == Shrub && kinds(a, Flower) < 12 {
				t.Errorf("shrub %s: %d flowers for 13 recent merges", name, kinds(a, Flower))
			}
		}
	}
}

func TestAPushMovesFewLeaves(t *testing.T) {
	now := t0.Add(400 * day)
	all := spread(Push, 300, 80*day, now) // all inside the leaf window, at fixed times
	for _, sp := range AllSpecies {
		for _, name := range sweepNames[:8] {
			prev := GrowAt(name, sp, Totals{}, all[:2], now)
			for n := 3; n <= len(all); n++ {
				next := GrowAt(name, sp, Totals{}, all[:n], now)
				gone := 0
				for y := 0; y < Height; y++ {
					for x := 0; x < Width; x++ {
						if prev.Grid[y][x].Kind == Leaf && next.Grid[y][x].Kind != Leaf {
							gone++
						}
					}
				}
				// A push adds growth and takes nothing away. Only one that grows the
				// skeleton a step may shift the leaf pool's edge, by a few leaves.
				switch step := stepsFor(sp, n) != stepsFor(sp, n-1); {
				case !step && gone > 0:
					t.Errorf("%s %s: push %d took %d leaves away; a push adds growth, it doesn't reshuffle", sp, name, n, gone)
				case step && gone > 10:
					t.Errorf("%s %s: push %d grew a step and took %d leaves away", sp, name, n, gone)
				}
				prev = next
			}
		}
	}
}
