package scene

import (
	"testing"
	"time"

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

// shot draws one shooting star, age into its flight, on an empty 80×40 sky.
func shot(age time.Duration) *pixel.Canvas {
	c := pixel.New(80, 40)
	now := at(23, 0)
	drawShootingStars(c, now, []Shooting{{Start: now.Add(-age), Seed: 7}}, 0, 40)
	return c
}

// rightmost is the rightmost painted column, or -1.
func rightmost(c *pixel.Canvas) int {
	for x := c.W - 1; x >= 0; x-- {
		for y := 0; y < c.H; y++ {
			if c.At(x, y) != (pixel.RGB{}) {
				return x
			}
		}
	}
	return -1
}

func TestShootingStarFliesThenFades(t *testing.T) {
	early, late := shot(300*time.Millisecond), shot(1200*time.Millisecond)
	if painted(early) == 0 {
		t.Fatal("no shooting star in flight")
	}
	if rightmost(late) <= rightmost(early) {
		t.Error("a shooting star should move across the sky")
	}
	for _, age := range []time.Duration{-time.Second, ShootingFor, 2 * time.Second} {
		if n := painted(shot(age)); n != 0 {
			t.Errorf("%v into its flight it drew %d pixels", age, n)
		}
	}
}

func TestShootingStarsStayInTheSky(t *testing.T) {
	now := at(23, 0)
	for seed := int64(0); seed < 20; seed++ {
		for age := time.Duration(0); age < ShootingFor; age += 100 * time.Millisecond {
			c := pixel.New(240, 60)
			drawShootingStars(c, now, []Shooting{{Start: now.Add(-age), Seed: seed}}, 0, 42)
			for y := 42; y < c.H; y++ {
				for x := 0; x < c.W; x++ {
					if c.At(x, y) != (pixel.RGB{}) {
						t.Fatalf("seed %d, %v in: drew below the sky at %d,%d", seed, age, x, y)
					}
				}
			}
		}
	}
}

func TestDrawShowsShootingStars(t *testing.T) {
	now := at(23, 0)
	v := View{Cols: 3 * BedCols, Plots: manyPlots(3), Now: now, Seed: 1}
	plain := Draw(v).Encode(pixel.TrueColor)
	shown := 0
	for seed := int64(1); seed <= 5; seed++ { // plants in front may hide one
		v.Shooting = []Shooting{{Start: now.Add(-700 * time.Millisecond), Seed: seed}}
		if Draw(v).Encode(pixel.TrueColor) != plain {
			shown++
		}
	}
	if shown == 0 {
		t.Error("Draw showed none of the shooting stars in flight")
	}
	v.Shooting = []Shooting{{Start: now.Add(-2 * time.Second), Seed: 1}}
	if Draw(v).Encode(pixel.TrueColor) != plain {
		t.Error("a landed shooting star should leave no trace")
	}
}

func TestShootingStarsStandOut(t *testing.T) {
	for _, now := range []time.Time{at(12, 0), at(23, 30)} {
		top, bottom := SkyAt(now)
		for _, col := range []pixel.RGB{meteorHead, meteorTail} {
			if dist(col, top) < 60 || dist(col, bottom) < 60 {
				t.Errorf("shooting star colour %v blends into the %s sky", col, now.Format("15:04"))
			}
		}
	}
}
