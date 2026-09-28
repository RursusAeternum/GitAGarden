package scene

import (
	"math"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

var (
	canColor   = rgb(96, 132, 128) // galvanised tin
	canShine   = rgb(170, 200, 196)
	droneBody  = rgb(150, 155, 165)
	droneRotor = rgb(210, 215, 225)
	dirtColor  = rgb(120, 85, 55)
	weedGreen  = rgb(130, 140, 60) // the garden's weed colour
)

// drawWatering draws a push. A watering can tips over the plant, or for an
// agent's push a drone flies in, hovers and flies off; either sprinkles an
// arc of drops that land on the plant at the change moment. top is the
// plant's top pixel row.
func drawWatering(c *pixel.Canvas, r Reaction, age time.Duration, cx, oy, top int) {
	s := age.Seconds()
	hoverX, hoverY := cx+4, max(top-7, oy+1)
	pour0, pour1 := 0.3, 1.1 // when the first and last drops leave
	if r.Drone {
		pour0, pour1 = 0.6, 1.4
		x := hoverX
		switch {
		case s < 0.6:
			x = hoverX + int(math.Round((1-s/0.6)*12)) // flies in from the right
		case s > 1.8:
			x = hoverX - int(math.Round((s-1.8)/1.2*24)) // and off to the left
		}
		drawDrone(c, x, hoverY, s)
	} else if s < 1.6 {
		drawCan(c, hoverX, hoverY, s > pour0)
	}
	drop := dropFor(r.Start.Add(age))
	for k := 0; k < 6; k++ { // each drop falls for half a second
		t := s - (pour0 + (pour1-pour0)*float64(k)/6)
		if t < 0 || t > 0.5 {
			continue
		}
		f := t / 0.5
		x := hoverX - 2 - int(math.Round(f*float64(3+k%3)))
		y := hoverY + 2 + int(math.Round(f*f*float64(top-hoverY)))
		c.Set(x, y, drop)
	}
}

// dropBlues are the colours a drop can take. The sky passes through blues
// at dawn and dusk, so each frame uses whichever stands out most.
var dropBlues = []pixel.RGB{rainColor, rgb(200, 230, 255), rgb(30, 50, 110)}

// dropFor is the drop colour that stands out most from the sky at now.
func dropFor(now time.Time) pixel.RGB {
	top, bottom := SkyAt(now)
	best, far := dropBlues[0], -1
	for _, c := range dropBlues {
		if d := min(sqDist(c, top), sqDist(c, bottom)); d > far {
			best, far = c, d
		}
	}
	return best
}

// sqDist is the squared distance between two colours.
func sqDist(a, b pixel.RGB) int {
	dr, dg, db := int(a.R)-int(b.R), int(a.G)-int(b.G), int(a.B)-int(b.B)
	return dr*dr + dg*dg + db*db
}

// drawCan is a small watering can at (x, y): a body 4 px wide, a handle on
// the right and a spout to the left that dips while it pours.
func drawCan(c *pixel.Canvas, x, y int, pouring bool) {
	for dx := 0; dx < 4; dx++ {
		for dy := 0; dy < 3; dy++ {
			c.Set(x+dx, y+dy, canColor)
		}
	}
	c.Set(x+1, y, canShine)
	c.Set(x+4, y+1, canColor)
	c.Set(x-1, y+1, canColor)
	if pouring {
		c.Set(x-2, y+2, canColor)
	} else {
		c.Set(x-2, y, canColor)
	}
}

// drawDrone is a small quadcopter centered on column x: rotor arms along row
// y whose blades blur as they spin, a grey body and a nozzle below.
func drawDrone(c *pixel.Canvas, x, y int, secs float64) {
	spin := int(secs*20) % 2
	for dx := -3; dx <= 3; dx++ {
		if dx == -3 || dx == 3 || (dx+3+spin)%2 == 0 {
			c.Set(x+dx, y, droneRotor)
		}
	}
	for dx := -1; dx <= 1; dx++ {
		c.Set(x+dx, y+1, droneBody)
	}
	c.Set(x, y+2, critterInk)
}

// drawWeedIn draws a new issue: its weeds grow up from the soil with a puff
// of dirt, next to the weeds shown already. shown is the plant as drawn this
// frame, which doesn't count them yet; now is the plant after the change.
func drawWeedIn(c *pixel.Canvas, r Reaction, age time.Duration, cx, oy int, shown, now *garden.Plant) {
	if r.Before == nil {
		return
	}
	s, g := age.Seconds(), min(1, float64(age)/float64(growFor))
	for k := 0; k < weedsGained(r.Before, now); k++ {
		i := shown.OpenIssues + k
		sp, ok := shown.WeedSpot(i)
		if !ok {
			continue
		}
		x, y := sp.At(cx, plantBase(oy))
		if age < growFor { // as the garden paints a weed, growing up
			slotBlend(c, cx, x, y, weedGreen, min(1, 2*g))
			if i%2 == 0 {
				slotBlend(c, cx, x, y-1, weedGreen.Scale(1.15), max(0, 2*g-1))
			}
		}
		if s < 0.6 {
			a, lift := 1-s/0.6, int(s*4)
			for j, d := range [][2]int{{-2, 0}, {2, 0}, {-1, -1}, {1, -1}} {
				slotBlend(c, cx, x+d[0]*(1+lift/2), y+d[1]-lift*(j%2), dirtColor, a)
			}
		}
	}
}

// drawWeedOut draws a closed issue: its weeds, the last ones shown until it
// started, lift out of the soil, rise and fade. shown is the plant as drawn
// this frame, which no longer counts them; now is the plant after the change.
func drawWeedOut(c *pixel.Canvas, r Reaction, age time.Duration, cx, oy int, shown, now *garden.Plant) {
	if r.Before == nil {
		return
	}
	f := age.Seconds() / 1.5
	lift, a := int(math.Round(f*8)), 1-f
	for k := 0; k < weedsLost(r.Before, now); k++ {
		i := shown.OpenIssues + k
		sp, ok := shown.WeedSpot(i)
		if !ok {
			continue
		}
		x, y := sp.At(cx, plantBase(oy))
		slotBlend(c, cx, x, y-lift, weedGreen, a)
		if i%2 == 0 {
			slotBlend(c, cx, x, y-1-lift, weedGreen, a)
		}
	}
}

var (
	budPink    = rgb(235, 110, 150)
	petalColor = rgb(255, 170, 210)
	sparkGold  = rgb(255, 215, 90)
	sparkWhite = rgb(255, 255, 255)
	rainbow    = []pixel.RGB{rgb(230, 80, 70), rgb(240, 150, 60), rgb(240, 220, 90),
		rgb(110, 200, 100), rgb(90, 150, 230), rgb(150, 110, 210)}
)

// drawBurst draws a merge. A bud swells where the new flower opens and
// bursts into petals at the change moment, then a butterfly lifts off and
// flies away.
func drawBurst(c *pixel.Canvas, r Reaction, age time.Duration, cx, oy int, now *garden.Plant) {
	x, y := cx, plantTop(now, oy)
	if r.Before != nil {
		if spots := now.NewSpots(r.Before, garden.Flower); len(spots) > 0 {
			x, y = spots[0].At(cx, plantBase(oy))
		}
	}
	s := age.Seconds()
	switch {
	case s < 0.6:
		slotBlend(c, cx, x, y, budPink, 1)
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			slotBlend(c, cx, x+d[0], y+d[1], budPink, s/0.6)
		}
	case s < 1.0:
		f := (s - 0.6) / 0.4
		rad := 1 + f*3
		for k := 0; k < 8; k++ {
			th := float64(k) * math.Pi / 4
			slotBlend(c, cx, x+int(math.Round(rad*math.Cos(th))), y+int(math.Round(rad*math.Sin(th))), petalColor, 1-f/2)
		}
	}
	if s >= 1.0 {
		f := (s - 1.0) / 2.0
		bx := x + int(math.Round(f*14))
		by := y - int(math.Round(f*18)) + int(math.Round(math.Sin(s*9)))
		drawButterfly(c, bx, by, wingColors[int(r.Seed&0xff)%len(wingColors)], int(s*6)%2 == 0)
	}
}

// drawSparkle draws a release: gold and white sparkles burst out around the
// plant, then two bees circle it and leave.
func drawSparkle(c *pixel.Canvas, r Reaction, age time.Duration, cx, oy int, now *garden.Plant) {
	s := age.Seconds()
	mx, my := cx, (plantTop(now, oy)+plantBase(oy))/2
	if s < 1.5 {
		rad, a := 3+7*s/1.5, 1-s/1.5
		for k := 0; k < 10; k++ {
			th := float64(k)*math.Pi/5 + noise(r.Seed, k, 1)
			x, y := mx+int(math.Round(rad*math.Cos(th))), my+int(math.Round(rad*0.8*math.Sin(th)))
			slotBlend(c, cx, x, y, sparkWhite, a)
			for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				slotBlend(c, cx, x+d[0], y+d[1], sparkGold, a*0.8)
			}
		}
	}
	if s >= 1.0 {
		t := s - 1.0
		out := math.Max(0, t-2.2) / 0.8 // in their last 0.8 s they fly off
		for i := 0; i < 2; i++ {
			th, rad := 2.4*t+float64(i)*math.Pi, 7+20*out
			drawBee(c, mx+int(math.Round(rad*math.Cos(th))), my+int(math.Round(rad*0.6*math.Sin(th))))
		}
	}
}

// drawStormIn draws CI going red. The storm cloud slides down into place,
// darkening as it comes, and flashes once as it arrives; after that the
// plot's steady storm takes over.
func drawStormIn(c *pixel.Canvas, age time.Duration, cx, oy int) {
	top, s := max(oy, 0), age.Seconds()
	if s < 1.5 {
		f := s / 1.5
		cloudIn(c, cx, cx, top-5+int(math.Round(f*6)), 8, pixel.Lerp(greyCloud, stormCloud, f), 1, top)
		return
	}
	if s < 1.8 {
		for i, dx := range []int{-1, 0, -1, 0, 1} {
			c.Set(cx+dx, top+3+i, lightning)
		}
	}
}

// drawClearing draws CI recovering: the storm cloud drifts off and fades,
// and a rainbow arcs over the plant, fading in and out.
func drawClearing(c *pixel.Canvas, age time.Duration, cx, oy int) {
	top, s := max(oy, 0), age.Seconds()
	if s < 1.5 {
		f := s / 1.5
		cloudIn(c, cx, cx+int(math.Round(f*6)), top+1, 8, stormCloud, 1-f, top)
	}
	if s < 1.0 || s >= 5.0 {
		return
	}
	a := 0.85 * math.Min(1, (s-1.0)/0.5) * math.Min(1, (5.0-s)/0.5)
	for b, col := range rainbow {
		rad := float64(10 - b)
		for k := 0; k <= 24; k++ {
			th := math.Pi + math.Pi*float64(k)/24
			slotBlend(c, cx, cx+int(math.Round(rad*math.Cos(th))), top+13+int(math.Round(rad*0.7*math.Sin(th))), col, a)
		}
	}
}

// cloudIn draws a cloud the shape of puff, centered on column x with its base
// on row y+1, blended by a. It is clipped to the slot centered on slotCx and
// to rows from minY down.
func cloudIn(c *pixel.Canvas, slotCx, x, y, half int, col pixel.RGB, a float64, minY int) {
	for dx := -half; dx <= half; dx++ {
		for _, yy := range []int{y - 1, y, y + 1} {
			in := yy == y+1 || (yy == y && dx > -half && dx < half) || (yy == y-1 && dx >= -half/2 && dx <= half/2)
			if in && yy >= minY {
				slotBlend(c, slotCx, x+dx, yy, col, a)
			}
		}
	}
}
