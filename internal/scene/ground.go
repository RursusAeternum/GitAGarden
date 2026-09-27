package scene

import "github.com/RursusAeternum/GitAGarden/internal/pixel"

var (
	soilTop  = rgb(110, 78, 50)
	soilDeep = rgb(80, 55, 35)
	potBody  = rgb(205, 110, 70)
	potShade = rgb(160, 80, 50)
	potRim   = rgb(185, 95, 60)
	potSoil  = rgb(60, 40, 28)
)

// DrawGround fills pixel rows [y0, y1) with soil that darkens with depth.
func DrawGround(c *pixel.Canvas, y0, y1 int, seed int64) {
	span := float64(max(1, y1-y0-1))
	for y := y0; y < y1; y++ {
		base := pixel.Lerp(soilTop, soilDeep, float64(y-y0)/span)
		for x := 0; x < c.W; x++ {
			c.Set(x, y, base.Scale(0.9+0.2*noise(seed, x, y)))
		}
	}
}

// Pot size in pixels: rim width and total height.
const PotW, PotH = 15, 8

// DrawPot draws a terracotta pot centered on cx: dark soil at row top, the
// rim below it, then a body that tapers and is shaded on the right.
func DrawPot(c *pixel.Canvas, cx, top int) {
	for j := 0; j < PotH; j++ {
		half := PotW / 2
		if j >= 2 {
			half -= (j - 1) / 2
		}
		for dx := -half; dx <= half; dx++ {
			var col pixel.RGB
			switch {
			case j == 0 && dx > -half && dx < half:
				col = potSoil
			case j <= 1:
				col = potRim
			case dx >= half-1:
				col = potShade
			case dx <= -half+2:
				col = potBody.Scale(1.1)
			default:
				col = potBody
			}
			c.Set(cx+dx, top+j, col)
		}
	}
}
