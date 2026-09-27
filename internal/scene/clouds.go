package scene

import (
	"math/rand"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

var cloudColor = rgb(250, 250, 255)

// cloudDrift is how long a cloud takes to move one pixel to the right.
const cloudDrift = 4 * time.Second

// DrawClouds draws seeded, puffy clouds in the upper part of pixel rows
// [y0, y1). They drift right with the wall clock and wrap around; at night
// they fade so the stars show through. Bands under 8 px get no clouds.
func DrawClouds(c *pixel.Canvas, t time.Time, y0, y1 int, seed int64) {
	if c.W == 0 || y1-y0 < 8 {
		return
	}
	r := rand.New(rand.NewSource(seed ^ 0x5eed))
	shift := int(t.Unix() / int64(cloudDrift/time.Second))
	alpha := 0.85 - 0.6*Darkness(t)
	band := (y1 - y0) / 2
	for i := 0; i < max(1, c.W/28); i++ {
		w := 8 + r.Intn(9)
		span := c.W + w
		x0 := (r.Intn(span)+shift)%span - w
		y := y0 + 3 + r.Intn(band) // bumps reach 2 px above y, the base 1 px below
		drawCloud(c, x0, y, w, r, alpha)
	}
}

// drawCloud is a two-row base with rounded bumps along its top. The shape
// is collected first so overlapping puffs blend each pixel only once.
func drawCloud(c *pixel.Canvas, x0, y, w int, r *rand.Rand, alpha float64) {
	shape := map[[2]int]bool{}
	for dy := 0; dy < 2; dy++ {
		for x := x0; x < x0+w; x++ {
			shape[[2]int{x, y + dy}] = true
		}
	}
	for x := x0 + 1; x < x0+w-1; x += 3 + r.Intn(2) {
		rad := 1 + r.Intn(2)
		for dy := -rad; dy < 0; dy++ {
			for dx := -rad; dx <= rad; dx++ {
				if dx*dx+dy*dy <= rad*rad {
					shape[[2]int{x + dx, y + dy}] = true
				}
			}
		}
	}
	for p := range shape {
		c.Blend(p[0], p[1], cloudColor, alpha)
	}
}
