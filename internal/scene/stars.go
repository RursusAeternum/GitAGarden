package scene

import (
	"math"
	"time"

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

// ShootingFor is how long a shooting star takes to cross the sky.
const ShootingFor = 1500 * time.Millisecond

// Shooting is a shooting star that starts at Start; Seed picks its path.
type Shooting struct {
	Start time.Time
	Seed  int64
}

var (
	meteorHead = rgb(255, 250, 225)
	meteorTail = rgb(255, 205, 120) // warm, so it shows against the pale day sky too
)

const meteorSlope = 0.35 // pixels down per pixel across

// drawShootingStars draws the shooting stars in flight at t across pixel
// rows [y0, y1): a bright head and a fading warm tail, sliding down and to
// the right. Each starts in the top third of the sky and ends above y1, so
// it never streaks into the plants.
func drawShootingStars(c *pixel.Canvas, t time.Time, shots []Shooting, y0, y1 int) {
	y0 = max(y0, 0) // a short window crops the top of the sky
	if y1-y0 < 6 {
		return
	}
	for _, s := range shots {
		age := t.Sub(s.Start)
		if age < 0 || age >= ShootingFor {
			continue
		}
		f := float64(age) / float64(ShootingFor)
		x0 := int(noise(s.Seed, 0, 1) * float64(c.W) * 0.6)
		ys := y0 + int(noise(s.Seed, 0, 2)*float64((y1-y0)/3))
		run := math.Min(float64(c.W)*0.4, float64(y1-1-ys)/meteorSlope)
		for k := 0; k < 8; k++ { // the head, then its tail
			d := f*run - float64(k)*1.5
			if d < 0 {
				break
			}
			col, a := meteorTail, 0.9*(1-float64(k)/8)
			if k == 0 {
				col, a = meteorHead, 1
			}
			c.Blend(x0+int(d), ys+int(d*meteorSlope), col, a)
		}
	}
}
