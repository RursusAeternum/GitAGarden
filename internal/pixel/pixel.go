// Package pixel is a small RGB framebuffer drawn in the terminal with
// half-block characters: each cell shows two vertically stacked pixels.
package pixel

import (
	"strconv"
	"strings"
)

type RGB struct{ R, G, B uint8 }

// Lerp mixes a toward b by t, clamped to [0, 1].
func Lerp(a, b RGB, t float64) RGB {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	m := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return RGB{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B)}
}

// Scale multiplies brightness by f, clamped to the valid range.
func (c RGB) Scale(f float64) RGB {
	s := func(v uint8) uint8 {
		x := float64(v)*f + 0.5
		if x > 255 {
			x = 255
		}
		if x < 0 {
			x = 0
		}
		return uint8(x)
	}
	return RGB{s(c.R), s(c.G), s(c.B)}
}

// Profile is the color depth used when encoding.
type Profile int

const (
	TrueColor Profile = iota
	ANSI256
)

type textCell struct {
	ch  rune
	fg  RGB
	set bool
}

// Canvas is W×H pixels. H is always even: terminal row r shows pixel rows
// 2r (top half) and 2r+1 (bottom half).
type Canvas struct {
	W, H int
	px   []RGB
	text []textCell // one per terminal cell, W × H/2
}

func New(w, h int) *Canvas {
	if h%2 == 1 {
		h++
	}
	return &Canvas{W: w, H: h, px: make([]RGB, w*h), text: make([]textCell, w*h/2)}
}

func (c *Canvas) in(x, y int) bool { return x >= 0 && y >= 0 && x < c.W && y < c.H }

func (c *Canvas) Set(x, y int, col RGB) {
	if c.in(x, y) {
		c.px[y*c.W+x] = col
	}
}

func (c *Canvas) At(x, y int) RGB {
	if !c.in(x, y) {
		return RGB{}
	}
	return c.px[y*c.W+x]
}

// Blend mixes col over the pixel at x,y with the given opacity.
func (c *Canvas) Blend(x, y int, col RGB, alpha float64) {
	if c.in(x, y) {
		i := y*c.W + x
		c.px[i] = Lerp(c.px[i], col, alpha)
	}
}

func (c *Canvas) Fill(col RGB) {
	for i := range c.px {
		c.px[i] = col
	}
}

func (c *Canvas) Rect(x, y, w, h int, col RGB) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			c.Set(i, j, col)
		}
	}
}

// Line draws from x0,y0 to x1,y1 inclusive (Bresenham).
func (c *Canvas) Line(x0, y0, x1, y1 int, col RGB) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		c.Set(x0, y0, col)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// Text writes s into terminal cells starting at column col of terminal row
// row, one rune per cell; callers pass only single-width runes. Each
// character keeps its cell's top pixel as background.
func (c *Canvas) Text(col, row int, s string, fg RGB) {
	for _, r := range s {
		if col >= 0 && col < c.W && row >= 0 && row < c.H/2 {
			c.text[row*c.W+col] = textCell{ch: r, fg: fg, set: true}
		}
		col++
	}
}

// Encode renders the canvas as terminal text, emitting a color code only
// when a cell's colors differ from the previous cell's.
func (c *Canvas) Encode(p Profile) string {
	var b strings.Builder
	b.Grow(c.W * c.H * 6)
	for row := 0; row < c.H/2; row++ {
		var fg, bg RGB
		have := false
		for x := 0; x < c.W; x++ {
			top, bot := c.px[2*row*c.W+x], c.px[(2*row+1)*c.W+x]
			ch, cfg, cbg := '▀', top, bot
			if t := c.text[row*c.W+x]; t.set {
				ch, cfg, cbg = t.ch, t.fg, top
			} else if top == bot {
				ch, cbg = ' ', top
				if have {
					cfg = fg // a space shows no foreground; keep the current one
				}
			}
			if !have || cfg != fg {
				writeColor(&b, p, 38, cfg)
				fg = cfg
			}
			if !have || cbg != bg {
				writeColor(&b, p, 48, cbg)
				bg = cbg
			}
			have = true
			b.WriteRune(ch)
		}
		b.WriteString("\x1b[0m")
		if row < c.H/2-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func writeColor(b *strings.Builder, p Profile, layer int, c RGB) {
	b.WriteString("\x1b[")
	b.WriteString(strconv.Itoa(layer))
	if p == ANSI256 {
		b.WriteString(";5;")
		b.WriteString(strconv.Itoa(to256(c)))
	} else {
		b.WriteString(";2;")
		b.WriteString(strconv.Itoa(int(c.R)))
		b.WriteByte(';')
		b.WriteString(strconv.Itoa(int(c.G)))
		b.WriteByte(';')
		b.WriteString(strconv.Itoa(int(c.B)))
	}
	b.WriteByte('m')
}

// to256 maps a color onto the 6×6×6 cube of the xterm 256-color palette.
func to256(c RGB) int {
	q := func(v uint8) int { return (int(v)*5 + 127) / 255 }
	return 16 + 36*q(c.R) + 6*q(c.G) + q(c.B)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
