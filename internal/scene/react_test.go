package scene

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// earlier is gag-core's plant from early on in the demo plots' history.
func earlier() *garden.Plant {
	return garden.Grow("gag-core", garden.Shrub, garden.FakeHistory("gag-core", 20, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
}

// reacting is the demo plots at 14:00 with r on the first, age into it.
func reacting(r Reaction, age time.Duration) View {
	now := at(14, 0)
	r.Plot, r.Start = 1, now.Add(-age)
	return View{Cols: 3 * BedCols, Plots: demoPlots(), Now: now, Seed: 1, Reactions: []Reaction{r}}
}

func frameText(v View) string { return Draw(v).Encode(pixel.TrueColor) }

func TestReactionsPlayThenLeave(t *testing.T) {
	plain := frameText(View{Cols: 3 * BedCols, Plots: demoPlots(), Now: at(14, 0), Seed: 1})
	for _, r := range []Reaction{{Kind: ReactPush}, {Kind: ReactPush, Drone: true}, {Kind: ReactWeedIn}, {Kind: ReactWeedOut}} {
		r.Before, r.Reveals, r.Seed = earlier(), true, 3
		if frameText(reacting(r, r.Duration()/2)) == plain {
			t.Errorf("kind %v (drone %v): nothing drawn halfway through", r.Kind, r.Drone)
		}
		if frameText(reacting(r, r.Duration())) != plain {
			t.Errorf("kind %v (drone %v): something left behind after it ended", r.Kind, r.Drone)
		}
	}
}

func TestOldShapeUntilTheChange(t *testing.T) {
	pl, before := demoPlots()[0], earlier()
	v := View{Plots: []Plot{pl}, Now: at(14, 0)}
	shape := func(age time.Duration) Plot {
		v.Reactions = []Reaction{{Plot: 1, Kind: ReactPush, Start: v.Now.Add(-age), Before: before, Reveals: true}}
		return v.shaped(0, pl)
	}
	for _, age := range []time.Duration{-time.Second, 0, 700 * time.Millisecond} {
		if got := shape(age); got.Plant != before || got.Style.Before != nil {
			t.Errorf("%v in: the plant should keep its old shape", age)
		}
	}
	if got := shape(1300 * time.Millisecond); got.Plant != pl.Plant || got.Style.Before != before || got.Style.Grown <= 0 || got.Style.Grown >= 1 {
		t.Errorf("1.3 s in: the new cells should be growing in, got Grown %v", got.Style.Grown)
	}
	if got := shape(2 * time.Second); got.Plant != pl.Plant || got.Style.Before != nil {
		t.Error("2 s in: the plant should have its new shape")
	}
}

func TestWeatherWaitsForTheStorm(t *testing.T) {
	pl := demoPlots()[0]
	pl.Weather = Storm
	v := View{Plots: []Plot{pl}, Now: at(14, 0)}
	v.Reactions = []Reaction{{Plot: 1, Kind: ReactStorm, Start: v.Now.Add(-time.Second)}}
	if v.shaped(0, pl).Weather != Clear {
		t.Error("the steady storm should wait for its cloud to roll in")
	}
	v.Reactions[0].Start = v.Now.Add(-1600 * time.Millisecond)
	if v.shaped(0, pl).Weather != Storm {
		t.Error("once the cloud has arrived the storm should stay")
	}
	pl.Weather = Clear
	v.Reactions = []Reaction{{Plot: 1, Kind: ReactClear, Start: v.Now.Add(time.Second)}}
	if v.shaped(0, pl).Weather != Storm {
		t.Error("the old storm should stay until its clearing starts")
	}
	v.Reactions[0].Start = v.Now.Add(-time.Second)
	if v.shaped(0, pl).Weather != Clear {
		t.Error("once clearing, the storm should be gone")
	}
}

func TestDroneReplacesTheCan(t *testing.T) {
	can := Draw(reacting(Reaction{Kind: ReactPush, Before: earlier(), Reveals: true}, 800*time.Millisecond))
	drone := Draw(reacting(Reaction{Kind: ReactPush, Drone: true, Before: earlier(), Reveals: true}, 800*time.Millisecond))
	if colored(can, canColor) == 0 || colored(can, droneBody) != 0 {
		t.Errorf("a person's push: can %d px, drone %d px", colored(can, canColor), colored(can, droneBody))
	}
	if colored(drone, droneBody) == 0 || colored(drone, canColor) != 0 {
		t.Errorf("an agent's push: drone %d px, can %d px", colored(drone, droneBody), colored(drone, canColor))
	}
}
