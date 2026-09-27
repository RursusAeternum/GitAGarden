package scene

import (
	"math"
	"math/rand"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

type skyKey struct {
	hour        float64
	top, bottom pixel.RGB
}

var (
	night   = skyKey{0, rgb(8, 10, 30), rgb(20, 24, 60)}
	day     = skyKey{0, rgb(80, 150, 230), rgb(175, 215, 245)}
	skyKeys = []skyKey{
		{0, night.top, night.bottom},
		{5, night.top, night.bottom},
		{6.5, rgb(90, 110, 180), rgb(250, 170, 120)},
		{9, day.top, day.bottom},
		{17, day.top, day.bottom},
		{19, rgb(60, 60, 130), rgb(240, 130, 90)},
		{20.5, night.top, night.bottom},
		{24, night.top, night.bottom},
	}
	sunColor  = rgb(255, 230, 140)
	moonColor = rgb(230, 230, 215)
	starColor = rgb(255, 255, 230)
)

// hourOf is the fractional hour on the viewer's local clock.
func hourOf(t time.Time) float64 {
	t = t.Local()
	return float64(t.Hour()) + float64(t.Minute())/60
}

// SkyAt is the sky gradient's top and bottom color at t's local time.
func SkyAt(t time.Time) (top, bottom pixel.RGB) {
	h := hourOf(t)
	for i := 1; i < len(skyKeys); i++ {
		if h < skyKeys[i].hour {
			a, b := skyKeys[i-1], skyKeys[i]
			f := (h - a.hour) / (b.hour - a.hour)
			return pixel.Lerp(a.top, b.top, f), pixel.Lerp(a.bottom, b.bottom, f)
		}
	}
	k := skyKeys[len(skyKeys)-1]
	return k.top, k.bottom
}

// Darkness is 0 in daylight, 1 at night, and in between at dusk and dawn.
func Darkness(t time.Time) float64 {
	switch h := hourOf(t); {
	case h < 5 || h >= 20.5:
		return 1
	case h < 6.5:
		return (6.5 - h) / 1.5
	case h >= 19:
		return (h - 19) / 1.5
	}
	return 0
}

// DrawSky fills pixel rows [y0, y1) with the sky at time t, with stars.
// The sun or moon is drawn separately, once per frame, by Compose.
func DrawSky(c *pixel.Canvas, t time.Time, y0, y1 int, seed int64) {
	skyGradient(c, t, y0, y1)
	DrawStars(c, y0, y1, seed, Darkness(t))
}

// skyGradient fills pixel rows [y0, y1) with the sky's colour at time t.
func skyGradient(c *pixel.Canvas, t time.Time, y0, y1 int) {
	top, bot := SkyAt(t)
	span := float64(max(1, y1-y0-1))
	for y := y0; y < y1; y++ {
		col := pixel.Lerp(top, bot, float64(y-y0)/span)
		for x := 0; x < c.W; x++ {
			c.Set(x, y, col)
		}
	}
}

// DrawStars scatters seeded stars over the upper part of the band, as
// bright as the night is dark.
func DrawStars(c *pixel.Canvas, y0, y1 int, seed int64, darkness float64) {
	if darkness <= 0 || c.W == 0 || y1 <= y0 {
		return
	}
	r := rand.New(rand.NewSource(seed))
	band := max(1, (y1-y0)*2/3)
	for i := 0; i < c.W*(y1-y0)/40; i++ {
		x, y := r.Intn(c.W), y0+r.Intn(band)
		c.Blend(x, y, starColor, darkness*(0.4+0.6*r.Float64()))
	}
}

// drawSunMoon moves the sun across the band from 06:00 to 20:00, and the
// moon from 20:00 to 06:00, highest at the middle of its arc.
func drawSunMoon(c *pixel.Canvas, t time.Time, y0, y1 int) {
	h := hourOf(t)
	col, frac := sunColor, (h-6)/14
	if h < 6 || h >= 20 {
		if h < 6 {
			h += 24
		}
		col, frac = moonColor, (h-20)/10
	}
	x := int(frac * float64(c.W-1))
	y := y0 + int(float64(y1-y0)*(0.12+0.5*math.Pow(2*frac-1, 2)))
	for dy := -2; dy <= 2; dy++ {
		if y+dy < y0 || y+dy >= y1 {
			continue // stay inside this band; beds stack vertically
		}
		for dx := -2; dx <= 2; dx++ {
			if dx*dx+dy*dy <= 5 {
				c.Set(x+dx, y+dy, col)
			}
		}
	}
}
