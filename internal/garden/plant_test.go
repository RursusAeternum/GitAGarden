package garden

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestGrowIsDeterministic(t *testing.T) {
	events := FakeHistory("repo", 200, t0)
	for _, sp := range AllSpecies {
		a, b := Grow("repo", sp, events), Grow("repo", sp, events)
		if a.Grid != b.Grid {
			t.Errorf("%s: same inputs produced different plants", sp)
		}
	}
}

func TestDifferentNamesGrowDifferently(t *testing.T) {
	events := FakeHistory("repo", 80, t0)
	if Grow("alpha", Shrub, events).Grid == Grow("beta", Shrub, events).Grid {
		t.Error("different repo names produced identical shrubs")
	}
}

func TestEveryEarlyPushAddsSomething(t *testing.T) {
	for _, sp := range AllSpecies {
		var events []Event
		prev := Grow("repo", sp, nil)
		for i := 0; i < 30; i++ {
			events = append(events, Event{Kind: Push, At: t0.Add(time.Duration(i) * time.Hour)})
			next := Grow("repo", sp, events)
			if next.Grid == prev.Grid {
				t.Fatalf("%s: push %d left the plant unchanged", sp, i+1)
			}
			prev = next
		}
	}
}

func TestLongHistoriesDoNotPanic(t *testing.T) {
	for _, sp := range AllSpecies {
		for _, name := range []string{"a", "b", "c", "d", "e"} {
			p := Grow(name, sp, FakeHistory(name, 2000, t0))
			c := pixel.New(Width+4, Height+4)
			Paint(c, p, (Width+4)/2, Height, Style{Health: 0.2, Sway: 3})
		}
	}
}

func TestHealthDecays(t *testing.T) {
	p := Grow("repo", Shrub, []Event{{Kind: Push, At: t0}})
	if h := Health(p, t0.Add(24*time.Hour), 45); h != 1 {
		t.Errorf("health a day after tending = %v, want 1", h)
	}
	if h := Health(p, t0.Add(24*time.Hour*20), 45); h <= 0 || h >= 1 {
		t.Errorf("health after 20 idle days = %v, want between 0 and 1", h)
	}
	if h := Health(p, t0.Add(24*time.Hour*100), 45); h != 0 {
		t.Errorf("health after 100 idle days = %v, want 0", h)
	}
}

func TestIssuesDoNotCountAsTending(t *testing.T) {
	p := Grow("repo", Shrub, []Event{
		{Kind: Push, At: t0},
		{Kind: IssueOpened, At: t0.Add(48 * time.Hour)},
	})
	if !p.LastTended.Equal(t0) || p.OpenIssues != 1 {
		t.Errorf("LastTended=%v OpenIssues=%d", p.LastTended, p.OpenIssues)
	}
}

func topRow(p *Plant) int {
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			if p.Grid[y][x].Kind != Empty {
				return y
			}
		}
	}
	return Height
}

func TestTinyReposAreSeedlings(t *testing.T) {
	for _, sp := range AllSpecies {
		for _, events := range [][]Event{nil, pushes(1)} {
			p := Grow("repo", sp, events)
			if n := len(p.cellsOf(Stem, Body, Leaf)); n < 3 {
				t.Errorf("%s with %d events: only %d cells, want a seedling", sp, len(events), n)
			}
			if h := ground - topRow(p); h > seedlingHeight+2 {
				t.Errorf("%s with %d events is %dpx tall, want a seedling", sp, len(events), h)
			}
		}
	}
}

func TestPlantsGrowWithTheLogOfPushes(t *testing.T) {
	for _, sp := range []Species{Shrub, Cactus} {
		small, big := Grow("repo", sp, pushes(8)), Grow("repo", sp, pushes(500))
		hs, hb := ground-topRow(small), ground-topRow(big)
		if hb <= hs {
			t.Errorf("%s: 500 pushes (%dpx) not taller than 8 (%dpx)", sp, hb, hs)
		}
		if hs > heightCap(8)+2 {
			t.Errorf("%s: 8 pushes reached %dpx, cap is %dpx", sp, hs, heightCap(8))
		}
	}
}

func TestCactusTrunkIsThreePixelsWide(t *testing.T) {
	p := Grow("repo", Cactus, pushes(40))
	for dx := -1; dx <= 1; dx++ {
		if k := p.Grid[ground][center+dx].Kind; k != Body {
			t.Errorf("trunk at dx=%d is kind %d, want Body", dx, k)
		}
	}
}

func TestCactusGrowsThroughItsFlowers(t *testing.T) {
	// An early merge puts a flower on the trunk's top; later pushes must
	// still raise the trunk (carrying the flower up), not stop under it.
	events := append(pushes(2), Event{Kind: Merge, At: t0.Add(3 * time.Hour)})
	for i := 0; i < 120; i++ {
		events = append(events, Event{Kind: Push, At: t0.Add(time.Duration(4+i) * time.Hour)})
	}
	p := Grow("repo", Cactus, events)
	trunk := 0
	for y := ground; y >= 0 && p.Grid[y][center].Kind == Body; y-- {
		trunk++
	}
	if min := heightCap(p.Pushes) / 2; trunk < min {
		t.Errorf("trunk is %dpx after 122 pushes, want at least %d", trunk, min)
	}
	if len(p.cellsOf(Flower)) != 1 {
		t.Errorf("flowers = %d, want the 1 flower kept", len(p.cellsOf(Flower)))
	}
}
