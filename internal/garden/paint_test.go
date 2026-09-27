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
