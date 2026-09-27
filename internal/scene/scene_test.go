package scene

import (
	"os"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func TestMain(m *testing.M) {
	time.Local = time.UTC // the sky follows the local clock; pin it for tests
	os.Exit(m.Run())
}

func at(h, min int) time.Time { return time.Date(2026, 6, 1, h, min, 0, 0, time.UTC) }

func painted(c *pixel.Canvas) int {
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

func TestDarkness(t *testing.T) {
	if d := Darkness(at(23, 0)); d != 1 {
		t.Errorf("23:00 darkness = %v, want 1", d)
	}
	if d := Darkness(at(3, 0)); d != 1 {
		t.Errorf("03:00 darkness = %v, want 1", d)
	}
	if d := Darkness(at(12, 0)); d != 0 {
		t.Errorf("12:00 darkness = %v, want 0", d)
	}
	if d := Darkness(at(19, 45)); d <= 0 || d >= 1 {
		t.Errorf("19:45 darkness = %v, want dusk between 0 and 1", d)
	}
}

func TestSkyAtNoonIsDaylight(t *testing.T) {
	top, bot := SkyAt(at(12, 0))
	if top != rgb(80, 150, 230) || bot != rgb(175, 215, 245) {
		t.Errorf("noon sky = %v / %v", top, bot)
	}
}

func TestSkyUsesLocalTime(t *testing.T) {
	noon := at(12, 0)
	tokyo := noon.In(time.FixedZone("JST", 9*3600)) // same instant, different zone
	a, _ := SkyAt(noon)
	b, _ := SkyAt(tokyo)
	if a != b {
		t.Errorf("sky depends on the timestamp's zone: %v vs %v", a, b)
	}
}

func TestStarsOnlyAtNight(t *testing.T) {
	day, night := pixel.New(40, 20), pixel.New(40, 20)
	DrawStars(day, 0, 20, 1, 0)
	DrawStars(night, 0, 20, 1, 1)
	if n := painted(day); n != 0 {
		t.Errorf("%d stars in daylight", n)
	}
	if painted(night) == 0 {
		t.Error("no stars at night")
	}
}

func TestDrawSkyFillsOnlyItsBand(t *testing.T) {
	c := pixel.New(30, 20)
	DrawSky(c, at(12, 0), 4, 12, 1)
	for x := 0; x < c.W; x++ { // includes the sun's columns
		if c.At(x, 3) != (pixel.RGB{}) || c.At(x, 12) != (pixel.RGB{}) {
			t.Fatalf("sky or sun drawn outside its band at column %d", x)
		}
	}
	if c.At(0, 4) == (pixel.RGB{}) {
		t.Error("sky band left empty")
	}
}

func TestGroundAndPot(t *testing.T) {
	c := pixel.New(30, 20)
	DrawGround(c, 12, 20, 1)
	if p := c.At(5, 15); !(p.R > p.G && p.G > p.B) {
		t.Errorf("soil is not brown: %v", p)
	}
	DrawPot(c, 15, 4)
	if got := c.At(15, 5); got != potRim {
		t.Errorf("pot rim = %v, want %v", got, potRim)
	}
}

func TestClocheIsTranslucent(t *testing.T) {
	bg := rgb(100, 0, 0)
	c := pixel.New(30, 30)
	c.Fill(bg)
	DrawCloche(c, 15, 2, 28, 21)
	mid, edge := c.At(15, 20), c.At(5, 20)
	if mid == bg {
		t.Error("glass fill missing")
	}
	if mid.G >= edge.G {
		t.Errorf("edge (%v) should be more visible than the fill (%v)", edge, mid)
	}
	if c.At(0, 0) != bg {
		t.Error("cloche drawn outside its bounds")
	}
}
