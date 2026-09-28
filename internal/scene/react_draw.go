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
	dropColor  = rainColor // steel blue: darker than the day sky, lighter than the night
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
	for k := 0; k < 6; k++ { // each drop falls for half a second
		t := s - (pour0 + (pour1-pour0)*float64(k)/6)
		if t < 0 || t > 0.5 {
			continue
		}
		f := t / 0.5
		x := hoverX - 2 - int(math.Round(f*float64(3+k%3)))
		y := hoverY + 2 + int(math.Round(f*f*float64(top-hoverY)))
		c.Set(x, y, dropColor)
	}
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

// drawWeedIn draws a new issue: a puff of dirt where its weed comes up. The
// weed itself grows in with the plant's new shape.
func drawWeedIn(c *pixel.Canvas, r Reaction, age time.Duration, cx, oy int, now *garden.Plant) {
	s := age.Seconds()
	if r.Before == nil || s >= 0.6 {
		return
	}
	sp, ok := now.WeedSpot(r.Before.OpenIssues)
	if !ok {
		return
	}
	x, y := sp.At(cx, plantBase(oy))
	a, lift := 1-s/0.6, int(s*4)
	for k, d := range [][2]int{{-2, 0}, {2, 0}, {-1, -1}, {1, -1}} {
		slotBlend(c, cx, x+d[0]*(1+lift/2), y+d[1]-lift*(k%2), dirtColor, a)
	}
}

// drawWeedOut draws a closed issue: the pulled weed lifts out of the soil,
// rises and fades.
func drawWeedOut(c *pixel.Canvas, r Reaction, age time.Duration, cx, oy int) {
	if r.Before == nil || r.Before.OpenIssues == 0 {
		return
	}
	i := r.Before.OpenIssues - 1 // the last weed is the one that goes
	sp, ok := r.Before.WeedSpot(i)
	if !ok {
		return
	}
	x, y := sp.At(cx, plantBase(oy))
	f := age.Seconds() / 1.5
	lift, a := int(math.Round(f*8)), 1-f
	slotBlend(c, cx, x, y-lift, weedGreen, a)
	if i%2 == 0 {
		slotBlend(c, cx, x, y-1-lift, weedGreen, a)
	}
}
