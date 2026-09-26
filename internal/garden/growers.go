package garden

import (
	"math"
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

// A grower turns events into additions on the canvas. Each species has its
// own growth habit, but every push, merge and release adds something.
type grower interface {
	push()
	merge()
	release()
}

func newGrower(sp Species, p *Plant, r *rand.Rand) grower {
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

// sprout places a cell in a free spot next to one of the anchors. The ground
// row is left to the stem base and weeds.
func (b base) sprout(anchors []pt, k CellKind, g rune, upOnly bool) bool {
	ok := func(a, d pt) bool {
		x, y := a.x+d.x, a.y+d.y
		if (upOnly && d.y > 0) || y >= ground || !b.p.free(x, y) {
			return false
		}
		b.p.set(x, y, k, g)
		return true
	}
	if len(anchors) == 0 {
		return false
	}
	for try := 0; try < 24; try++ {
		if ok(anchors[b.r.Intn(len(anchors))], dirs8[b.r.Intn(len(dirs8))]) {
			return true
		}
	}
	for _, i := range b.r.Perm(len(anchors)) {
		for _, d := range dirs8 {
			if ok(anchors[i], d) {
				return true
			}
		}
	}
	return false
}

// bloom places a flower or fruit near the anchors, falling back to anywhere on
// the plant, and finally to turning an existing leaf into it.
func (b base) bloom(anchors []pt, k CellKind, g rune) {
	if b.sprout(anchors, k, g, true) || b.sprout(b.p.cellsOf(Stem, Body, Leaf), k, g, false) {
		return
	}
	leaves := b.p.cellsOf(Leaf)
	if len(leaves) > 0 {
		c := leaves[b.r.Intn(len(leaves))]
		b.p.set(c.x, c.y, k, g)
	}
}

// thicken makes an existing leaf lusher. Used once there is no room to add a
// new one, so late-life pushes still visibly count. glyphs maps level→glyph;
// nil keeps the leaf's glyph and lets the renderer show the level.
func (b base) thicken(glyphs []rune) bool {
	leaves := b.p.cellsOf(Leaf)
	for _, i := range b.r.Perm(len(leaves)) {
		c := &b.p.Grid[leaves[i].y][leaves[i].x]
		if c.Level < maxLevel {
			c.Level++
			if glyphs != nil {
				c.Glyph = glyphs[c.Level]
			}
			return true
		}
	}
	return false
}

// ---- shrub: branching woody stems with leafy clusters ----

type tip struct{ x, y, lean int }

type shrub struct {
	base
	tips []tip
}

var shrubLeaves = []rune{'\'', '&', '@'}

func newShrub(b base) *shrub {
	b.p.set(center, ground, Stem, '|')
	return &shrub{base: b, tips: []tip{{center, ground, 0}}}
}

func (s *shrub) push() {
	if s.r.Float64() < 0.4 && s.grow() {
		return
	}
	anchors := s.p.cellsOf(Stem)
	if s.r.Float64() < 0.35 {
		anchors = s.p.cellsOf(Stem, Leaf)
	}
	if s.sprout(anchors, Leaf, shrubLeaves[0], false) || s.grow() {
		return
	}
	s.thicken(shrubLeaves)
}

// grow extends one living branch tip upwards, sometimes forking.
func (s *shrub) grow() bool {
	var dead []int
	for _, i := range s.r.Perm(len(s.tips)) {
		t := &s.tips[i]
		leans := []int{t.lean, -1, 0, 1}
		s.r.Shuffle(3, func(a, b int) { leans[a+1], leans[b+1] = leans[b+1], leans[a+1] })
		for _, dx := range leans {
			x, y := t.x+dx, t.y-1
			if y < 1 || !s.p.free(x, y) {
				continue
			}
			s.p.set(x, y, Stem, map[int]rune{-1: '\\', 0: '|', 1: '/'}[dx])
			t.x, t.y = x, y
			if s.r.Float64() < 0.35 {
				t.lean = dx
			}
			if len(s.tips) < 6 && y < ground-1 && s.r.Float64() < 0.2 {
				s.tips = append(s.tips, tip{x, y, []int{-1, 1}[s.r.Intn(2)]})
			}
			return true
		}
		dead = append(dead, i)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(dead)))
	for _, i := range dead {
		s.tips = append(s.tips[:i], s.tips[i+1:]...)
	}
	return false
}

func (s *shrub) tipPts() []pt {
	var out []pt
	for _, t := range s.tips {
		out = append(out, pt{t.x, t.y})
	}
	if len(out) == 0 {
		out = s.p.cellsOf(Stem)
	}
	return out
}

func (s *shrub) merge()   { s.bloom(s.tipPts(), Flower, '✿') }
func (s *shrub) release() { s.bloom(s.p.cellsOf(Leaf), Fruit, '●') }

// ---- cactus: a column that grows tall, then sprouts arms; spines fill in ----

type cactus struct {
	base
	top, minTop int
	arms        []tip
}

var spines = []rune{'\'', '+', '*'}

func newCactus(b base) *cactus {
	b.p.set(center, ground, Body, '█')
	return &cactus{base: b, top: ground, minTop: 1 + b.r.Intn(3)}
}

// open reports whether flesh may grow into x,y; it pushes spines aside.
func (c *cactus) open(x, y int) bool {
	return c.p.inBounds(x, y) && (c.p.Grid[y][x].Kind == Empty || c.p.Grid[y][x].Kind == Leaf)
}

func (c *cactus) push() {
	f := c.r.Float64()
	switch {
	case f < 0.35 && c.growBody():
		return
	case f < 0.50 && c.sproutArm():
		return
	case f < 0.65 && c.growArm():
		return
	}
	if c.sprout(c.p.cellsOf(Body), Leaf, spines[0], false) || c.growBody() || c.growArm() {
		return
	}
	c.thicken(spines)
}

func (c *cactus) growBody() bool {
	if c.top <= c.minTop || !c.open(center, c.top-1) {
		return false
	}
	c.top--
	c.p.set(center, c.top, Body, '█')
	return true
}

func (c *cactus) sproutArm() bool {
	if len(c.arms) >= 2 || ground-c.top < 4 {
		return false
	}
	side := []int{-1, 1}[c.r.Intn(2)]
	if len(c.arms) == 1 {
		side = -(c.arms[0].x - center) / 2
	}
	y := c.top + 2 + c.r.Intn(ground-c.top-3)
	if !c.open(center+side, y) || !c.open(center+2*side, y) {
		return false
	}
	c.p.set(center+side, y, Body, '█')
	c.p.set(center+2*side, y, Body, '█')
	c.arms = append(c.arms, tip{center + 2*side, y, 0})
	return true
}

func (c *cactus) growArm() bool {
	for _, i := range c.r.Perm(len(c.arms)) {
		a := &c.arms[i]
		if a.y-1 > c.top && c.open(a.x, a.y-1) {
			a.y--
			c.p.set(a.x, a.y, Body, '█')
			return true
		}
	}
	return false
}

func (c *cactus) merge() {
	anchors := []pt{{center, c.top}}
	for _, a := range c.arms {
		anchors = append(anchors, pt{a.x, a.y})
	}
	c.bloom(anchors, Flower, '✿')
}

func (c *cactus) release() { c.bloom(c.p.cellsOf(Body), Fruit, '●') }

// ---- rosette: a low succulent that fills out ring by ring, then sends up a
// flowering stalk as PRs land ----

type rosette struct {
	base
	slots []pt
	next  int
	stalk []pt
}

func newRosette(b base) *rosette {
	type slot struct {
		p   pt
		key float64
	}
	var ss []slot
	for y := ground - 3; y <= ground; y++ {
		for x := 0; x < Width; x++ {
			dx, dy := float64(x-center)/8.5, float64(ground-y)/3.6
			if e := dx*dx + dy*dy; e <= 1 {
				ss = append(ss, slot{pt{x, y}, e + b.r.Float64()*0.25})
			}
		}
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].key < ss[j].key })
	r := &rosette{base: b}
	for _, s := range ss {
		r.slots = append(r.slots, s.p)
	}
	return r
}

func rosetteGlyph(x, y int) rune {
	dx, dy := x-center, ground-y
	switch {
	case dx == 0 && dy == 0:
		return 'V'
	case dx == 0:
		return '|'
	case dy == 0 && dx < 0:
		return '('
	case dy == 0:
		return ')'
	case math.Abs(float64(dx)) > 3*float64(dy):
		return '~'
	case dx < 0:
		return '\\'
	}
	return '/'
}

func (g *rosette) push() {
	for g.next < len(g.slots) {
		s := g.slots[g.next]
		g.next++
		if g.p.free(s.x, s.y) {
			g.p.set(s.x, s.y, Leaf, rosetteGlyph(s.x, s.y))
			return
		}
	}
	if !g.thicken(nil) {
		g.growStalk(1)
	}
}

func (g *rosette) growStalk(n int) {
	for i := 0; i < n; i++ {
		y := ground - 4 - len(g.stalk)
		if y < 2 || !g.p.free(center, y) {
			return
		}
		g.p.set(center, y, Stem, '|')
		g.stalk = append(g.stalk, pt{center, y})
	}
}

func (g *rosette) merge() {
	g.growStalk(2)
	anchors := g.stalk
	if len(anchors) > 3 {
		anchors = anchors[len(anchors)-3:]
	}
	if len(anchors) == 0 {
		anchors = g.p.cellsOf(Leaf)
	}
	g.bloom(anchors, Flower, '❀')
}

func (g *rosette) release() { g.bloom(g.p.cellsOf(Leaf), Fruit, '✦') }
