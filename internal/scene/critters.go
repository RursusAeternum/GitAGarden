package scene

import (
	"math"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

var (
	beeBody    = rgb(250, 200, 40)
	critterInk = rgb(40, 30, 20)
	beeWing    = rgb(230, 240, 255)
	wingColors = []pixel.RGB{rgb(255, 160, 60), rgb(120, 180, 255), rgb(250, 250, 250)}
)

// Host is the pixel a critter circles: the middle of a flowering plant.
type Host struct{ X, Y int }

// Flowering reports whether a plot attracts critters: a healthy, blooming
// plant that isn't under glass.
func Flowering(pl Plot) bool {
	return !pl.Finished && pl.Style.Health >= 0.6 && pl.Plant.Flowers() > 0
}

// DrawCritters draws up to three bees and butterflies looping around the
// hosts, in daylight only. Their paths are pure functions of time, so a
// frame is reproducible.
func DrawCritters(c *pixel.Canvas, t time.Time, hosts []Host, seed int64) {
	if Darkness(t) > 0.5 {
		return
	}
	secs := float64(t.UnixMilli()) / 1000
	for i := 0; i < min(3, len(hosts)); i++ {
		h := hosts[i]
		phase := noise(seed, i, 7) * 2 * math.Pi
		speed := 0.35 + 0.2*noise(seed, i, 8) // radians per second
		x := h.X + int(math.Round(7*math.Sin(speed*secs+phase)))
		y := h.Y + int(math.Round(4*math.Sin(2*speed*secs+phase)))
		if i%2 == 0 {
			c.Set(x, y, beeBody)
			c.Set(x+1, y, critterInk)
			c.Set(x, y-1, beeWing)
			continue
		}
		wing := wingColors[i%len(wingColors)]
		c.Set(x, y, critterInk)
		if int(secs*6)%2 == 0 { // wings open
			c.Set(x-1, y-1, wing)
			c.Set(x+1, y-1, wing)
		} else {
			c.Set(x-1, y, wing)
			c.Set(x+1, y, wing)
		}
	}
}
