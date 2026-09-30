package garden

import (
	"hash/fnv"
	"math"
	"sort"
	"time"
)

// Size of the plant grid in pixels, excluding pot and scene.
const (
	Width  = 23
	Height = 32
	center = Width / 2
	ground = Height - 1 // bottom row of the grid, just above the pot soil
)

type CellKind uint8

const (
	Empty CellKind = iota
	Stem
	Body // succulent flesh, e.g. a cactus column
	Leaf
	Flower
	Fruit
)

type Cell struct {
	Kind  CellKind
	Level uint8 // how lush a leaf is; grows once there is no room for new ones
	At    int64 // Unix seconds of the event that grew or last thickened it; 0 for the seedling
}

type pt struct{ x, y int }

// Plant is the result of replaying a repo's events onto a seeded canvas.
// It holds no time-dependent state; wilting is applied at render time.
type Plant struct {
	Name    string
	Species Species
	Grid    [Height][Width]Cell

	Pushes, Merges, Releases int
	OpenIssues               int
	LastTended               time.Time

	weedSlots []int
	stamp     int64 // the time of the event being replayed; new cells get it
}

// Grow grows a plant from a repo's whole history, as of its last event.
// The same name, species and events always produce the same plant.
func Grow(name string, sp Species, events []Event) *Plant {
	return GrowAt(name, sp, Totals{}, events, time.Time{})
}

func (p *Plant) inBounds(x, y int) bool {
	return x >= 0 && x < Width && y >= 0 && y < Height
}

func (p *Plant) free(x, y int) bool {
	return p.inBounds(x, y) && p.Grid[y][x].Kind == Empty
}

func (p *Plant) set(x, y int, k CellKind) {
	p.Grid[y][x] = p.cell(k)
}

// cell is a new cell of kind k, born at the event being replayed.
func (p *Plant) cell(k CellKind) Cell { return Cell{Kind: k, At: p.stamp} }

func (p *Plant) cellsOf(kinds ...CellKind) []pt {
	var out []pt
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			for _, k := range kinds {
				if p.Grid[y][x].Kind == k {
					out = append(out, pt{x, y})
					break
				}
			}
		}
	}
	return out
}

func seedOf(s string) int64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return int64(h.Sum64())
}

// growthFrac maps a push count onto 0..1 logarithmically: the first few
// commits count most, and ~500 fill the plot.
func growthFrac(pushes int) float64 {
	return math.Min(1, math.Log2(1+float64(pushes))/math.Log2(513))
}

const seedlingHeight = 4

// heightCap is how many pixels above the ground a plant may reach.
func heightCap(pushes int) int {
	return seedlingHeight + int(growthFrac(pushes)*float64(Height-2-seedlingHeight))
}

// Flowers is how many flowers the plant has.
func (p *Plant) Flowers() int { return len(p.cellsOf(Flower)) }

// How signals age: flowers go to seed flowerLife after their merge, a PR's
// bud droops once it has waited budDroop, and on a rising plant growth
// younger than shootAge shows as fresh shoots. At most MaxBuds buds are shown.
const (
	flowerLife = 30 * 24 * time.Hour
	budDroop   = 7 * 24 * time.Hour
	shootAge   = 14 * 24 * time.Hour
	MaxBuds    = 5
)

// Blooming is how many of the plant's flowers are still in bloom at now. A
// zero now counts them all.
func (p *Plant) Blooming(now time.Time) int {
	n := 0
	for _, q := range p.cellsOf(Flower) {
		if !seedHead(p.Grid[q.y][q.x], now) {
			n++
		}
	}
	return n
}

// seedHead reports whether a flower has gone to seed by now.
func seedHead(c Cell, now time.Time) bool {
	return !now.IsZero() && c.At != 0 && now.Sub(time.Unix(c.At, 0)) > flowerLife
}

// fresh reports whether a cell grew less than shootAge before now.
func fresh(c Cell, now time.Time) bool {
	return !now.IsZero() && c.At != 0 && now.Sub(time.Unix(c.At, 0)) <= shootAge
}

// budSpots returns up to n free cells for buds: just above the plant's
// highest stems, leaves and flesh, never touching each other. The same
// plant always gives the same spots.
func (p *Plant) budSpots(n int) []pt {
	var cands []pt
	for y := 1; y < ground; y++ {
		for x := 0; x < Width; x++ {
			if !p.free(x, y) {
				continue
			}
			if below := p.Grid[y+1][x].Kind; below == Stem || below == Leaf || below == Body {
				cands = append(cands, pt{x, y})
			}
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].y != cands[j].y {
			return cands[i].y < cands[j].y
		}
		return hash01(p.Name, cands[i].x, -3) < hash01(p.Name, cands[j].x, -3)
	})
	var out []pt
	for _, c := range cands {
		if len(out) == n {
			break
		}
		touching := false
		for _, o := range out {
			if max(o.x-c.x, c.x-o.x) <= 1 && max(o.y-c.y, c.y-o.y) <= 1 {
				touching = true
				break
			}
		}
		if !touching {
			out = append(out, c)
		}
	}
	return out
}

// Spot is a cell of the plant grid: X from the left, Y from the top.
type Spot struct{ X, Y int }

// At is where the spot lands on a canvas for a plant painted with its ground
// row on baseY and centered on column baseX, as Paint places it before sway.
func (s Spot) At(baseX, baseY int) (int, int) { return baseX - center + s.X, baseY - ground + s.Y }

// NewSpots lists the cells of kind k that p has and before hadn't, top to
// bottom, left to right.
func (p *Plant) NewSpots(before *Plant, k CellKind) []Spot {
	var out []Spot
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			if p.Grid[y][x].Kind == k && before.Grid[y][x].Kind != k {
				out = append(out, Spot{x, y})
			}
		}
	}
	return out
}

// NewestFlower is where the flower of the newest merge opened, and false
// when the plant has no flower.
func (p *Plant) NewestFlower() (Spot, bool) {
	var best Spot
	newest := int64(-1)
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			if c := p.Grid[y][x]; c.Kind == Flower && c.At > newest {
				best, newest = Spot{x, y}, c.At
			}
		}
	}
	return best, newest >= 0
}

// WeedSpot is where the i-th weed grows, and false when that weed isn't
// drawn: past the last slot, or where the plant itself stands.
func (p *Plant) WeedSpot(i int) (Spot, bool) {
	if i < 0 || i >= len(p.weedSlots) {
		return Spot{}, false
	}
	s := p.weedSlots[i]
	return Spot{s, ground}, p.Grid[ground][s].Kind == Empty
}

// Top is the grid row of the plant's highest cell, or the ground row when it
// has none.
func (p *Plant) Top() int {
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			if p.Grid[y][x].Kind != Empty {
				return y
			}
		}
	}
	return ground
}
