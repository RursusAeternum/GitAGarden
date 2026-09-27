package garden

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func pushes(n int) []Event {
	ev := make([]Event, n)
	for i := range ev {
		ev[i] = Event{Kind: Push, At: t0.Add(time.Duration(i) * time.Hour)}
	}
	return ev
}

const padX, padY = 10, 6

func paintAlone(p *Plant, st Style) *pixel.Canvas {
	c := pixel.New(Width+padX, Height+padY)
	Paint(c, p, (Width+padX)/2, Height+padY/2, st)
	return c
}

func TestPaintIsDeterministic(t *testing.T) {
	p := Grow("repo", Shrub, pushes(60))
	a := paintAlone(p, Style{Health: 1}).Encode(pixel.TrueColor)
	b := paintAlone(p, Style{Health: 1}).Encode(pixel.TrueColor)
	if a != b {
		t.Error("same plant painted differently")
	}
}

func greenPixels(c *pixel.Canvas) int {
	n := 0
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if px := c.At(x, y); int(px.G) > int(px.R)+30 && px.G > px.B {
				n++
			}
		}
	}
	return n
}

func TestHealthyLeavesAreGreenWiltedAreNot(t *testing.T) {
	p := Grow("repo", Shrub, pushes(80))
	healthy := greenPixels(paintAlone(p, Style{Health: 1}))
	wilted := greenPixels(paintAlone(p, Style{Health: 0}))
	if healthy == 0 {
		t.Fatal("healthy plant has no green")
	}
	if wilted >= healthy/4 {
		t.Errorf("wilted plant is still green: %d vs %d pixels", wilted, healthy)
	}
}

func TestWeedsForOpenIssues(t *testing.T) {
	events := append(pushes(5),
		Event{Kind: IssueOpened, At: t0.Add(10 * time.Hour)},
		Event{Kind: IssueOpened, At: t0.Add(11 * time.Hour)},
		Event{Kind: IssueOpened, At: t0.Add(12 * time.Hour)})
	c := paintAlone(Grow("repo", Shrub, events), Style{Health: 1})
	n, baseY := 0, Height+padY/2
	for x := 0; x < c.W; x++ {
		if c.At(x, baseY) == weedColor {
			n++
		}
	}
	if n != 3 {
		t.Errorf("weed pixels on the ground row = %d, want 3", n)
	}
}

func TestSwayBendsTopNotBase(t *testing.T) {
	p := Grow("repo", Shrub, pushes(120))
	still := paintAlone(p, Style{Health: 1})
	swayed := paintAlone(p, Style{Health: 1, Sway: 4})
	baseY := Height + padY/2
	for x := 0; x < still.W; x++ {
		if still.At(x, baseY) != swayed.At(x, baseY) {
			t.Fatal("sway moved the base of the plant")
		}
	}
	if still.Encode(pixel.TrueColor) == swayed.Encode(pixel.TrueColor) {
		t.Error("sway changed nothing")
	}
}

func TestStatus(t *testing.T) {
	now := t0.Add(72 * time.Hour)
	if s := Status(Grow("r", Shrub, nil), now, false); s != "untended" {
		t.Errorf("empty repo status = %q, want untended", s)
	}
	p := Grow("r", Shrub, append(pushes(1), Event{Kind: IssueOpened, At: t0.Add(time.Hour)}))
	if s := Status(p, now, false); s != "3d ago · 1 issue" {
		t.Errorf("status = %q", s)
	}
	if s := Status(p, now, true); s != "finished" {
		t.Errorf("finished status = %q", s)
	}
}

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
