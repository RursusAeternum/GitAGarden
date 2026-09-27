package scene

import (
	"math"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

var (
	glass     = rgb(190, 230, 240)
	highlight = rgb(255, 255, 255)
)

// DrawCloche draws a glass bell jar w pixels wide over rows [top, bottom],
// centered on cx: a faint fill, brighter edges, and a highlight streak.
func DrawCloche(c *pixel.Canvas, cx, top, bottom, w int) {
	half := w / 2
	domeCY := top + half
	for y := top; y <= bottom; y++ {
		span := half
		if y < domeCY {
			dy := domeCY - y
			span = int(math.Sqrt(float64(half*half - dy*dy)))
		}
		for dx := -span; dx <= span; dx++ {
			alpha := 0.08
			if dx == -span || dx == span || y == top {
				alpha = 0.55
			}
			c.Blend(cx+dx, y, glass, alpha)
		}
	}
	for y := domeCY; y < bottom-2; y++ {
		c.Blend(cx-half+2, y, highlight, 0.35)
	}
}

// A streak of light sweeps across each cloche for glintSweep out of every
// glintEvery.
const glintEvery, glintSweep = 20 * time.Second, 1500 * time.Millisecond

// DrawGlint draws the moving streak of light across a cloche with the same
// geometry as DrawCloche. seed staggers cloches so they don't flash in
// unison.
func DrawGlint(c *pixel.Canvas, t time.Time, cx, top, bottom, w int, seed int64) {
	period := glintEvery.Milliseconds()
	offset := (seed%period + period) % period
	ms := (t.UnixMilli() + offset) % period
	if ms >= glintSweep.Milliseconds() {
		return
	}
	half := w / 2
	x := cx - half + int(float64(w)*float64(ms)/float64(glintSweep.Milliseconds()))
	for y := top + half/2; y < bottom-2; y++ {
		c.Blend(x, y, highlight, 0.45)
		c.Blend(x+1, y, highlight, 0.2)
	}
}
