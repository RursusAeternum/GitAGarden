package scene

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func cloudFrame(at time.Time, seed int64) *pixel.Canvas {
	c := pixel.New(60, 24)
	DrawClouds(c, at, 0, 24, seed)
	return c
}

func TestCloudsAreDeterministicAndDrift(t *testing.T) {
	noon := at(12, 0)
	a, b := cloudFrame(noon, 1), cloudFrame(noon, 1)
	if a.Encode(pixel.TrueColor) != b.Encode(pixel.TrueColor) {
		t.Error("the same moment drew different clouds")
	}
	if painted(a) == 0 {
		t.Fatal("no clouds drawn")
	}
	later := cloudFrame(noon.Add(20*time.Second), 1)
	if a.Encode(pixel.TrueColor) == later.Encode(pixel.TrueColor) {
		t.Error("clouds did not drift in 20 seconds")
	}
}

func TestCloudsStayInTheirBand(t *testing.T) {
	c := pixel.New(80, 40)
	for s := 0; s < 20; s++ {
		DrawClouds(c, at(12, 0).Add(time.Duration(s)*time.Minute), 10, 30, int64(s))
	}
	for x := 0; x < c.W; x++ {
		for _, y := range []int{9, 30} {
			if c.At(x, y) != (pixel.RGB{}) {
				t.Fatalf("cloud pixel outside its band at %d,%d", x, y)
			}
		}
	}
}

func TestCloudsFadeAtNight(t *testing.T) {
	brightest := func(c *pixel.Canvas) uint8 {
		var m uint8
		for y := 0; y < c.H; y++ {
			for x := 0; x < c.W; x++ {
				m = max(m, c.At(x, y).R)
			}
		}
		return m
	}
	// Day clouds blend at 0.85, night ones at 0.25; two separate clouds may
	// overlap, so allow for a double blend at night.
	day, night := brightest(cloudFrame(at(12, 0), 3)), brightest(cloudFrame(at(0, 0), 3))
	if night == 0 || int(night)*4 >= int(day)*3 {
		t.Errorf("night clouds (%d) should be much fainter than day clouds (%d)", night, day)
	}
}

func TestGlintSweepsNowAndThen(t *testing.T) {
	base := time.UnixMilli(20_000 * 1000) // a whole number of 20 s glint periods
	glint := func(at time.Time) int {
		c := pixel.New(30, 30)
		DrawGlint(c, at, 15, 2, 28, 21, 0)
		return painted(c)
	}
	if glint(base.Add(700*time.Millisecond)) == 0 {
		t.Error("no glint during the sweep")
	}
	if n := glint(base.Add(5 * time.Second)); n != 0 {
		t.Errorf("glint drawn outside the sweep: %d px", n)
	}
}
