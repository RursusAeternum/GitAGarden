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
