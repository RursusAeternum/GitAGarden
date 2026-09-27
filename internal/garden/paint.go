package garden

import (
	"math"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// Style is how a plant is painted in one frame.
type Style struct {
	Health float64 // 1 fresh … 0 fully wilted
	Sway   float64 // horizontal offset of the plant's top row, in pixels
}

func rgbOf(r, g, b uint8) pixel.RGB { return pixel.RGB{R: r, G: g, B: b} }

type palette struct {
	leaf, stem, body, flower, fruit pixel.RGB
}

var (
	speciesPalettes = map[Species]palette{
		Shrub:   {leaf: rgbOf(72, 165, 72), stem: rgbOf(118, 84, 56)},
		Cactus:  {leaf: rgbOf(235, 225, 190), stem: rgbOf(118, 84, 56), body: rgbOf(62, 150, 92)}, // leaves are spines
		Rosette: {leaf: rgbOf(112, 176, 138), stem: rgbOf(120, 150, 90)},
	}
	flowerColors = []pixel.RGB{rgbOf(255, 135, 200), rgbOf(255, 215, 90), rgbOf(245, 245, 245), rgbOf(190, 140, 255), rgbOf(255, 120, 90)}
	fruitColor   = rgbOf(230, 70, 60)
	weedColor    = rgbOf(130, 140, 60)
	dryColor     = rgbOf(200, 160, 70)
	deadColor    = rgbOf(122, 82, 48)
)

// palette is the species palette with a per-plant tint and flower color,
// so no two plants look identical.
func (p *Plant) palette() palette {
	pal := speciesPalettes[p.Species]
	tint := 0.88 + 0.24*hash01(p.Name, -1, -1)
	pal.leaf, pal.body = pal.leaf.Scale(tint), pal.body.Scale(tint)
	pal.flower = flowerColors[int(hash01(p.Name, -2, -2)*float64(len(flowerColors)))%len(flowerColors)]
	pal.fruit = fruitColor
	return pal
}

// wilt shifts a healthy color toward dry yellow, then dead brown.
func wilt(c pixel.RGB, h float64) pixel.RGB {
	switch {
	case h >= 0.6:
		return c
	case h >= 0.3:
		return pixel.Lerp(dryColor, c, (h-0.3)/0.3)
	}
	return pixel.Lerp(deadColor, dryColor, h/0.3)
}

// Paint draws p with its ground row on pixel row baseY, centered on
// column baseX. Higher rows are shifted by up to st.Sway pixels.
func Paint(c *pixel.Canvas, p *Plant, baseX, baseY int, st Style) {
	pal, h := p.palette(), st.Health
	for y := 0; y < Height; y++ {
		dx := int(math.Round(st.Sway * float64(ground-y) / float64(Height)))
		for x := 0; x < Width; x++ {
			cell := p.Grid[y][x]
			var col pixel.RGB
			switch cell.Kind {
			case Empty:
				continue
			case Stem:
				col = pal.stem
				if h < 0.3 {
					col = col.Scale(0.8)
				}
			case Body:
				col = wilt(pal.body, h)
			case Leaf:
				if h < 0.45 && hash01(p.Name, x, y) < (0.45-h)/0.45*0.75 {
					continue // dropped
				}
				col = pal.leaf
				if p.Species != Cactus {
					col = wilt(col, h)
				}
				col = col.Scale(1 + 0.12*float64(cell.Level))
			case Flower:
				col = pal.flower
				if h < 0.5 {
					col = pixel.Lerp(col, deadColor, 0.6)
				}
			case Fruit:
				col = pal.fruit
				if h < 0.3 {
					col = deadColor
				}
			}
			c.Set(baseX-center+x+dx, baseY-ground+y, col.Scale(0.9+0.2*hash01(p.Name, x, y)))
		}
	}
	for i := 0; i < p.OpenIssues && i < len(p.weedSlots); i++ {
		s := p.weedSlots[i]
		if p.Grid[ground][s].Kind != Empty {
			continue
		}
		c.Set(baseX-center+s, baseY, weedColor)
		if i%2 == 0 {
			c.Set(baseX-center+s, baseY-1, weedColor.Scale(1.15))
		}
	}
}
