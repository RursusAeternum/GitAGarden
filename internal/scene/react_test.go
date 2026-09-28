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

func TestMoreReactionsPlayThenLeave(t *testing.T) {
	plain := frameText(View{Cols: 3 * BedCols, Plots: demoPlots(), Now: at(14, 0), Seed: 1})
	for _, k := range []ReactKind{ReactMerge, ReactRelease, ReactStorm, ReactClear} {
		r := Reaction{Kind: k, Before: earlier(), Reveals: true, Seed: 5}
		if frameText(reacting(r, r.Duration()/2)) == plain {
			t.Errorf("kind %v: nothing drawn halfway through", k)
		}
		if frameText(reacting(r, r.Duration())) != plain {
			t.Errorf("kind %v: something left behind after it ended", k)
		}
	}
}

func TestReactionColoursStandOut(t *testing.T) {
	for _, now := range []time.Time{at(12, 0), at(23, 30)} {
		top, bottom := SkyAt(now)
		for name, col := range map[string]pixel.RGB{
			"gold sparkle": sparkGold, "white sparkle": sparkWhite, "drone": droneBody,
			"rainbow red": rainbow[0], "rainbow yellow": rainbow[2], "rainbow violet": rainbow[5],
		} {
			if dist(col, top) < 60 || dist(col, bottom) < 60 {
				t.Errorf("%s %v blends into the %s sky", name, col, now.Format("15:04"))
			}
		}
	}
}

func TestDropsStandOutAllDay(t *testing.T) {
	for m := 0; m < 24*60; m += 10 {
		now := at(0, 0).Add(time.Duration(m) * time.Minute)
		top, bottom := SkyAt(now)
		if drop := dropFor(now); dist(drop, top) < 60 || dist(drop, bottom) < 60 {
			t.Errorf("drops %v blend into the %s sky", drop, now.Format("15:04"))
		}
	}
}

// strayPixels counts the pixels r changes outside the middle plot's slot, at
// age into it. The frame is 3 plots wide with 5-column margins, so the middle
// slot is columns 31-56.
func strayPixels(plots []Plot, r Reaction, age time.Duration) int {
	now := at(14, 0)
	v := View{Cols: 3*BedCols + 10, Plots: plots, Now: now, Seed: 1}
	plain := Draw(v)
	r.Plot, r.Start = 2, now.Add(-age)
	v.Reactions = []Reaction{r}
	got := Draw(v)
	n := 0
	for y := 0; y < got.H; y++ {
		for x := 0; x < got.W; x++ {
			if (x < 31 || x >= 31+BedCols) && plain.At(x, y) != got.At(x, y) {
				n++
			}
		}
	}
	return n
}

func TestReactionsStayInTheirSlot(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seedling := demoPlots()
	seedling[1] = Plot{Plant: garden.Grow("tiny", garden.Shrub, []garden.Event{{Kind: garden.Push, At: t0}}),
		Style: garden.Style{Health: 1}, Name: "tiny", Status: "new"}
	gardens := []struct {
		plots  []Plot
		before *garden.Plant
	}{
		{demoPlots(), garden.Grow("rustyfs", garden.Cactus, garden.FakeHistory("rustyfs", 30, t0))},
		{seedling, garden.Grow("tiny", garden.Shrub, nil)},
	}
	ms := func(ms ...int) []time.Duration {
		var out []time.Duration
		for _, m := range ms {
			out = append(out, time.Duration(m)*time.Millisecond)
		}
		return out
	}
	cases := []struct {
		kind ReactKind
		ages []time.Duration // before any critter takes off
	}{
		{ReactPush, ms(200, 600, 1100, 1500, 2000)},
		{ReactMerge, ms(300, 800)},
		{ReactRelease, ms(300, 900)},
		{ReactWeedIn, ms(200, 1000)},
		{ReactWeedOut, ms(200, 1000)},
		{ReactStorm, ms(300, 1000, 1600)},
		{ReactClear, ms(300, 1200, 3000, 4800)},
	}
	for gi, g := range gardens {
		for _, c := range cases {
			for _, age := range c.ages {
				r := Reaction{Kind: c.kind, Before: g.before, Reveals: true, Seed: 9}
				if n := strayPixels(g.plots, r, age); n > 0 {
					t.Errorf("garden %d, kind %v at %v: %d pixels outside its slot", gi, c.kind, age, n)
				}
			}
		}
	}
}

func TestCardCoversReactions(t *testing.T) {
	now := at(14, 0)
	v := View{Cols: 3 * BedCols, Plots: demoPlots(), Now: now, Seed: 1, Selected: 1, Card: sampleCard()}
	x, y, w, h, ok := CardRect(v)
	if !ok {
		t.Fatal("no card")
	}
	plain := Draw(v)
	v.Reactions = []Reaction{{Plot: 2, Kind: ReactRelease, Start: now.Add(-time.Second), Seed: 1}}
	got := Draw(v)
	for py := 2 * y; py < 2*(y+h); py++ {
		for px := x; px < x+w; px++ {
			if plain.At(px, py) != got.At(px, py) {
				t.Fatalf("a reaction shows through the card at %d,%d", px, py)
			}
		}
	}
}

func TestShapesStepThroughQueuedReveals(t *testing.T) {
	pl, first := demoPlots()[0], earlier()
	middle := garden.Grow("gag-core", garden.Shrub, garden.FakeHistory("gag-core", 40, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	start := at(14, 0)
	reacts := []Reaction{ // two loads' pushes, the second queued 3 s after the first
		{Plot: 1, Kind: ReactPush, Start: start, Before: first, Reveals: true},
		{Plot: 1, Kind: ReactPush, Start: start.Add(3 * time.Second), Before: middle, Reveals: true},
	}
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	for _, c := range []struct {
		into          time.Duration
		plant, before *garden.Plant
	}{
		{ms(500), first, nil},        // before the first change
		{ms(1300), middle, first},    // the first change grows in
		{ms(2200), middle, nil},      // between the two
		{ms(3500), middle, nil},      // before the second change
		{ms(4300), pl.Plant, middle}, // the second change grows in
		{ms(6000), pl.Plant, nil},    // both done
	} {
		v := View{Plots: []Plot{pl}, Now: start.Add(c.into), Reactions: reacts}
		got := v.shaped(0, pl)
		if got.Plant != c.plant || got.Style.Before != c.before {
			t.Errorf("%v in: plant %p (before %p), want %p (before %p)", c.into, got.Plant, got.Style.Before, c.plant, c.before)
		}
	}
}

// weedy is a small plant with opened issues, closed of them closed.
func weedy(opened, closed int) *garden.Plant {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ev := []garden.Event{{Kind: garden.Push, At: t0}}
	for i := 0; i < opened; i++ {
		ev = append(ev, garden.Event{Kind: garden.IssueOpened, At: t0})
	}
	for i := 0; i < closed; i++ {
		ev = append(ev, garden.Event{Kind: garden.IssueClosed, At: t0})
	}
	return garden.Grow("weeds", garden.Shrub, ev)
}

func TestWeedsFollowTheirReactions(t *testing.T) {
	start := at(14, 0)
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	plot := func(p *garden.Plant) Plot { return Plot{Plant: p, Style: garden.Style{Health: 1}, Name: "weeds"} }
	three, one := weedy(3, 0), weedy(1, 0)
	type at struct {
		into time.Duration
		want int
	}
	for _, c := range []struct {
		name   string
		now    *garden.Plant
		reacts []Reaction
		checks []at
	}{
		{"a push, then a closed issue", weedy(3, 1), []Reaction{
			{Plot: 1, Kind: ReactPush, Start: start, Before: three, Reveals: true},
			{Plot: 1, Kind: ReactWeedOut, Start: start.Add(3 * time.Second), Before: three},
		}, []at{{ms(500), 3}, {ms(1500), 3}, {ms(3200), 2}}},
		{"one closed, one opened", weedy(4, 1), []Reaction{
			{Plot: 1, Kind: ReactWeedOut, Start: start, Before: three, Reveals: true},
			{Plot: 1, Kind: ReactWeedIn, Start: start.Add(3 * time.Second), Before: three},
		}, []at{{ms(-1000), 3}, {ms(500), 2}, {ms(3500), 2}, {ms(4500), 3}}},
		{"a closed issue on a plant without weeds", weedy(0, 0), []Reaction{
			{Plot: 1, Kind: ReactWeedOut, Start: start.Add(time.Second), Before: weedy(0, 0), Reveals: true},
		}, []at{{ms(500), 0}, {ms(1500), 0}}},
		{"a burst of new issues", weedy(4, 0), []Reaction{
			{Plot: 1, Kind: ReactWeedIn, Start: start, Before: one, Reveals: true},
		}, []at{{ms(-500), 1}, {ms(500), 1}, {ms(1200), 4}}},
	} {
		pl := plot(c.now)
		for _, chk := range c.checks {
			v := View{Plots: []Plot{pl}, Now: start.Add(chk.into), Reactions: c.reacts}
			if got := v.shaped(0, pl).Plant.OpenIssues; got != chk.want {
				t.Errorf("%s, %v in: %d weeds, want %d", c.name, chk.into, got, chk.want)
			}
		}
	}
}

func TestABurstOfWeedsComesUpTogether(t *testing.T) {
	start, now := at(14, 0), weedy(4, 0)
	v := View{Cols: BedCols, Plots: []Plot{{Plant: now, Style: garden.Style{Health: 1}, Name: "weeds"}}, Now: start}
	cx := BedCols / 2
	weedAt := func(c *pixel.Canvas, i int) bool {
		sp, ok := now.WeedSpot(i)
		x, y := sp.At(cx, plantBaseY)
		return ok && c.At(x, y) == weedGreen
	}
	plain := Draw(v)
	for i := 0; i < 4; i++ {
		if !weedAt(plain, i) {
			t.Fatalf("weed %d isn't where the test looks for it", i)
		}
	}
	v.Now = start.Add(900 * time.Millisecond)
	v.Reactions = []Reaction{{Plot: 1, Kind: ReactWeedIn, Start: start, Before: weedy(1, 0), Reveals: true}}
	growing := Draw(v)
	for i := 1; i < 4; i++ {
		if !weedAt(growing, i) {
			t.Errorf("new weed %d hasn't come up 0.9 s in", i)
		}
	}
}
