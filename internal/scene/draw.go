package scene

import (
	"math"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// View describes one frame of the garden.
type View struct {
	Cols, Rows int // terminal cells to fill; Rows 0 means as tall as the beds need
	Plots      []Plot
	Now        time.Time // wall clock: sky, sun, clouds, glints
	Pan        float64   // camera offset in plot columns when the garden overflows
	Seed       int64
	Motion     bool       // a live frame: glints on glass
	Sky        SkyMode    // what the night sky shows
	StarTotal  int        // the garden's GitHub stars, for SkyStars
	Shooting   []Shooting // shooting stars, drawn while in flight
	Selected   int        // 1 + the selected plot's index; 0, the zero value, selects none
	Card       *Card      // the selected plot's detail card; nil for none
}

// Layout is how plots are arranged in a frame.
type Layout struct {
	PerRow   int  // plot columns visible side by side
	Beds     int  // bed rows on screen
	Columns  int  // plot columns in the whole garden; more than PerRow when it pans
	Overflow bool // the garden is wider than the window and pans
}

// LayoutFor fits n plots into cols×rows terminal cells. Beds stack while the
// height allows; beyond that plots go into columns that pan. rows <= 0 means
// unlimited height (static prints), so nothing pans.
func LayoutFor(cols, rows, n int) Layout {
	per := PerRow(cols)
	need := max(1, (n+per-1)/per)
	fit := need
	if rows > 0 {
		fit = max(1, rows/BedRows)
	}
	if need <= fit {
		return Layout{PerRow: per, Beds: need, Columns: per}
	}
	return Layout{PerRow: per, Beds: fit, Columns: (n + fit - 1) / fit, Overflow: true}
}

// Draw renders a frame. With Rows set the canvas is exactly Cols×Rows
// cells: beds sit at the bottom, spare height above them is sky, and a
// window shorter than the beds crops them from the top.
func Draw(v View) *pixel.Canvas {
	g := geometryOf(v)
	c := pixel.New(g.cols, g.h)
	var hosts []Host
	for b := 0; b < g.lay.Beds; b++ {
		oy := g.bedTop(b)
		skyTop := oy
		if b == 0 && oy > 0 {
			skyTop = 0
		}
		if v.Sky == SkyStars {
			skyGradient(c, v.Now, skyTop, oy+groundTop)
			if b == 0 { // one sky of stars, in the top band
				DrawStarSky(c, skyTop, oy+groundTop, v.StarTotal, Darkness(v.Now))
			}
		} else {
			DrawSky(c, v.Now, skyTop, oy+groundTop, v.Seed+int64(b))
		}
		if b == 0 {
			drawSunMoon(c, v.Now, skyTop, oy+groundTop) // one sun for the whole garden
		}
		DrawClouds(c, v.Now, skyTop, oy+groundTop, v.Seed+int64(b))
		if b == 0 {
			drawShootingStars(c, v.Now, v.Shooting, skyTop, oy+groundTop)
		}
		DrawGround(c, oy+groundTop, oy+bedPx, v.Seed)
		for _, s := range g.slots(v, b) {
			pl := v.Plots[s.index]
			selected := v.Selected == s.index+1
			if selected {
				lightGround(c, s.cx, oy+groundTop, oy+bedPx)
			}
			drawPlot(c, v, pl, s.cx, oy, selected)
			if v.Motion && Flowering(pl) {
				hosts = append(hosts, Host{X: s.cx, Y: oy + plantBaseY - garden.Height/2})
			}
		}
	}
	if v.Motion {
		DrawCritters(c, v.Now, hosts, v.Seed)
	}
	if v.Card != nil {
		drawCard(c, v)
	}
	return c
}

type slot struct{ index, cx int }

// slots lists the plots bed b shows and the columns they're centered on.
// Without overflow plots fill beds row by row, centered. With overflow they
// go into columns (column j holds plots j*Beds … j*Beds+Beds-1), and the
// camera shows PerRow of them plus one sliding in, wrapping around.
func slots(lay Layout, b, n, cols int, pan float64) []slot {
	var out []slot
	if !lay.Overflow {
		lo, hi := b*lay.PerRow, min(n, (b+1)*lay.PerRow)
		left := (cols - (hi-lo)*BedCols) / 2
		for i := lo; i < hi; i++ {
			out = append(out, slot{i, left + (i-lo)*BedCols + BedCols/2})
		}
		return out
	}
	pan = math.Mod(pan, float64(lay.Columns))
	if pan < 0 {
		pan += float64(lay.Columns)
	}
	first := int(pan)
	shift := int((pan - float64(first)) * BedCols)
	left := (cols - lay.PerRow*BedCols) / 2
	for k := 0; k <= lay.PerRow; k++ {
		j := (first + k) % lay.Columns
		if i := j*lay.Beds + b; i < n {
			out = append(out, slot{i, left + k*BedCols + BedCols/2 - shift})
		}
	}
	return out
}

// The camera holds each view for panHold, then slides one column over
// panSlide.
const panHold, panSlide = 30 * time.Second, 1200 * time.Millisecond

// PanAt is the camera offset in plot columns, elapsed time into a live
// session, for a garden of the given number of columns. It wraps around.
func PanAt(elapsed time.Duration, columns int) float64 {
	if columns <= 1 || elapsed < 0 {
		return 0
	}
	k := int64(elapsed / panHold)
	into := elapsed - time.Duration(k)*panHold
	pos := float64(k)
	if k > 0 && into < panSlide {
		f := float64(into) / float64(panSlide)
		pos = float64(k-1) + f*f*(3-2*f) // ease in and out
	}
	return math.Mod(pos, float64(columns))
}

// Sliding reports whether the camera is mid-slide at elapsed.
func Sliding(elapsed time.Duration) bool {
	return elapsed >= panHold && elapsed%panHold < panSlide
}
