package scene

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func TestCrittersNeedDaylightAndHosts(t *testing.T) {
	hosts := []Host{{20, 10}}
	count := func(at time.Time, hs []Host) int {
		c := pixel.New(40, 20)
		DrawCritters(c, at, hs, 1)
		return painted(c)
	}
	if count(at(12, 0), nil) != 0 {
		t.Error("critters without any flowering plant")
	}
	if count(at(23, 0), hosts) != 0 {
		t.Error("critters at night")
	}
	if count(at(12, 0), hosts) == 0 {
		t.Error("no critters around a flowering plant at noon")
	}
}

func TestCrittersFly(t *testing.T) {
	hosts := []Host{{20, 10}, {30, 10}}
	frame := func(at time.Time) string {
		c := pixel.New(50, 20)
		DrawCritters(c, at, hosts, 1)
		return c.Encode(pixel.TrueColor)
	}
	if frame(at(12, 0)) == frame(at(12, 0).Add(1500*time.Millisecond)) {
		t.Error("critters did not move in 1.5 s")
	}
}

func TestLiveFramesHaveCritters(t *testing.T) {
	plots := demoPlots()[:1] // gag-core: healthy and flowering
	if !Flowering(plots[0]) {
		t.Fatal("test plot should be flowering")
	}
	v := View{Cols: BedCols, Plots: plots, Now: at(12, 0), Seed: 1}
	still := Draw(v).Encode(pixel.TrueColor)
	v.Motion = true
	if Draw(v).Encode(pixel.TrueColor) == still {
		t.Error("no critters in a live frame at noon")
	}
}
