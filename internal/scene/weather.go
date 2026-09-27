package scene

import (
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// Weather is the sky over one plant, from its CI state.
type Weather int

const (
	Clear  Weather = iota // passing, or no CI
	Cloudy                // CI running
	Storm                 // CI failing
)

var (
	greyCloud   = rgb(172, 178, 190)
	stormCloud  = rgb(46, 48, 62)
	stormRim    = rgb(160, 160, 190) // a lit edge, so the storm reads against day and night skies
	lightning   = rgb(255, 240, 150)
	rainColor   = rgb(90, 110, 170) // darker than the day sky, lighter than the night
	shellColor  = rgb(150, 96, 56)
	shellSpiral = rgb(205, 160, 100)
	snailBody   = rgb(176, 172, 150)
)

// rainStep is how long a raindrop takes to fall one pixel; snailCrawl is how
// long the snail takes to move one. Lightning flashes for flashFor out of
// every flashEvery.
const (
	rainStep, snailCrawl = 90 * time.Millisecond, 3 * time.Second
	flashEvery, flashFor = 5 * time.Second, 300 * time.Millisecond
)

// drawWeather draws a plant's own weather at the top of its visible sky: a
// small grey cloud while CI runs; when it fails, a dark cloud with a lit rim,
// dense rain falling to the soil at row soil, and a lightning flash every few
// seconds, so a broken build shows from across the room by day and by night.
// Raindrops and flashes come from the clock, so they move in live frames.
func drawWeather(c *pixel.Canvas, cx, top, soil int, w Weather, t time.Time) {
	switch w {
	case Cloudy:
		puff(c, cx, top+1, 4, greyCloud)
	case Storm:
		const half = 8
		puff(c, cx, top+1, half, stormCloud)
		for dx := -half; dx <= half; dx++ { // light the cloud's upper edge
			y := top + 2
			switch {
			case dx >= -half/2 && dx <= half/2:
				y = top
			case dx > -half && dx < half:
				y = top + 1
			}
			c.Set(cx+dx, y, stormRim)
		}
		if fall := int64(soil - (top + 3)); fall > 0 {
			step := t.UnixMilli() / rainStep.Milliseconds()
			for x := cx - 7; x <= cx+7; x += 2 {
				y := top + 3 + int(((step+int64(x)*5)%fall+fall)%fall)
				c.Set(x, y, rainColor)
				c.Set(x, y+1, rainColor)
			}
		}
		if t.UnixMilli()%flashEvery.Milliseconds() < flashFor.Milliseconds() {
			for i, dx := range []int{-1, 0, -1, 0, 1} {
				c.Set(cx+dx, top+3+i, lightning)
			}
		}
	}
}

// puff is a flat-bottomed cloud 2·half+1 px wide whose base is row y+1.
func puff(c *pixel.Canvas, cx, y, half int, col pixel.RGB) {
	for dx := -half; dx <= half; dx++ {
		c.Set(cx+dx, y+1, col)
		if dx > -half && dx < half {
			c.Set(cx+dx, y, col)
		}
		if dx >= -half/2 && dx <= half/2 {
			c.Set(cx+dx, y-1, col)
		}
	}
}

// drawSnail draws a snail crawling on the soil left of the pot: a burst of
// new issues.
func drawSnail(c *pixel.Canvas, cx, oy int, t time.Time) {
	x := cx - BedCols/2 + 1 + int(t.Unix()/int64(snailCrawl/time.Second)%3)
	y := oy + groundTop - 1
	c.Set(x, y, shellColor)
	c.Set(x+1, y, shellSpiral)
	c.Set(x, y-1, shellColor)
	c.Set(x+1, y-1, shellColor)
	c.Set(x+2, y, snailBody)
	c.Set(x+3, y, snailBody)
	c.Set(x+3, y-1, snailBody)
}
