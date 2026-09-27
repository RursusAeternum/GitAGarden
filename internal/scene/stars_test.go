package scene

import (
	"testing"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func starSky(total int, darkness float64) *pixel.Canvas {
	c := pixel.New(80, 30)
	DrawStarSky(c, 0, 30, total, darkness)
	return c
}

func TestStarSkyHasOneDotPerStar(t *testing.T) {
	if n := painted(starSky(17, 1)); n != 17 {
		t.Errorf("17 stars drew %d dots", n)
	}
}

func TestStarSkyKeepsItsStars(t *testing.T) {
	before, after := starSky(17, 1), starSky(18, 1)
	for y := 0; y < before.H; y++ {
		for x := 0; x < before.W; x++ {
			if px := before.At(x, y); px != (pixel.RGB{}) && after.At(x, y) != px {
				t.Fatalf("a new star moved or changed the star at %d,%d", x, y)
			}
		}
	}
	if n := painted(after); n != 18 {
		t.Errorf("18 stars drew %d dots", n)
	}
}

func TestStarSkyCapsAndBrightens(t *testing.T) {
	capacity := 80 * 30 / 40
	brightness := func(c *pixel.Canvas) int {
		sum := 0
		for y := 0; y < c.H; y++ {
			for x := 0; x < c.W; x++ {
				sum += int(c.At(x, y).R)
			}
		}
		return sum
	}
	full, huge := starSky(capacity, 1), starSky(50000, 1)
	if n := painted(huge); n != capacity {
		t.Errorf("50000 stars drew %d dots, want the sky's capacity %d", n, capacity)
	}
	if brightness(huge) <= brightness(full) {
		t.Error("more stars than the sky holds should make it brighter")
	}
}

func TestStarSkyOnlyAtNightAndOnlyStars(t *testing.T) {
	if n := painted(starSky(17, 0)); n != 0 {
		t.Errorf("%d stars in daylight", n)
	}
	if n := painted(starSky(0, 1)); n != 0 {
		t.Errorf("a garden with no stars drew %d", n)
	}
}

func TestStarModeShowsTheGardensStars(t *testing.T) {
	bright := func(v View) int {
		c := Draw(v)
		n := 0
		for y := 0; y < c.H; y++ {
			for x := 0; x < c.W; x++ {
				if c.At(x, y).G > 100 {
					n++
				}
			}
		}
		return n
	}
	v := View{Cols: 3 * BedCols, Plots: manyPlots(3), Now: at(23, 0), Seed: 1}
	random := bright(v)
	v.Sky = SkyStars
	none := bright(v)
	v.StarTotal = 40
	forty := bright(v)
	if none >= random || none >= forty {
		t.Errorf("bright pixels: random sky %d, no stars %d, 40 stars %d", random, none, forty)
	}
}
