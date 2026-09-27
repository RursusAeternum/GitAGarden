package scene

import (
	"math"

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
