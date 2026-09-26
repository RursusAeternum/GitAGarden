package garden

import (
	"hash/fnv"
	"math/rand"
	"time"
)

// Canvas size of the plant itself, excluding pot and glass casing.
const (
	Width  = 21
	Height = 11
	center = Width / 2
	ground = Height - 1 // bottom row of the canvas, just above the pot rim
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
	Glyph rune
	Level uint8 // how lush a leaf is; grows once the canvas has no free room
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
}

// Grow builds a plant from scratch by replaying events in order. The same
// name, species and events always produce the same plant.
func Grow(name string, sp Species, events []Event) *Plant {
	r := rand.New(rand.NewSource(seedOf(name)))
	p := &Plant{Name: name, Species: sp}
	for x := 0; x < Width; x++ {
		if x < center-1 || x > center+1 {
			p.weedSlots = append(p.weedSlots, x)
		}
	}
	r.Shuffle(len(p.weedSlots), func(i, j int) {
		p.weedSlots[i], p.weedSlots[j] = p.weedSlots[j], p.weedSlots[i]
	})

	g := newGrower(sp, p, r)
	for _, e := range events {
		switch e.Kind {
		case Push:
			p.Pushes++
			g.push()
		case Merge:
			p.Merges++
			g.merge()
		case Release:
			p.Releases++
			g.release()
		case IssueOpened:
			p.OpenIssues++
		case IssueClosed:
			if p.OpenIssues > 0 {
				p.OpenIssues--
			}
		}
		if e.Kind.Tends() && e.At.After(p.LastTended) {
			p.LastTended = e.At
		}
	}
	return p
}

func (p *Plant) inBounds(x, y int) bool {
	return x >= 0 && x < Width && y >= 0 && y < Height
}

func (p *Plant) free(x, y int) bool {
	return p.inBounds(x, y) && p.Grid[y][x].Kind == Empty
}

func (p *Plant) set(x, y int, k CellKind, g rune) {
	p.Grid[y][x] = Cell{Kind: k, Glyph: g}
}

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
