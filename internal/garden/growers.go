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

// A grower turns events into additions on the grid. Each species has its
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

// sprout places a new cell of kind k in a free spot next to one of the
// anchors and says where. The ground row is left to the stem base and weeds.
func (b base) sprout(anchors []pt, k CellKind, upOnly bool) (pt, bool) {
	return b.sproutCell(anchors, b.p.cell(k), upOnly)
}

// sproutCell is sprout for a ready-made cell, so a moved bloom keeps its
// birth time.
func (b base) sproutCell(anchors []pt, cell Cell, upOnly bool) (pt, bool) {
	try := func(a, d pt) (pt, bool) {
		x, y := a.x+d.x, a.y+d.y
		if (upOnly && d.y > 0) || y >= ground || !b.p.free(x, y) {
			return pt{}, false
		}
		b.p.Grid[y][x] = cell
		return pt{x, y}, true
	}
	if len(anchors) == 0 {
		return pt{}, false
	}
	for i := 0; i < 24; i++ {
		if q, ok := try(anchors[b.r.Intn(len(anchors))], dirs8[b.r.Intn(len(dirs8))]); ok {
			return q, true
		}
	}
	for _, i := range b.r.Perm(len(anchors)) {
		for _, d := range dirs8 {
			if q, ok := try(anchors[i], d); ok {
				return q, true
			}
		}
	}
	return pt{}, false
}

// bloom places a new flower or fruit near the anchors.
func (b base) bloom(anchors []pt, k CellKind) { b.bloomCell(anchors, b.p.cell(k)) }

// bloomCell places cell near the anchors, falling back to anywhere on the
// plant, and finally to taking an existing leaf's place.
func (b base) bloomCell(anchors []pt, cell Cell) {
	if _, ok := b.sproutCell(anchors, cell, true); ok {
		return
	}
	if _, ok := b.sproutCell(b.p.cellsOf(Stem, Body, Leaf), cell, false); ok {
		return
	}
	if leaves := b.p.cellsOf(Leaf); len(leaves) > 0 {
		c := leaves[b.r.Intn(len(leaves))]
		b.p.Grid[c.y][c.x] = cell
	}
}

// thicken makes an existing leaf lusher. Used once there is no room to add a
// new one, so late-life pushes still visibly count.
func (b base) thicken() bool {
	leaves := b.p.cellsOf(Leaf)
	for _, i := range b.r.Perm(len(leaves)) {
		c := &b.p.Grid[leaves[i].y][leaves[i].x]
		if c.Level < maxLevel {
			c.Level++
			c.At = b.p.stamp // lusher is new growth too
			return true
		}
	}
	return false
}

// topY is the highest row the plant may currently grow into.
func (b base) topY() int { return max(1, ground-heightCap(b.p.Pushes)) }

// ---- shrub: branching woody stems with leaf clusters ----

type tip struct{ x, y, lean int }

type shrub struct {
	base
	tips []tip
}

func newShrub(b base) *shrub {
	// Seedling: a short stem with a leaf on each side.
	for y := ground; y > ground-3; y-- {
		b.p.set(center, y, Stem)
	}
	b.p.set(center-1, ground-2, Leaf)
	b.p.set(center+1, ground-2, Leaf)
	return &shrub{base: b, tips: []tip{{center, ground - 2, 0}}}
}

func (s *shrub) push() {
	if s.r.Float64() < 0.45 && s.grow() {
		return
	}
	anchors := s.p.cellsOf(Stem)
	if s.r.Float64() < 0.4 {
		anchors = s.p.cellsOf(Stem, Leaf)
	}
	if s.leafCluster(anchors) || s.grow() {
		return
	}
	s.thicken()
}

// leafCluster sprouts a leaf and, when there's room, a second one touching
// it, so foliage reads as leaves rather than single-pixel noise.
func (s *shrub) leafCluster(anchors []pt) bool {
	q, ok := s.sprout(anchors, Leaf, false)
	if ok {
		s.sprout([]pt{q}, Leaf, false)
	}
	return ok
}

// grow extends one branch tip upwards, sometimes forking. A tip held back
// only by the height cap stays alive for later; one boxed in is dropped.
func (s *shrub) grow() bool {
	top := s.topY()
	var dead []int
	for _, i := range s.r.Perm(len(s.tips)) {
		t := &s.tips[i]
		leans := []int{t.lean, -1, 0, 1}
		s.r.Shuffle(3, func(a, b int) { leans[a+1], leans[b+1] = leans[b+1], leans[a+1] })
		for _, dx := range leans {
			x, y := t.x+dx, t.y-1
			if y < top || !s.p.free(x, y) {
				continue
			}
			s.p.set(x, y, Stem)
			t.x, t.y = x, y
			if s.r.Float64() < 0.35 {
				t.lean = dx
			}
			if len(s.tips) < 7 && y < ground-4 && s.r.Float64() < 0.18 {
				s.tips = append(s.tips, tip{x, y, []int{-1, 1}[s.r.Intn(2)]})
			}
			return true
		}
		if t.y-1 >= top {
			dead = append(dead, i)
		}
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

func (s *shrub) merge()   { s.bloom(s.tipPts(), Flower) }
func (s *shrub) release() { s.bloom(s.p.cellsOf(Leaf), Fruit) }

// ---- cactus: a three-pixel trunk that grows tall, then sprouts two-pixel
// arms; spines fill in around it ----

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
		c.grow(center+dx, y)
	}
}

// open reports whether flesh may grow into x,y: empty, or a spine, flower
// or fruit that it pushes aside.
func (c *cactus) open(x, y int) bool {
	if !c.p.inBounds(x, y) {
		return false
	}
	k := c.p.Grid[y][x].Kind
	return k == Empty || k == Leaf || k == Flower || k == Fruit
}

// grow turns x,y into flesh. A flower or fruit there is carried up onto the
// new growth, keeping its birth time, so merges crown the cactus instead of
// capping it.
func (c *cactus) grow(x, y int) {
	old := c.p.Grid[y][x]
	c.p.set(x, y, Body)
	if old.Kind == Flower || old.Kind == Fruit {
		c.bloomCell([]pt{{x, y}}, old)
	}
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
	if _, ok := c.sprout(c.p.cellsOf(Body), Leaf, false); ok {
		return
	}
	if c.growBody() || c.growArm() {
		return
	}
	c.thicken()
}

func (c *cactus) growBody() bool {
	y := c.top - 1
	if y < c.topY() {
		return false
	}
	for dx := -1; dx <= 1; dx++ {
		if !c.open(center+dx, y) {
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
		if !c.open(center+dx*side, y) {
			return false
		}
	}
	for dx := 2; dx <= 4; dx++ {
		c.grow(center+dx*side, y)
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
		if y <= c.top+1 || !c.open(a.x, y) || !c.open(a.x+a.lean, y) {
			continue
		}
		a.y = y
		c.grow(a.x, y)
		c.grow(a.x+a.lean, y)
		return true
	}
	return false
}

func (c *cactus) merge() {
	anchors := []pt{{center, c.top}}
	for _, a := range c.arms {
		anchors = append(anchors, pt{a.x, a.y})
	}
	c.bloom(anchors, Flower)
}

func (c *cactus) release() { c.bloom(c.p.cellsOf(Body), Fruit) }

// ---- rosette: a low succulent that fills out from the middle, then sends
// up a flowering stalk as PRs land ----

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
	for i := 0; i < 3; i++ { // seedling
		g.addLeaf()
	}
	return g
}

// addLeaf fills the next free slot, innermost first.
func (g *rosette) addLeaf() bool {
	for g.next < len(g.slots) {
		s := g.slots[g.next]
		g.next++
		if g.p.free(s.x, s.y) {
			g.p.set(s.x, s.y, Leaf)
			return true
		}
	}
	return false
}

func (g *rosette) push() {
	// The spread grows with the log of pushes too, so busy rosettes fill out gradually.
	if limit := 3 + int(growthFrac(g.p.Pushes)*float64(len(g.slots)-3)); g.next < limit && g.addLeaf() {
		return
	}
	if !g.thicken() {
		g.growStalk(1)
	}
}

func (g *rosette) growStalk(n int) {
	for i := 0; i < n; i++ {
		y := ground - 9 - len(g.stalk)
		if y < 2 || !g.p.free(center, y) {
			return
		}
		g.p.set(center, y, Stem)
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
	g.bloom(anchors, Flower)
}

func (g *rosette) release() { g.bloom(g.p.cellsOf(Leaf), Fruit) }
