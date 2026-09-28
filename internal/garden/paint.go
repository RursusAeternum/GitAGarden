package garden

import (
	"math"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// Style is how a plant is painted in one frame.
type Style struct {
	Health float64     // 1 fresh … 0 fully wilted
	Sway   float64     // horizontal offset of the plant's top row, in pixels
	Now    time.Time   // the frame's time; ages flowers and buds (zero: all fresh)
	Buds   []time.Time // when each open PR was opened; up to MaxBuds are drawn
	Rising bool        // more commits lately than before: recent growth shows as shoots
	Before *Plant      // an earlier shape of this plant: the cells it lacks grow in
	Grown  float64     // how far they have come in, 0 to 1; by the stem first
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
	seedColor    = rgbOf(214, 196, 150)
	shootColor   = rgbOf(185, 245, 105)
	budColor     = rgbOf(235, 110, 150)
	paleBud      = rgbOf(215, 185, 175)
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

// swayAt is how far row y is shifted by the plant's sway.
func swayAt(st Style, y int) int {
	return int(math.Round(st.Sway * float64(ground-y) / float64(Height)))
}

// Paint draws p with its ground row on pixel row baseY, centered on
// column baseX. Higher rows are shifted by up to st.Sway pixels.
func Paint(c *pixel.Canvas, p *Plant, baseX, baseY int, st Style) {
	pal, h := p.palette(), st.Health
	for y := 0; y < Height; y++ {
		dx := swayAt(st, y)
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
				if seedHead(cell, st.Now) {
					col = seedColor
				}
				if h < 0.5 {
					col = pixel.Lerp(col, deadColor, 0.6)
				}
			case Fruit:
				col = pal.fruit
				if h < 0.3 {
					col = deadColor
				}
			}
			if st.Rising && (cell.Kind == Stem || cell.Kind == Leaf || cell.Kind == Body) && fresh(cell, st.Now) {
				col = pixel.Lerp(col, shootColor, 0.55)
			}
			col = col.Scale(0.9 + 0.2*hash01(p.Name, x, y))
			a := 1.0
			if st.Before != nil && st.Before.Grid[y][x].Kind != cell.Kind {
				a = unfurl(st.Grown, x) // a new cell, still coming in
			}
			c.Blend(baseX-center+x+dx, baseY-ground+y, col, a)
		}
	}
	for i := 0; i < p.OpenIssues && i < len(p.weedSlots); i++ {
		s := p.weedSlots[i]
		if p.Grid[ground][s].Kind != Empty {
			continue
		}
		low, high := 1.0, 1.0
		if st.Before != nil && i >= st.Before.OpenIssues { // a new weed grows up from the soil
			low, high = math.Min(1, 2*st.Grown), math.Max(0, 2*st.Grown-1)
		}
		c.Blend(baseX-center+s, baseY, weedColor, low)
		if i%2 == 0 {
			c.Blend(baseX-center+s, baseY-1, weedColor.Scale(1.15), high)
		}
	}
	for i, s := range p.budSpots(min(len(st.Buds), MaxBuds)) {
		col, y := budColor, s.y
		if !st.Now.IsZero() && st.Now.Sub(st.Buds[i]) > budDroop {
			col, y = paleBud, s.y+1 // waited too long: it droops and fades
		}
		c.Set(baseX-center+s.x+swayAt(st, y), baseY-ground+y, col)
	}
}

// unfurl is how far a new cell in column x has come in once the plant's new
// cells are g of the way in: those by the stem first, the outermost last.
func unfurl(g float64, x int) float64 {
	d := math.Abs(float64(x-center)) / center
	return math.Max(0, math.Min(1, 2*g-d))
}
