package scene

import (
	"math"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// SkyMode picks what the night sky shows.
type SkyMode int

const (
	SkyRandom SkyMode = iota // seeded random stars
	SkyStars                 // one star per GitHub star in the garden
)

const starSeed = 0x5a7

// DrawStarSky draws one star per GitHub star in the upper part of pixel rows
// [y0, y1), as bright as the night is dark. Star i's place depends only on i
// and the stars before it, so a new star never moves the others. When there
// are more stars than the sky has room for (one per 40 px), the sky shows as
// many as fit and glows brighter instead of crowding.
func DrawStarSky(c *pixel.Canvas, y0, y1, total int, darkness float64) {
	y0 = max(y0, 0) // a short window crops the top of the sky; count only what shows
	if darkness <= 0 || total <= 0 || c.W == 0 || y1 <= y0 {
		return
	}
	band := max(1, (y1-y0)*2/3)
	capacity := max(1, c.W*(y1-y0)/40)
	n, boost := total, 0.0
	if total > capacity {
		n = capacity
		boost = math.Min(1, math.Log10(float64(total)/float64(capacity))/2)
	}
	taken := make(map[[2]int]bool, n)
	for i := 0; i < n; i++ {
		x := int(noise(starSeed, i, 1) * float64(c.W))
		y := y0 + int(noise(starSeed, i, 2)*float64(band))
		for tries := 0; taken[[2]int{x, y}] && tries < c.W*band; tries++ {
			if x++; x >= c.W { // the next free spot, left to right, top to bottom
				x, y = 0, y+1
				if y >= y0+band {
					y = y0
				}
			}
		}
		taken[[2]int{x, y}] = true
		bright := 0.4 + 0.6*noise(starSeed, i, 3)
		c.Blend(x, y, starColor, darkness*math.Min(1, bright+boost))
	}
}
