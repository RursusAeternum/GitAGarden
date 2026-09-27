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
	stormCloud  = rgb(70, 74, 92)
	rainColor   = rgb(150, 180, 230)
	shellColor  = rgb(150, 96, 56)
	shellSpiral = rgb(205, 160, 100)
	snailBody   = rgb(176, 172, 150)
)

// rainStep is how long a raindrop takes to fall one pixel; snailCrawl is how
// long the snail takes to move one.
const rainStep, snailCrawl = 90 * time.Millisecond, 3 * time.Second

// drawWeather draws a plant's own weather in its headroom: a small grey cloud
// while CI runs, a dark cloud with rain falling to the pot when it fails.
// Raindrop positions come from the clock, so rain falls in live frames.
func drawWeather(c *pixel.Canvas, cx, oy int, w Weather, t time.Time) {
	switch w {
	case Cloudy:
		puff(c, cx, oy+1, 4, greyCloud)
	case Storm:
		puff(c, cx, oy+1, 8, stormCloud)
		fall := int64(potTop - 3)
		step := t.UnixMilli() / rainStep.Milliseconds()
		for x := cx - 7; x <= cx+7; x += 3 {
			y := oy + 3 + int(((step+int64(x)*5)%fall+fall)%fall)
			c.Blend(x, y, rainColor, 0.85)
			c.Blend(x, y+1, rainColor, 0.6)
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
