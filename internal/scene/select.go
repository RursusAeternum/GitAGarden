package scene

import (
	"math"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// geometry is where a View's beds and plots land. Draw, PlotAt, OnScreen
// and the card's placement all use it, so what you click is what was drawn.
type geometry struct {
	lay  Layout
	cols int
	h    int // canvas height in pixels
	top  int // pixel row of bed 0's top; negative when a short window crops it
}

func geometryOf(v View) geometry {
	cols := max(v.Cols, 1)
	lay := LayoutFor(cols, v.Rows, len(v.Plots))
	h := lay.Beds * bedPx
	if v.Rows > 0 {
		h = v.Rows * 2
	}
	return geometry{lay: lay, cols: cols, h: h, top: h - lay.Beds*bedPx}
}

// bedTop is the pixel row where bed b starts.
func (g geometry) bedTop(b int) int { return g.top + b*bedPx }

// slots lists the plots bed b shows and the columns they're centered on.
func (g geometry) slots(v View, b int) []slot { return slots(g.lay, b, len(v.Plots), g.cols, v.Pan) }

// slotLeft is the first terminal column of the slot centered on cx.
func slotLeft(cx int) int { return cx - BedCols/2 }

// PlotAt is the plot drawn at terminal cell (col, row) of v's frame: the
// whole column of its slot, from the sky down to its labels. The top bed's
// sky reaches the top of the frame. ok is false on open ground beside the
// slots and outside the frame.
func PlotAt(v View, col, row int) (index int, ok bool) {
	g := geometryOf(v)
	y := row * 2
	if col < 0 || col >= g.cols || y < 0 || y >= g.h {
		return 0, false
	}
	for b := 0; b < g.lay.Beds; b++ {
		oy := g.bedTop(b)
		if y >= oy+bedPx || (y < oy && b > 0) {
			continue
		}
		for _, s := range g.slots(v, b) {
			if left := slotLeft(s.cx); col >= left && col < left+BedCols {
				return s.index, true
			}
		}
		return 0, false
	}
	return 0, false
}

// OnScreen lists the plots whose whole slot is inside v's frame, in reading
// order: bed by bed, left to right.
func OnScreen(v View) []int {
	g := geometryOf(v)
	var out []int
	for b := 0; b < g.lay.Beds; b++ {
		for _, s := range g.slots(v, b) {
			if left := slotLeft(s.cx); left >= 0 && left+BedCols <= g.cols {
				out = append(out, s.index)
			}
		}
	}
	return out
}

// ReadingOrder lists all n plots of a layout in reading order: along each
// row of beds, then the next. A panning garden's rows run across all its
// columns, not only the visible ones.
func ReadingOrder(lay Layout, n int) []int {
	if !lay.Overflow {
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	var out []int
	for b := 0; b < lay.Beds; b++ {
		for j := 0; j < lay.Columns; j++ {
			if i := j*lay.Beds + b; i < n {
				out = append(out, i)
			}
		}
	}
	return out
}

// SlideFor is how long the camera takes to slide to a new position.
const SlideFor = panSlide

// PanShowing is the camera position, in whole plot columns, nearest to pan
// at which column col is on screen, when perRow columns fit side by side in
// a garden of columns. A garden that fits returns 0.
func PanShowing(pan float64, col, perRow, columns int) float64 {
	if columns <= perRow {
		return 0
	}
	p := wrap(int(math.Round(pan)), columns)
	if wrap(col-p, columns) < perRow {
		return float64(p)
	}
	right := wrap(col-perRow+1, columns) // col becomes the rightmost column on screen
	left := col                          // col becomes the leftmost
	if ringDist(pan, float64(right), columns) <= ringDist(pan, float64(left), columns) {
		return float64(right)
	}
	return float64(left)
}

// SlidePan is the camera f of the way (0 to 1, eased) through a slide from
// from to to, the short way round a garden of columns.
func SlidePan(from, to, f float64, columns int) float64 {
	if columns <= 0 {
		return 0
	}
	n := float64(columns)
	d := math.Mod(to-from, n)
	switch {
	case d > n/2:
		d -= n
	case d < -n/2:
		d += n
	}
	f = math.Max(0, math.Min(1, f))
	pos := math.Mod(from+d*f*f*(3-2*f), n)
	if pos < 0 {
		pos += n
	}
	return pos
}

// ResumeAt is the time into automatic panning at which PanAt has just come
// to rest on column col. A camera handing back to automatic panning offsets
// its clock so that the moment it resumes is ResumeAt of its column.
func ResumeAt(col int) time.Duration { return time.Duration(col)*panHold + panSlide }

// wrap is a mod n, from 0 to n-1 even for negative a.
func wrap(a, n int) int { return (a%n + n) % n }

// ringDist is the distance between two positions on a ring of n columns.
func ringDist(a, b float64, n int) float64 {
	d := math.Abs(math.Mod(a-b, float64(n)))
	return math.Min(d, float64(n)-d)
}

// groundLit is the warm light on the selected plant's ground strip.
var groundLit = rgb(236, 196, 128)

// lightGround brightens pixel rows [y0, y1) of the slot centered on cx.
func lightGround(c *pixel.Canvas, cx, y0, y1 int) {
	for y := max(y0, 0); y < min(y1, c.H); y++ {
		for x := max(slotLeft(cx), 0); x < min(slotLeft(cx)+BedCols, c.W); x++ {
			c.Set(x, y, pixel.Lerp(c.At(x, y), groundLit, 0.35))
		}
	}
}
