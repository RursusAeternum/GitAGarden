package garden

import (
	"math/rand"
	"sort"
	"strings"
)

type Species string

const (
	Shrub   Species = "shrub"
	Cactus  Species = "cactus"
	Rosette Species = "rosette"
)

var AllSpecies = []Species{Shrub, Cactus, Rosette}

// SpeciesFor picks a species from a repo's primary language, so the garden
// reads at a glance.
func SpeciesFor(language string) Species {
	switch strings.ToLower(language) {
	case "rust", "c", "c++", "c#", "java", "zig":
		return Cactus
	case "python", "javascript", "typescript", "ruby":
		return Rosette
	}
	return Shrub
}

func (s Species) Next() Species {
	for i, sp := range AllSpecies {
		if sp == s {
			return AllSpecies[(i+1)%len(AllSpecies)]
		}
	}
	return AllSpecies[0]
}

const maxLevel = 2

// A habit is how a species grows. Its skeleton grows one structural step at
// a time, from the plant's own random stream; the canopy then goes into the
// spots it offers, all inside the species' silhouette.
type habit interface {
	// step grows the skeleton by one structural step, no higher than row top.
	step(top int)
	// leafSpots are the cells leaves may take, in the plant's own order.
	leafSpots() []pt
	// flowerSpots are where flowers open.
	flowerSpots() []pt
}

// fullSteps is how many structural steps a species takes to reach full
// size, at about 500 pushes.
var fullSteps = map[Species]int{Shrub: 34, Cactus: 44, Rosette: 60}

func newHabit(sp Species, p *Plant, r *rand.Rand) habit {
	b := base{p: p, r: r}
	switch sp {
	case Cactus:
		return newCactus(b)
	case Rosette:
		return newRosette(b)
	}
	return newShrub(b)
}

type base struct {
	p *Plant
	r *rand.Rand
}

var dirs8 = []pt{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}

// near lists the empty cells above the ground row within Chebyshev distance
// d of a cell of one of kinds, top to bottom, left to right.
func (b base) near(d int, kinds ...CellKind) []pt {
	var out []pt
	for y := 0; y < ground; y++ {
		for x := 0; x < Width; x++ {
			if !b.p.free(x, y) {
				continue
			}
		search:
			for dy := -d; dy <= d; dy++ {
				for dx := -d; dx <= d; dx++ {
					if b.p.inBounds(x+dx, y+dy) {
						for _, k := range kinds {
							if b.p.Grid[y+dy][x+dx].Kind == k {
								out = append(out, pt{x, y})
								break search
							}
						}
					}
				}
			}
		}
	}
	return out
}

// shuffled is ps in an order of the plant's own: the same plant always gives
// the same order, whatever else happens.
func (b base) shuffled(ps []pt, salt int) []pt {
	out := append([]pt(nil), ps...)
	sort.SliceStable(out, func(i, j int) bool {
		return hash01(b.p.Name, out[i].x+salt*100, out[i].y) < hash01(b.p.Name, out[j].x+salt*100, out[j].y)
	})
	return out
}

// ---- shrub: branching woody stems; leaves cluster around them in a
// rounded crown ----

type tip struct{ x, y, lean int }

type shrub struct {
	base
	tips []tip
}

func newShrub(b base) *shrub {
	for y := ground; y > ground-3; y-- { // seedling: a short stem
		b.p.set(center, y, Stem)
	}
	return &shrub{base: b, tips: []tip{{center, ground - 2, 0}}}
}

// step extends one branch tip upwards, sometimes forking. A tip held back
// only by the height limit stays alive for later; one boxed in is dropped.
func (s *shrub) step(top int) {
	var dead []int
	defer func() {
		sort.Sort(sort.Reverse(sort.IntSlice(dead)))
		for _, i := range dead {
			s.tips = append(s.tips[:i], s.tips[i+1:]...)
		}
	}()
	for _, i := range s.r.Perm(len(s.tips)) {
		t := &s.tips[i]
		leans := []int{t.lean, -1, 0, 1}
		s.r.Shuffle(3, func(a, b int) { leans[a+1], leans[b+1] = leans[b+1], leans[a+1] })
		for _, dx := range leans {
			x, y := t.x+dx, t.y-1
			if y < top || !s.p.free(x, y) || abs(x-center) > 2*(ground-y)/5+2 { // a widening cone: bushy, not sprawling
				continue
			}
			s.p.set(x, y, Stem)
			t.x, t.y = x, y
			if s.r.Float64() < 0.25 {
				t.lean = dx
			}
			if len(s.tips) < 7 && y < ground-4 && s.r.Float64() < 0.25 {
				s.tips = append(s.tips, tip{x, y, []int{-1, 1}[s.r.Intn(2)]})
			}
			return
		}
		if t.y-1 >= top {
			dead = append(dead, i)
		}
	}
}

// crown is the shrub's rounded crown: an ellipse around its stems, clear
// of the bare lower trunk. in reports whether a cell lies inside it.
func (s *shrub) crown(stems []pt) (in func(x, y int) bool) {
	top, left, right := ground, center, center
	for _, q := range stems {
		top, left, right = min(top, q.y), min(left, q.x), max(right, q.x)
	}
	bottom := ground - 3
	if top >= bottom {
		return func(x, y int) bool { return y >= top-1 && y < ground }
	}
	cy, ry := float64(top+bottom)/2, float64(bottom-top)/2+2
	cx, rx := float64(left+right)/2, float64(right-left)/2+4
	return func(x, y int) bool { // never more than a row above the highest stem
		dx, dy := (float64(x)-cx)/rx, (float64(y)-cy)/ry
		return y >= top-1 && dx*dx+dy*dy <= 1
	}
}

// Shrub leaves take the crown nearest the branches first, so foliage
// gathers around them and the crown's edge stays airy.
func (s *shrub) leafSpots() []pt {
	stems := s.p.cellsOf(Stem)
	inCrown := s.crown(stems)
	type spot struct {
		p   pt
		key float64
	}
	var in []spot
	for y := 0; y < ground-2; y++ {
		for x := 0; x < Width; x++ {
			if !s.p.free(x, y) || !inCrown(x, y) {
				continue
			}
			d := Width
			for _, q := range stems {
				d = min(d, max(abs(q.x-x), abs(q.y-y)))
			}
			in = append(in, spot{pt{x, y}, float64(d) + 0.9*hash01(s.p.Name, x+100, y)})
		}
	}
	sort.SliceStable(in, func(i, j int) bool { return in[i].key < in[j].key })
	out := make([]pt, len(in))
	for i, sp := range in {
		out[i] = sp.p
	}
	return out
}

func abs(v int) int { return max(v, -v) }

func (s *shrub) flowerSpots() []pt {
	var out []pt
	for _, t := range s.tips {
		for _, d := range dirs8 {
			if x, y := t.x+d.x, t.y+d.y; s.p.free(x, y) && y < ground {
				out = append(out, pt{x, y})
			}
		}
	}
	if len(out) == 0 {
		out = s.leafSpots()
	}
	return s.shuffled(dedup(out), 2)
}

// ---- cactus: a three-pixel trunk that grows tall, then sprouts two-pixel
// arms; spines stand around its flesh ----

type cactus struct {
	base
	top  int
	arms []tip // lean is the side (-1 left, 1 right)
}

func newCactus(b base) *cactus {
	c := &cactus{base: b, top: ground - 1}
	c.row(ground) // seedling: a stubby two-row trunk
	c.row(ground - 1)
	return c
}

func (c *cactus) row(y int) {
	for dx := -1; dx <= 1; dx++ {
		c.p.set(center+dx, y, Body)
	}
}

func (c *cactus) step(top int) {
	f := c.r.Float64()
	switch {
	case f < 0.55 && c.growBody(top):
	case f < 0.70 && c.sproutArm():
	case c.growArm():
	case c.growBody(top):
	}
}

func (c *cactus) growBody(top int) bool {
	y := c.top - 1
	if y < max(top, 1) {
		return false
	}
	for dx := -1; dx <= 1; dx++ {
		if !c.p.free(center+dx, y) {
			return false
		}
	}
	c.top = y
	c.row(y)
	return true
}

func (c *cactus) sproutArm() bool {
	if len(c.arms) >= 2 || ground-c.top < 9 {
		return false
	}
	side := []int{-1, 1}[c.r.Intn(2)]
	if len(c.arms) == 1 {
		side = -c.arms[0].lean
	}
	y := c.top + 3 + c.r.Intn(ground-c.top-7) // between top+3 and ground-5
	for dx := 2; dx <= 4; dx++ {
		if !c.p.free(center+dx*side, y) {
			return false
		}
	}
	for dx := 2; dx <= 4; dx++ {
		c.p.set(center+dx*side, y, Body)
	}
	c.arms = append(c.arms, tip{x: center + 3*side, y: y, lean: side})
	return true
}

// growArm raises an arm by a row. Arms are two pixels wide and stay below
// the trunk's top.
func (c *cactus) growArm() bool {
	for _, i := range c.r.Perm(len(c.arms)) {
		a := &c.arms[i]
		y := a.y - 1
		if y <= c.top+1 || !c.p.free(a.x, y) || !c.p.free(a.x+a.lean, y) {
			continue
		}
		a.y = y
		c.p.set(a.x, y, Body)
		c.p.set(a.x+a.lean, y, Body)
		return true
	}
	return false
}

// Spines stand beside the flesh, never above the crown of the trunk or an
// arm, where flowers go.
func (c *cactus) leafSpots() []pt {
	var out []pt
	for _, q := range c.near(1, Body) {
		if q.y > c.top {
			out = append(out, q)
		}
	}
	return c.shuffled(out, 1)
}

func (c *cactus) flowerSpots() []pt {
	var out []pt
	for dx := -1; dx <= 1; dx++ {
		if c.p.free(center+dx, c.top-1) {
			out = append(out, pt{center + dx, c.top - 1})
		}
	}
	for _, a := range c.arms {
		for _, x := range []int{a.x, a.x + a.lean} {
			if c.p.free(x, a.y-1) {
				out = append(out, pt{x, a.y - 1})
			}
		}
	}
	return c.shuffled(out, 2)
}

// ---- rosette: a low succulent that fills out from the middle, and sends
// up a flowering stalk ----

type rosette struct {
	base
	slots []pt // the rosette's cells, innermost first
	width int  // how many of them its base fills
	stalk int
}

// baseShare is how much of the full rosette its base fills at full size;
// the canopy's leaves go in the ring around it.
const baseShare = 0.6

func newRosette(b base) *rosette {
	type slot struct {
		p   pt
		key float64
	}
	var ss []slot
	for y := ground - 8; y <= ground; y++ {
		for x := 0; x < Width; x++ {
			dx, dy := float64(x-center)/10.5, float64(ground-y)/8.5
			if e := dx*dx + dy*dy; e <= 1 {
				ss = append(ss, slot{pt{x, y}, e + b.r.Float64()*0.2})
			}
		}
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].key < ss[j].key })
	g := &rosette{base: b}
	for _, s := range ss {
		g.slots = append(g.slots, s.p)
	}
	g.widen(3) // seedling
	return g
}

// widen fills the base out to its first n slots.
func (g *rosette) widen(n int) {
	for ; g.width < min(n, len(g.slots)); g.width++ {
		if s := g.slots[g.width]; g.p.free(s.x, s.y) {
			g.p.set(s.x, s.y, Leaf)
		}
	}
}

// step widens the base, and every fourth step or so raises the stalk from
// its middle.
func (g *rosette) step(top int) {
	full := int(baseShare * float64(len(g.slots)))
	g.widen(g.width + max(1, full/fullSteps[Rosette]))
	if g.r.Intn(4) == 0 && g.stalk < maxStalk {
		if y := ground - 1 - g.stalk; y >= max(top, 2) {
			g.p.set(center, y, Stem) // through the base's leaves
			g.stalk++
		}
	}
}

// maxStalk is how tall a rosette's stalk grows, from the ground.
const maxStalk = 16

// The rosette's canopy is the ring of slots around its base, innermost
// first: a busy rosette spreads wider.
func (g *rosette) leafSpots() []pt {
	var out []pt
	for _, s := range g.slots[g.width:] {
		if g.p.free(s.x, s.y) {
			out = append(out, s)
		}
	}
	return out
}

func (g *rosette) flowerSpots() []pt {
	var out []pt
	if g.stalk > 0 {
		for y := ground - g.stalk; y <= ground-g.stalk+2; y++ {
			for _, x := range []int{center - 1, center + 1} {
				if g.p.free(x, y) {
					out = append(out, pt{x, y})
				}
			}
		}
		if y := ground - 1 - g.stalk; g.p.free(center, y) {
			out = append(out, pt{center, y})
		}
	}
	if len(out) == 0 {
		out = g.leafSpots()
	}
	return g.shuffled(out, 2)
}

func dedup(ps []pt) []pt {
	seen := map[pt]bool{}
	var out []pt
	for _, q := range ps {
		if !seen[q] {
			seen[q] = true
			out = append(out, q)
		}
	}
	return out
}

// stepsFor is how many structural steps a skeleton takes at a push count:
// the same log curve as its height.
func stepsFor(sp Species, pushes int) int {
	return int(float64(fullSteps[sp]) * growthFrac(pushes))
}

// stepTop is the highest row the skeleton may reach at step k of n: the
// height limit rises with the steps taken, as it rises with pushes.
func stepTop(k, n int) int {
	return ground - (seedlingHeight + k*(Height-2-seedlingHeight)/max(n, 1))
}
