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
