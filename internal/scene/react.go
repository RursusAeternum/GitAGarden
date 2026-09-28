package scene

import (
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// ReactKind is what a reaction shows: the change that caused it.
type ReactKind int

const (
	ReactPush    ReactKind = iota // a watering can, or an agent's drone, waters the plant
	ReactMerge                    // a bud bursts and a butterfly flies off
	ReactRelease                  // sparkles, then bees
	ReactWeedIn                   // a new issue's weed comes up
	ReactWeedOut                  // a closed issue's weed is pulled
	ReactStorm                    // CI failed: the storm rolls in
	ReactClear                    // CI recovered: the storm clears, a rainbow
)

// Reaction is an animation on one plot, from Start.
type Reaction struct {
	Plot    int // 1 + the plot's index, like View.Selected
	Kind    ReactKind
	Start   time.Time
	Before  *garden.Plant // the plant before the change; nil when unknown
	Reveals bool          // the plot shows Before until this reaction's change moment
	Drone   bool          // a push by an AI agent: a drone waters instead of a can
	Seed    int64
}

// growFor is how long new cells take to come in.
const growFor = time.Second

// Duration is how long the reaction plays.
func (r Reaction) Duration() time.Duration {
	switch r.Kind {
	case ReactPush:
		if r.Drone {
			return 3 * time.Second
		}
		return 2500 * time.Millisecond
	case ReactMerge:
		return 3 * time.Second
	case ReactRelease:
		return 4 * time.Second
	case ReactWeedIn, ReactWeedOut:
		return 1500 * time.Millisecond
	case ReactStorm:
		return 2 * time.Second
	case ReactClear:
		return 5 * time.Second
	}
	return 0
}

// ChangeAt is when, into the reaction, the plot takes its new shape: when
// the drops land, the bud bursts or the storm cloud arrives.
func (r Reaction) ChangeAt() time.Duration {
	switch r.Kind {
	case ReactPush:
		if r.Drone {
			return time.Second
		}
		return 800 * time.Millisecond
	case ReactMerge:
		return 600 * time.Millisecond
	case ReactStorm:
		return 1500 * time.Millisecond
	}
	return 0
}

// shaped is plot i as its reactions show it at v.Now. Until a revealing
// reaction's change moment, even while that reaction still waits, the plant
// keeps its old shape; then its new cells grow in over growFor. A storm's
// steady weather waits for its cloud to roll in, and a clearing storm stays
// until its reaction starts.
func (v View) shaped(i int, pl Plot) Plot {
	for _, r := range v.Reactions {
		if r.Plot != i+1 {
			continue
		}
		age := v.Now.Sub(r.Start)
		if age >= r.Duration() {
			continue
		}
		if r.Reveals && r.Before != nil && !pl.Finished {
			switch at := r.ChangeAt(); {
			case age < at:
				pl.Plant = r.Before
			case age < at+growFor:
				pl.Style.Before, pl.Style.Grown = r.Before, float64(age-at)/float64(growFor)
			}
		}
		switch {
		case r.Kind == ReactStorm && age < r.ChangeAt():
			pl.Weather = Clear
		case r.Kind == ReactClear && age < 0:
			pl.Weather = Storm
		}
	}
	return pl
}

// placedPlot is where Draw put a plot: its index, the column its slot is
// centered on, and its bed's top pixel row.
type placedPlot struct{ index, cx, oy int }

// drawReactions draws the reactions playing on one placed plot.
func drawReactions(c *pixel.Canvas, v View, p placedPlot) {
	pl := v.Plots[p.index]
	for _, r := range v.Reactions {
		if r.Plot != p.index+1 || pl.Finished {
			continue
		}
		age := v.Now.Sub(r.Start)
		if age < 0 || age >= r.Duration() {
			continue
		}
		switch r.Kind {
		case ReactPush:
			drawWatering(c, r, age, p.cx, p.oy, plantTop(pl.Plant, p.oy))
		case ReactWeedIn:
			drawWeedIn(c, r, age, p.cx, p.oy, pl.Plant)
		case ReactWeedOut:
			drawWeedOut(c, r, age, p.cx, p.oy)
		}
	}
}

// plantTop is the pixel row of a plant's highest cell, in a bed whose top is
// pixel row oy.
func plantTop(p *garden.Plant, oy int) int { return oy + headroom + p.Top() }

// plantBase is the pixel row a bed's plants stand on.
func plantBase(oy int) int { return oy + plantBaseY }

// slotBlend blends col onto (x, y) when x is inside the slot centered on cx,
// so a plot's effects stay in its own slot.
func slotBlend(c *pixel.Canvas, cx, x, y int, col pixel.RGB, a float64) {
	if x >= slotLeft(cx) && x < slotLeft(cx)+BedCols {
		c.Blend(x, y, col, a)
	}
}
