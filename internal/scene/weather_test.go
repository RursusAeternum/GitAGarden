package scene

import (
	"math"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func frameOf(pl Plot, now time.Time) *pixel.Canvas {
	return Draw(View{Cols: BedCols, Plots: []Plot{pl}, Now: now, Seed: 1})
}

func colored(c *pixel.Canvas, col pixel.RGB) int {
	n := 0
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if c.At(x, y) == col {
				n++
			}
		}
	}
	return n
}

func TestStormRainsOnThePlant(t *testing.T) {
	pl := demoPlots()[0]
	if colored(frameOf(pl, at(12, 0)), stormCloud) != 0 {
		t.Fatal("clear weather should have no storm cloud")
	}
	pl.Weather = Storm
	now := at(12, 0)
	storm := frameOf(pl, now)
	if colored(storm, stormCloud) == 0 {
		t.Error("no storm cloud over a failing plant")
	}
	if storm.Encode(pixel.TrueColor) == frameOf(pl, now.Add(400*time.Millisecond)).Encode(pixel.TrueColor) {
		t.Error("rain did not fall")
	}
}

func TestCloudyIsASmallGreyCloud(t *testing.T) {
	pl := demoPlots()[0]
	pl.Weather = Cloudy
	grey := colored(frameOf(pl, at(12, 0)), greyCloud)
	pl.Weather = Storm
	dark := colored(frameOf(pl, at(12, 0)), stormCloud)
	if grey == 0 || grey >= dark {
		t.Errorf("pending CI cloud = %d px, storm cloud = %d px; want a small grey cloud", grey, dark)
	}
}

func TestGlassKeepsWeatherAndSnailsOut(t *testing.T) {
	var glass Plot
	for _, p := range demoPlots() {
		if p.Finished {
			glass = p
		}
	}
	glass.Weather, glass.Snail = Storm, true
	c := frameOf(glass, at(12, 0))
	if colored(c, stormCloud) != 0 || colored(c, shellColor) != 0 {
		t.Error("finished plants under glass should have no weather or snail")
	}
}

func TestSnailCrawls(t *testing.T) {
	pl := demoPlots()[0]
	if colored(frameOf(pl, at(12, 0)), shellColor) != 0 {
		t.Fatal("snail drawn without new issues")
	}
	pl.Snail = true
	a := frameOf(pl, at(12, 0))
	if colored(a, shellColor) == 0 {
		t.Fatal("no snail")
	}
	if a.Encode(pixel.TrueColor) == frameOf(pl, at(12, 0).Add(3*time.Second)).Encode(pixel.TrueColor) {
		t.Error("the snail did not move")
	}
}

func TestSeedHeadsDontAttractCritters(t *testing.T) {
	pl := demoPlots()[0] // gag-core: healthy and flowering
	if !Flowering(pl) {
		t.Fatal("test plot should be flowering")
	}
	pl.Style.Now = pl.Plant.LastTended.Add(400 * 24 * time.Hour)
	if Flowering(pl) {
		t.Error("flowers gone to seed should not count as flowering")
	}
}

func dist(a, b pixel.RGB) float64 {
	dr, dg, db := float64(a.R)-float64(b.R), float64(a.G)-float64(b.G), float64(a.B)-float64(b.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func TestStormStandsOutDayAndNight(t *testing.T) {
	pl := demoPlots()[0]
	pl.Weather = Storm
	for _, now := range []time.Time{at(12, 0), at(23, 30)} {
		top, bottom := SkyAt(now)
		ambientTop := pixel.Lerp(top, cloudColor, 0.85-0.6*Darkness(now)) // a drifting cloud at the top of the sky
		for _, c := range []struct {
			name string
			col  pixel.RGB
		}{{"rain", rainColor}, {"storm rim", stormRim}} {
			if dist(c.col, top) < 60 || dist(c.col, bottom) < 60 {
				t.Errorf("%s at %s blends into the sky", c.name, now.Format("15:04"))
			}
		}
		if dist(stormRim, ambientTop) < 45 {
			t.Errorf("storm rim at %s looks like an ordinary cloud", now.Format("15:04"))
		}
		if colored(frameOf(pl, now), stormRim) == 0 {
			t.Errorf("no bright rim on the storm cloud at %s", now.Format("15:04"))
		}
		flashes := 0
		for ms := 0; ms < 5000; ms += 100 { // lightning within any 5 s
			flashes += colored(frameOf(pl, now.Add(time.Duration(ms)*time.Millisecond)), lightning)
		}
		if flashes == 0 {
			t.Errorf("no lightning within 5 s at %s", now.Format("15:04"))
		}
	}
}

func TestWeatherSurvivesAShortWindow(t *testing.T) {
	// A default 80x24 terminal crops the top 2 px of the bed: the cloud must
	// still show.
	pl := demoPlots()[0]
	pl.Weather = Storm
	full := colored(frameOf(pl, at(12, 0)), stormCloud)
	cropped := colored(Draw(View{Cols: BedCols, Rows: BedRows - 1, Plots: []Plot{pl}, Now: at(12, 0), Seed: 1}), stormCloud)
	if cropped*3 < full*2 {
		t.Errorf("cropped storm cloud has %d of %d px", cropped, full)
	}
}
