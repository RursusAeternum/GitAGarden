# Living Garden v0.2 "New look" Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace GAG's ASCII rendering with true-color pixel art (half-block characters) in a static scene of sky, ground, pots and labels. `gag garden` and `gag replay` both use it.

**Architecture:** There are two new packages. `internal/pixel` is a garden-agnostic RGB framebuffer that encodes to half-block terminal output. `internal/scene` draws the sky, ground, pots and cloches, and composes garden beds. `internal/garden` keeps its deterministic event-replay growth, gains a pixel `Paint` step, and moves from a character-sized grid (21×11) to a pixel grid (23×32) with log-scaled growth. The old ASCII renderer (`garden/render.go`) is deleted.

**Deviation from the spec (deliberate):** the spec describes plant structure as polylines and angled leaves. This plan keeps the existing grid growth model at pixel resolution instead: each grid cell is one pixel of a kind (stem, body, leaf, flower, fruit), and the painter colors it. It reuses the tested growers, keeps growth deterministic, and makes sway a simple per-row shift. If plants look too blocky after Task 5, vector structure can come in a later phase without changing `Paint`'s signature.

**Tech Stack:** Go 1.26, Bubble Tea (replay UI), termenv (color-profile detection), go-runewidth (label sanitizing), golang.org/x/term.

**Spec:** `docs/superpowers/specs/2026-09-27-living-garden-design.md`. This plan covers phase 1, "v0.2 New look". v0.3 (Alive), v0.4 (Signals) and v0.5 (Reactions) each get their own plan after the previous release ships, because they build on the API this plan produces.

## Global Constraints

- Module `github.com/RursusAeternum/GitAGarden`, `go 1.26.0`. Go is at `~/sdk/go/bin`: run `export PATH=$HOME/sdk/go/bin:$PATH` before any `go` command.
- No new third-party modules. termenv and go-runewidth are already in `go.sum` as indirect dependencies; `go mod tidy` promotes them to direct.
- Rendering: each terminal cell is `▀` with foreground = top pixel and background = bottom pixel. A cell whose two pixels match is a space with that background. Output is true color, with a 256-color fallback.
- Determinism: the same name, species and events always produce the same plant. The same inputs always produce a byte-identical frame.
- Performance: encoding a 240×65-cell frame (240×130 px) takes under 10 ms.
- After every task, `go vet ./... && go test ./...` passes and `gofmt -l .` prints nothing.
- Every commit message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Match the existing code style: short doc comments on exported identifiers, no comment noise, table-free tests in the existing style.

## Review Focus

1. **Window narrower than one bed** (a split tmux pane or a small window, `cols < BedCols`): the frame must be exactly the window's width so lines never wrap. Pinned by Task 4 `TestComposeNarrowWindowKeepsWidth`.
2. **Repo names with wide or odd runes, or very long names** (CJK, emoji, `day-dreamers-combat-prototype`): labels are truncated with `…`, and runes that aren't exactly one cell wide become `?`, so the row never shifts. Pinned by Task 4 `TestLabelSanitizesAndTruncates`.
3. **Timestamps arriving in UTC** (GitHub data, replay clocks): the sky must follow the viewer's local clock, not the timestamp's zone. Pinned by Task 2 `TestSkyUsesLocalTime`.
4. **Terminals that don't advertise true color** (tmux without `Tc`, some macOS setups): output must fall back to 256-color codes, and `GAG_COLOR=truecolor|256` must override detection. Pinned by Task 1 `TestEncodeANSI256` and Task 4 `TestColorProfileOverride`.
5. **Empty or brand-new repos** (0–1 commits, like a freshly created repo): they render as a proper seedling with status `untended` and never panic. Pinned by Task 3 `TestStatus` and Task 5 `TestTinyReposAreSeedlings`.

---

### Task 1: `internal/pixel` canvas and half-block encoder

**Files:**
- Create: `internal/pixel/pixel.go`
- Test: `internal/pixel/pixel_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type RGB struct{ R, G, B uint8 }`
  - `func Lerp(a, b RGB, t float64) RGB`
  - `func (c RGB) Scale(f float64) RGB`
  - `type Profile int` with `const TrueColor Profile = iota; ANSI256`
  - `type Canvas struct{ W, H int /* unexported fields */ }`
  - `func New(w, h int) *Canvas` (rounds `h` up to even)
  - `func (c *Canvas) Set(x, y int, col RGB)`
  - `func (c *Canvas) At(x, y int) RGB` (zero `RGB` when out of bounds)
  - `func (c *Canvas) Blend(x, y int, col RGB, alpha float64)`
  - `func (c *Canvas) Fill(col RGB)`
  - `func (c *Canvas) Rect(x, y, w, h int, col RGB)`
  - `func (c *Canvas) Line(x0, y0, x1, y1 int, col RGB)`
  - `func (c *Canvas) Text(col, row int, s string, fg RGB)` (terminal-cell coordinates; background = the cell's top pixel)
  - `func (c *Canvas) Encode(p Profile) string` (rows joined by `\n`, each ending in `\x1b[0m`)

- [ ] **Step 1: Write the failing tests**

`internal/pixel/pixel_test.go`:

```go
package pixel

import (
	"strings"
	"testing"
)

var (
	red   = RGB{255, 0, 0}
	blue  = RGB{0, 0, 255}
	green = RGB{0, 255, 0}
	white = RGB{255, 255, 255}
)

func TestNewRoundsHeightUpToEven(t *testing.T) {
	if c := New(3, 3); c.H != 4 {
		t.Fatalf("H = %d, want 4", c.H)
	}
}

func TestEncodeHalfBlock(t *testing.T) {
	c := New(1, 2)
	c.Set(0, 0, red)
	c.Set(0, 1, blue)
	want := "\x1b[38;2;255;0;0m\x1b[48;2;0;0;255m▀\x1b[0m"
	if got := c.Encode(TrueColor); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestEncodeEmitsColorsOnlyOnChange(t *testing.T) {
	c := New(3, 2)
	for x := 0; x < 3; x++ {
		c.Set(x, 0, red)
		c.Set(x, 1, blue)
	}
	out := c.Encode(TrueColor)
	if n := strings.Count(out, "\x1b[38;"); n != 1 {
		t.Errorf("fg codes = %d, want 1", n)
	}
	if n := strings.Count(out, "\x1b[48;"); n != 1 {
		t.Errorf("bg codes = %d, want 1", n)
	}
	if n := strings.Count(out, "▀"); n != 3 {
		t.Errorf("cells = %d, want 3", n)
	}
}

func TestEncodeUniformCellIsASpace(t *testing.T) {
	c := New(2, 2)
	c.Fill(green)
	out := c.Encode(TrueColor)
	if strings.Contains(out, "▀") {
		t.Errorf("uniform cells should be spaces: %q", out)
	}
	if n := strings.Count(out, "\x1b[48;"); n != 1 {
		t.Errorf("bg codes = %d, want 1", n)
	}
}

func TestEncodeRowsEndWithReset(t *testing.T) {
	out := New(2, 4).Encode(TrueColor)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	for _, l := range lines {
		if !strings.HasSuffix(l, "\x1b[0m") {
			t.Errorf("line not reset: %q", l)
		}
	}
}

func TestEncodeANSI256(t *testing.T) {
	c := New(1, 2)
	c.Set(0, 0, red)
	c.Set(0, 1, white)
	out := c.Encode(ANSI256)
	if !strings.Contains(out, "\x1b[38;5;196m") || !strings.Contains(out, "\x1b[48;5;231m") {
		t.Fatalf("got %q", out)
	}
	if strings.Contains(out, ";2;") {
		t.Errorf("true-color code in 256-color output: %q", out)
	}
}

func TestTextOverlay(t *testing.T) {
	bg := RGB{10, 20, 30}
	c := New(3, 2)
	c.Fill(bg)
	c.Text(0, 0, "hi", white)
	out := c.Encode(TrueColor)
	if !strings.Contains(out, "\x1b[38;2;255;255;255m") || !strings.Contains(out, "hi") {
		t.Fatalf("got %q", out)
	}
	if !strings.Contains(out, "\x1b[48;2;10;20;30m") {
		t.Errorf("text should keep the pixel background: %q", out)
	}
}

func TestTextClipsAtEdges(t *testing.T) {
	c := New(2, 2)
	c.Text(1, 0, "abc", red)
	c.Text(-1, 0, "z", red)
	c.Text(0, 5, "q", red)
	out := c.Encode(TrueColor)
	if strings.ContainsAny(out, "bczq") {
		t.Errorf("text outside the canvas leaked: %q", out)
	}
}

func TestOutOfBoundsIsIgnored(t *testing.T) {
	c := New(2, 2)
	c.Set(-1, 0, red)
	c.Set(0, 9, red)
	c.Blend(5, 5, red, 1)
	c.Rect(-3, -3, 10, 10, blue)
	if c.At(-1, 0) != (RGB{}) || c.At(0, 0) != blue {
		t.Errorf("out-of-bounds handling wrong: %v %v", c.At(-1, 0), c.At(0, 0))
	}
}

func TestLerpAndScale(t *testing.T) {
	if got := Lerp(RGB{}, RGB{200, 100, 50}, 0.5); got != (RGB{100, 50, 25}) {
		t.Errorf("Lerp = %v", got)
	}
	if got := (RGB{200, 100, 50}).Scale(2); got != (RGB{255, 200, 100}) {
		t.Errorf("Scale = %v", got)
	}
}

func TestLineHitsBothEnds(t *testing.T) {
	c := New(5, 6)
	c.Line(0, 0, 4, 5, red)
	if c.At(0, 0) != red || c.At(4, 5) != red {
		t.Error("line endpoints not drawn")
	}
}

func BenchmarkEncodeFullScreen(b *testing.B) {
	c := New(240, 130)
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			c.Set(x, y, RGB{uint8(x), uint8(y), uint8(x ^ y)})
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Encode(TrueColor)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/pixel/`
Expected: FAIL, build errors such as `undefined: New`.

- [ ] **Step 3: Implement**

`internal/pixel/pixel.go`:

```go
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
```

- [ ] **Step 4: Run the tests and the benchmark**

Run: `go test ./internal/pixel/ && go test ./internal/pixel/ -run '^$' -bench EncodeFullScreen -benchtime 20x`
Expected: PASS, and the benchmark reports well under `10000000 ns/op` (10 ms).

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/pixel
git commit -m "pixel: RGB canvas with half-block terminal encoder

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: `internal/scene` sky, ground, pot and cloche

**Files:**
- Create: `internal/scene/noise.go`, `internal/scene/sky.go`, `internal/scene/ground.go`, `internal/scene/cloche.go`
- Test: `internal/scene/scene_test.go`

**Interfaces:**
- Consumes (Task 1): `pixel.RGB`, `pixel.Lerp`, `pixel.New`, `(*Canvas).Set/At/Blend/Fill`, `RGB.Scale`.
- Produces:
  - `func SkyAt(t time.Time) (top, bottom pixel.RGB)` (uses `t.Local()`)
  - `func Darkness(t time.Time) float64` (0 = day, 1 = night)
  - `func DrawSky(c *pixel.Canvas, t time.Time, y0, y1 int, seed int64)` (fills pixel rows `[y0, y1)`)
  - `func DrawStars(c *pixel.Canvas, y0, y1 int, seed int64, darkness float64)`
  - `func DrawGround(c *pixel.Canvas, y0, y1 int, seed int64)`
  - `const PotW, PotH = 15, 8`
  - `func DrawPot(c *pixel.Canvas, cx, top int)`
  - `func DrawCloche(c *pixel.Canvas, cx, top, bottom, w int)`
  - Unexported helpers used by Task 4: `rgb(r, g, b uint8) pixel.RGB`, `noise(seed int64, x, y int) float64`, and the color `potRim`.

- [ ] **Step 1: Write the failing tests**

`internal/scene/scene_test.go`:

```go
package scene

import (
	"os"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func TestMain(m *testing.M) {
	time.Local = time.UTC // the sky follows the local clock; pin it for tests
	os.Exit(m.Run())
}

func at(h, min int) time.Time { return time.Date(2026, 6, 1, h, min, 0, 0, time.UTC) }

func painted(c *pixel.Canvas) int {
	n := 0
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if c.At(x, y) != (pixel.RGB{}) {
				n++
			}
		}
	}
	return n
}

func TestDarkness(t *testing.T) {
	if d := Darkness(at(23, 0)); d != 1 {
		t.Errorf("23:00 darkness = %v, want 1", d)
	}
	if d := Darkness(at(3, 0)); d != 1 {
		t.Errorf("03:00 darkness = %v, want 1", d)
	}
	if d := Darkness(at(12, 0)); d != 0 {
		t.Errorf("12:00 darkness = %v, want 0", d)
	}
	if d := Darkness(at(19, 45)); d <= 0 || d >= 1 {
		t.Errorf("19:45 darkness = %v, want dusk between 0 and 1", d)
	}
}

func TestSkyAtNoonIsDaylight(t *testing.T) {
	top, bot := SkyAt(at(12, 0))
	if top != rgb(80, 150, 230) || bot != rgb(175, 215, 245) {
		t.Errorf("noon sky = %v / %v", top, bot)
	}
}

func TestSkyUsesLocalTime(t *testing.T) {
	noon := at(12, 0)
	tokyo := noon.In(time.FixedZone("JST", 9*3600)) // same instant, different zone
	a, _ := SkyAt(noon)
	b, _ := SkyAt(tokyo)
	if a != b {
		t.Errorf("sky depends on the timestamp's zone: %v vs %v", a, b)
	}
}

func TestStarsOnlyAtNight(t *testing.T) {
	day, night := pixel.New(40, 20), pixel.New(40, 20)
	DrawStars(day, 0, 20, 1, 0)
	DrawStars(night, 0, 20, 1, 1)
	if n := painted(day); n != 0 {
		t.Errorf("%d stars in daylight", n)
	}
	if painted(night) == 0 {
		t.Error("no stars at night")
	}
}

func TestDrawSkyFillsOnlyItsBand(t *testing.T) {
	c := pixel.New(30, 20)
	DrawSky(c, at(12, 0), 4, 12, 1)
	for x := 0; x < c.W; x++ { // includes the sun's columns
		if c.At(x, 3) != (pixel.RGB{}) || c.At(x, 12) != (pixel.RGB{}) {
			t.Fatalf("sky or sun drawn outside its band at column %d", x)
		}
	}
	if c.At(0, 4) == (pixel.RGB{}) {
		t.Error("sky band left empty")
	}
}

func TestGroundAndPot(t *testing.T) {
	c := pixel.New(30, 20)
	DrawGround(c, 12, 20, 1)
	if p := c.At(5, 15); !(p.R > p.G && p.G > p.B) {
		t.Errorf("soil is not brown: %v", p)
	}
	DrawPot(c, 15, 4)
	if got := c.At(15, 5); got != potRim {
		t.Errorf("pot rim = %v, want %v", got, potRim)
	}
}

func TestClocheIsTranslucent(t *testing.T) {
	bg := rgb(100, 0, 0)
	c := pixel.New(30, 30)
	c.Fill(bg)
	DrawCloche(c, 15, 2, 28, 21)
	mid, edge := c.At(15, 20), c.At(5, 20)
	if mid == bg {
		t.Error("glass fill missing")
	}
	if mid.G >= edge.G {
		t.Errorf("edge (%v) should be more visible than the fill (%v)", edge, mid)
	}
	if c.At(0, 0) != bg {
		t.Error("cloche drawn outside its bounds")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/scene/`
Expected: FAIL, `undefined: Darkness` and similar.

- [ ] **Step 3: Implement**

`internal/scene/noise.go`:

```go
// Package scene draws the world around the plants: sky, ground, pots and
// glass, and lays plants out in garden beds.
package scene

import "github.com/RursusAeternum/GitAGarden/internal/pixel"

func rgb(r, g, b uint8) pixel.RGB { return pixel.RGB{R: r, G: g, B: b} }

// noise is a stable pseudo-random value in [0, 1) per seed and position,
// used for texture so frames don't flicker.
func noise(seed int64, x, y int) float64 {
	h := uint64(seed) ^ uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xC2B2AE3D27D4EB4F
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	return float64(h>>11) / float64(1<<53)
}
```

`internal/scene/sky.go`:

```go
package scene

import (
	"math"
	"math/rand"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

type skyKey struct {
	hour        float64
	top, bottom pixel.RGB
}

var (
	night    = skyKey{0, rgb(8, 10, 30), rgb(20, 24, 60)}
	day      = skyKey{0, rgb(80, 150, 230), rgb(175, 215, 245)}
	skyKeys  = []skyKey{
		{0, night.top, night.bottom},
		{5, night.top, night.bottom},
		{6.5, rgb(90, 110, 180), rgb(250, 170, 120)},
		{9, day.top, day.bottom},
		{17, day.top, day.bottom},
		{19, rgb(60, 60, 130), rgb(240, 130, 90)},
		{20.5, night.top, night.bottom},
		{24, night.top, night.bottom},
	}
	sunColor  = rgb(255, 230, 140)
	moonColor = rgb(230, 230, 215)
	starColor = rgb(255, 255, 230)
)

// hourOf is the fractional hour on the viewer's local clock.
func hourOf(t time.Time) float64 {
	t = t.Local()
	return float64(t.Hour()) + float64(t.Minute())/60
}

// SkyAt is the sky gradient's top and bottom color at t's local time.
func SkyAt(t time.Time) (top, bottom pixel.RGB) {
	h := hourOf(t)
	for i := 1; i < len(skyKeys); i++ {
		if h < skyKeys[i].hour {
			a, b := skyKeys[i-1], skyKeys[i]
			f := (h - a.hour) / (b.hour - a.hour)
			return pixel.Lerp(a.top, b.top, f), pixel.Lerp(a.bottom, b.bottom, f)
		}
	}
	k := skyKeys[len(skyKeys)-1]
	return k.top, k.bottom
}

// Darkness is 0 in daylight, 1 at night, and in between at dusk and dawn.
func Darkness(t time.Time) float64 {
	switch h := hourOf(t); {
	case h < 5 || h >= 20.5:
		return 1
	case h < 6.5:
		return (6.5 - h) / 1.5
	case h >= 19:
		return (h - 19) / 1.5
	}
	return 0
}

// DrawSky fills pixel rows [y0, y1) with the sky at time t, with stars and
// the sun or moon.
func DrawSky(c *pixel.Canvas, t time.Time, y0, y1 int, seed int64) {
	top, bot := SkyAt(t)
	span := float64(max(1, y1-y0-1))
	for y := y0; y < y1; y++ {
		col := pixel.Lerp(top, bot, float64(y-y0)/span)
		for x := 0; x < c.W; x++ {
			c.Set(x, y, col)
		}
	}
	DrawStars(c, y0, y1, seed, Darkness(t))
	drawSunMoon(c, t, y0, y1)
}

// DrawStars scatters seeded stars over the upper part of the band, as
// bright as the night is dark.
func DrawStars(c *pixel.Canvas, y0, y1 int, seed int64, darkness float64) {
	if darkness <= 0 || c.W == 0 || y1 <= y0 {
		return
	}
	r := rand.New(rand.NewSource(seed))
	band := max(1, (y1-y0)*2/3)
	for i := 0; i < c.W*(y1-y0)/40; i++ {
		x, y := r.Intn(c.W), y0+r.Intn(band)
		c.Blend(x, y, starColor, darkness*(0.4+0.6*r.Float64()))
	}
}

// drawSunMoon moves the sun across the band from 06:00 to 20:00, and the
// moon from 20:00 to 06:00, highest at the middle of its arc.
func drawSunMoon(c *pixel.Canvas, t time.Time, y0, y1 int) {
	h := hourOf(t)
	col, frac := sunColor, (h-6)/14
	if h < 6 || h >= 20 {
		if h < 6 {
			h += 24
		}
		col, frac = moonColor, (h-20)/10
	}
	x := int(frac * float64(c.W-1))
	y := y0 + int(float64(y1-y0)*(0.12+0.5*math.Pow(2*frac-1, 2)))
	for dy := -2; dy <= 2; dy++ {
		if y+dy < y0 || y+dy >= y1 {
			continue // stay inside this band; beds stack vertically
		}
		for dx := -2; dx <= 2; dx++ {
			if dx*dx+dy*dy <= 5 {
				c.Set(x+dx, y+dy, col)
			}
		}
	}
}
```

`internal/scene/ground.go`:

```go
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
```

`internal/scene/cloche.go`:

```go
package scene

import (
	"math"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

var (
	glass     = rgb(190, 230, 240)
	highlight = rgb(255, 255, 255)
)

// DrawCloche draws a glass bell jar w pixels wide over rows [top, bottom],
// centered on cx: a faint fill, brighter edges, and a highlight streak.
func DrawCloche(c *pixel.Canvas, cx, top, bottom, w int) {
	half := w / 2
	domeCY := top + half
	for y := top; y <= bottom; y++ {
		span := half
		if y < domeCY {
			dy := domeCY - y
			span = int(math.Sqrt(float64(half*half - dy*dy)))
		}
		for dx := -span; dx <= span; dx++ {
			alpha := 0.08
			if dx == -span || dx == span || y == top {
				alpha = 0.55
			}
			c.Blend(cx+dx, y, glass, alpha)
		}
	}
	for y := domeCY; y < bottom-2; y++ {
		c.Blend(cx-half+2, y, highlight, 0.35)
	}
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/scene/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/scene
git commit -m "scene: sky with day/night, ground, pots and glass cloche

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: `garden.Paint`, the pixel painter for plants

This task paints the **existing** plant grid (still 21×11 cells of `CellKind`) onto a pixel canvas. Task 5 later switches the grid to pixel resolution without changing `Paint`'s signature. `Health`, `hash01`, `ago` and `issues` move out of `render.go` so that `render.go` can be deleted in Task 4.

**Files:**
- Create: `internal/garden/health.go`, `internal/garden/status.go`, `internal/garden/paint.go`
- Modify: `internal/garden/render.go` (remove the moved functions and their imports)
- Test: `internal/garden/paint_test.go`

**Interfaces:**
- Consumes (Task 1): `pixel.Canvas`, `pixel.RGB`, `pixel.Lerp`, `RGB.Scale`, `(*Canvas).Set`.
- Produces:
  - `type Style struct{ Health float64; Sway float64 }` (`Sway` = horizontal offset of the plant's top row in pixels; 0 for static frames)
  - `func Paint(c *pixel.Canvas, p *Plant, baseX, baseY int, st Style)` (`baseX` = the plot's center column, `baseY` = the pixel row the plant's ground row lands on)
  - `func Status(p *Plant, now time.Time, finished bool) string` (`"finished"`, `"untended"`, or `"3d ago · 2 issues"`)
  - `func Health(p *Plant, now time.Time, decayDays float64) float64` (moved, unchanged)
  - Unexported `weedColor` (used by tests)

- [ ] **Step 1: Write the failing tests**

`internal/garden/paint_test.go`:

```go
package garden

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func pushes(n int) []Event {
	ev := make([]Event, n)
	for i := range ev {
		ev[i] = Event{Kind: Push, At: t0.Add(time.Duration(i) * time.Hour)}
	}
	return ev
}

const padX, padY = 10, 6

func paintAlone(p *Plant, st Style) *pixel.Canvas {
	c := pixel.New(Width+padX, Height+padY)
	Paint(c, p, (Width+padX)/2, Height+padY/2, st)
	return c
}

func TestPaintIsDeterministic(t *testing.T) {
	p := Grow("repo", Shrub, pushes(60))
	a := paintAlone(p, Style{Health: 1}).Encode(pixel.TrueColor)
	b := paintAlone(p, Style{Health: 1}).Encode(pixel.TrueColor)
	if a != b {
		t.Error("same plant painted differently")
	}
}

func greenPixels(c *pixel.Canvas) int {
	n := 0
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if px := c.At(x, y); int(px.G) > int(px.R)+30 && px.G > px.B {
				n++
			}
		}
	}
	return n
}

func TestHealthyLeavesAreGreenWiltedAreNot(t *testing.T) {
	p := Grow("repo", Shrub, pushes(80))
	healthy := greenPixels(paintAlone(p, Style{Health: 1}))
	wilted := greenPixels(paintAlone(p, Style{Health: 0}))
	if healthy == 0 {
		t.Fatal("healthy plant has no green")
	}
	if wilted >= healthy/4 {
		t.Errorf("wilted plant is still green: %d vs %d pixels", wilted, healthy)
	}
}

func TestWeedsForOpenIssues(t *testing.T) {
	events := append(pushes(5),
		Event{Kind: IssueOpened, At: t0.Add(10 * time.Hour)},
		Event{Kind: IssueOpened, At: t0.Add(11 * time.Hour)},
		Event{Kind: IssueOpened, At: t0.Add(12 * time.Hour)})
	c := paintAlone(Grow("repo", Shrub, events), Style{Health: 1})
	n, baseY := 0, Height+padY/2
	for x := 0; x < c.W; x++ {
		if c.At(x, baseY) == weedColor {
			n++
		}
	}
	if n != 3 {
		t.Errorf("weed pixels on the ground row = %d, want 3", n)
	}
}

func TestSwayBendsTopNotBase(t *testing.T) {
	p := Grow("repo", Shrub, pushes(120))
	still := paintAlone(p, Style{Health: 1})
	swayed := paintAlone(p, Style{Health: 1, Sway: 4})
	baseY := Height + padY/2
	for x := 0; x < still.W; x++ {
		if still.At(x, baseY) != swayed.At(x, baseY) {
			t.Fatal("sway moved the base of the plant")
		}
	}
	if still.Encode(pixel.TrueColor) == swayed.Encode(pixel.TrueColor) {
		t.Error("sway changed nothing")
	}
}

func TestStatus(t *testing.T) {
	now := t0.Add(72 * time.Hour)
	if s := Status(Grow("r", Shrub, nil), now, false); s != "untended" {
		t.Errorf("empty repo status = %q, want untended", s)
	}
	p := Grow("r", Shrub, append(pushes(1), Event{Kind: IssueOpened, At: t0.Add(time.Hour)}))
	if s := Status(p, now, false); s != "3d ago · 1 issue" {
		t.Errorf("status = %q", s)
	}
	if s := Status(p, now, true); s != "finished" {
		t.Errorf("finished status = %q", s)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/garden/`
Expected: FAIL, `undefined: Paint`, `undefined: Style`, `undefined: Status`.

- [ ] **Step 3: Move the shared helpers out of `render.go`**

Create `internal/garden/health.go` with `defaultDecayDays`, `Health` and `hash01`, copied **verbatim** from `render.go`:

```go
package garden

import (
	"fmt"
	"hash/fnv"
	"math"
	"time"
)

const defaultDecayDays = 45

// Health is 1 for a recently tended plant, falling to 0 after DecayDays of
// neglect (with a two-day grace period).
func Health(p *Plant, now time.Time, decayDays float64) float64 {
	if p.LastTended.IsZero() {
		return 1
	}
	if decayDays <= 0 {
		decayDays = defaultDecayDays
	}
	idle := now.Sub(p.LastTended).Hours()/24 - 2
	if idle <= 0 {
		return 1
	}
	return math.Max(0, 1-idle/decayDays)
}

// hash01 gives a stable pseudo-random value per cell, so the same leaves fall
// first every time rather than flickering between frames.
func hash01(name string, x, y int) float64 {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s:%d:%d", name, x, y)
	return float64(h.Sum32()) / math.MaxUint32
}
```

Create `internal/garden/status.go`, moving `ago` and `issues` verbatim and adding `Status`:

```go
package garden

import (
	"fmt"
	"time"
)

// Status is the short line under a plant: "3d ago · 2 issues", "finished"
// or "untended".
func Status(p *Plant, now time.Time, finished bool) string {
	switch {
	case finished:
		return "finished"
	case p.LastTended.IsZero():
		return "untended"
	}
	return ago(now.Sub(p.LastTended)) + " · " + issues(p.OpenIssues)
}

func ago(d time.Duration) string {
	switch {
	case d < time.Hour:
		return "just now"
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func issues(n int) string {
	switch n {
	case 0:
		return "no issues"
	case 1:
		return "1 issue"
	}
	return fmt.Sprintf("%d issues", n)
}
```

Then delete `defaultDecayDays`, `Health`, `hash01`, `ago` and `issues` from `internal/garden/render.go`, and change its import block to:

```go
import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)
```

Run: `go build ./...`
Expected: success (render.go still builds and uses the moved functions from the same package).

- [ ] **Step 4: Implement the painter**

`internal/garden/paint.go`:

```go
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
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/garden/`
Expected: PASS, including all existing tests.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/garden
git commit -m "garden: pixel painter with species palettes, wilting, weeds and sway

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Compose garden beds, switch `gag garden` and `gag replay`, delete the ASCII renderer

**Files:**
- Create: `internal/scene/compose.go`, `internal/scene/compose_test.go`, `internal/scene/testdata/garden.golden` (generated), `cmd/gag/color.go`, `cmd/gag/color_test.go`
- Modify: `cmd/gag/main.go`, `internal/replay/model.go`, `internal/garden/plant_test.go`, `go.mod`/`go.sum` (via `go mod tidy`)
- Delete: `internal/garden/render.go`

**Interfaces:**
- Consumes: Task 1 (`pixel.*`), Task 2 (`DrawSky`, `DrawGround`, `DrawPot`, `PotH`, `DrawCloche`, `rgb`), Task 3 (`garden.Paint`, `garden.Style`, `garden.Status`, `garden.Health`), and the existing `garden.Width`, `garden.Height`, `garden.Grow`, `garden.FakeHistory`.
- Produces:
  - `const BedCols, BedRows` (terminal cells per plot)
  - `type Plot struct{ Plant *garden.Plant; Style garden.Style; Finished bool; Name, Status string }`
  - `func PerRow(cols int) int`
  - `func Compose(cols int, plots []Plot, t time.Time, seed int64) *pixel.Canvas` (exactly `cols` wide, one bed per `PerRow(cols)` plots)
  - `cmd/gag`: `func colorProfile() pixel.Profile` and `func termWidth() int`
  - `replay.Config.Profile pixel.Profile`

- [ ] **Step 1: Write the failing tests**

`internal/scene/compose_test.go`:

```go
package scene

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden")

func demoPlots() []Plot {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mk := func(name string, sp garden.Species, n int, finished bool) Plot {
		p := garden.Grow(name, sp, garden.FakeHistory(name, n, t0))
		now := p.LastTended.Add(72 * time.Hour)
		h := garden.Health(p, now, 45)
		if finished {
			h = 1
		}
		return Plot{Plant: p, Style: garden.Style{Health: h}, Finished: finished,
			Name: name, Status: garden.Status(p, now, finished)}
	}
	return []Plot{
		mk("gag-core", garden.Shrub, 160, false),
		mk("rustyfs", garden.Cactus, 90, false),
		mk("old-blog", garden.Rosette, 70, true),
	}
}

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

func visible(s string) string { return ansi.ReplaceAllString(s, "") }

func TestComposeGolden(t *testing.T) {
	got := Compose(3*BedCols, demoPlots(), at(14, 0), 1).Encode(pixel.TrueColor)
	path := filepath.Join("testdata", "garden.golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (generate it with: go test ./internal/scene -update)", err)
	}
	if got != string(want) {
		t.Errorf("frame differs from %s; if the change is intended, run: go test ./internal/scene -update", path)
	}
}

func TestComposeLayout(t *testing.T) {
	if PerRow(3*BedCols) != 3 || PerRow(1) != 1 {
		t.Errorf("PerRow = %d, %d", PerRow(3*BedCols), PerRow(1))
	}
	if c := Compose(2*BedCols, demoPlots(), at(14, 0), 1); c.H != 2*BedRows*2 {
		t.Errorf("3 plots at 2 per row should make 2 beds (H=%d), got H=%d", 2*BedRows*2, c.H)
	}
	if c := Compose(2*BedCols, nil, at(14, 0), 1); c.H != BedRows*2 {
		t.Errorf("an empty garden should still draw one bed, got H=%d", c.H)
	}
}

func TestComposeNarrowWindowKeepsWidth(t *testing.T) {
	c := Compose(12, demoPlots()[:1], at(14, 0), 1)
	if c.W != 12 {
		t.Fatalf("W = %d, want 12", c.W)
	}
	for i, line := range strings.Split(c.Encode(pixel.TrueColor), "\n") {
		if n := utf8.RuneCountInString(visible(line)); n != 12 {
			t.Fatalf("line %d is %d cells wide, want 12", i, n)
		}
	}
}

func TestLabelSanitizesAndTruncates(t *testing.T) {
	c := pixel.New(BedCols, 2)
	label(c, BedCols/2, 0, "日本語-"+strings.Repeat("x", 40), rgb(255, 255, 255))
	line := visible(c.Encode(pixel.TrueColor))
	if n := utf8.RuneCountInString(line); n != BedCols {
		t.Fatalf("label row is %d cells, want %d: %q", n, BedCols, line)
	}
	if strings.ContainsAny(line, "日本語") {
		t.Error("wide runes should be replaced")
	}
	if !strings.Contains(line, "…") {
		t.Error("long label should end in …")
	}
}
```

`cmd/gag/color_test.go`:

```go
package main

import (
	"testing"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func TestColorProfileOverride(t *testing.T) {
	t.Setenv("GAG_COLOR", "truecolor")
	if colorProfile() != pixel.TrueColor {
		t.Error("GAG_COLOR=truecolor should force true color")
	}
	t.Setenv("GAG_COLOR", "256")
	if colorProfile() != pixel.ANSI256 {
		t.Error("GAG_COLOR=256 should force 256 colors")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/scene/ ./cmd/gag/`
Expected: FAIL, `undefined: Compose`, `undefined: colorProfile`.

- [ ] **Step 3: Implement `Compose`**

`internal/scene/compose.go`:

```go
package scene

import (
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// Bed geometry. A bed is one row of plots: sky headroom, the plants, their
// pots, and a strip of ground carrying name and status labels. Everything
// derives from the plant grid size, so it follows garden.Width/Height.
const (
	headroom   = 5                            // px of sky above the tallest plant
	plantBaseY = headroom + garden.Height - 1 // px row of a plant's ground row
	potTop     = plantBaseY + 1
	groundTop  = potTop + 5
	bedPx      = (groundTop + 7) / 2 * 2 // even, so a bed is whole terminal rows
	BedRows    = bedPx / 2               // terminal rows per bed
	BedCols    = garden.Width + 3        // terminal columns per plot
)

// Plot is one plant with what's needed to draw it.
type Plot struct {
	Plant        *garden.Plant
	Style        garden.Style
	Finished     bool
	Name, Status string
}

var (
	nameColor  = rgb(240, 232, 214)
	glassLabel = rgb(150, 215, 230)
	sadLabel   = rgb(200, 120, 90)
	happyLabel = rgb(150, 220, 130)
)

// PerRow is how many plots fit side by side in cols terminal columns.
func PerRow(cols int) int { return max(1, cols/BedCols) }

// Compose draws the plots in beds under the sky at time t. The canvas is
// exactly cols wide (plots that don't fit are clipped, never wrapped) and
// holds one bed per PerRow(cols) plots.
func Compose(cols int, plots []Plot, t time.Time, seed int64) *pixel.Canvas {
	cols = max(cols, 1)
	per := PerRow(cols)
	beds := max(1, (len(plots)+per-1)/per)
	c := pixel.New(cols, beds*bedPx)
	for b := 0; b < beds; b++ {
		oy := b * bedPx
		DrawSky(c, t, oy, oy+groundTop, seed+int64(b))
		DrawGround(c, oy+groundTop, oy+bedPx, seed)
		lo := b * per
		if lo >= len(plots) {
			continue
		}
		row := plots[lo:min(len(plots), lo+per)]
		left := (cols - len(row)*BedCols) / 2
		for i, pl := range row {
			cx := left + i*BedCols + BedCols/2
			DrawPot(c, cx, oy+potTop)
			garden.Paint(c, pl.Plant, cx, oy+plantBaseY, pl.Style)
			if pl.Finished {
				DrawCloche(c, cx, oy+2, oy+potTop+PotH-1, BedCols-3)
			}
			labelRow := oy/2 + BedRows - 2
			label(c, cx, labelRow, pl.Name, nameColor)
			label(c, cx, labelRow+1, pl.Status, statusColor(pl))
		}
	}
	return c
}

func statusColor(pl Plot) pixel.RGB {
	if pl.Finished {
		return glassLabel
	}
	return pixel.Lerp(sadLabel, happyLabel, pl.Style.Health)
}

// label centers s on column cx, truncated to fit a bed. Runes that don't
// take exactly one terminal cell become '?', so they can't shift the row.
func label(c *pixel.Canvas, cx, row int, s string, fg pixel.RGB) {
	var rs []rune
	for _, r := range s {
		if runewidth.RuneWidth(r) != 1 {
			r = '?'
		}
		rs = append(rs, r)
	}
	if limit := BedCols - 2; len(rs) > limit {
		rs = append(rs[:limit-1], '…')
	}
	c.Text(cx-len(rs)/2, row, string(rs), fg)
}
```

- [ ] **Step 4: Generate the golden frame and look at it**

Run: `go test ./internal/scene -run TestComposeGolden -update && go test ./internal/scene/ && cat internal/scene/testdata/garden.golden; echo`
Expected: PASS. The terminal shows three beds under a daytime sky: a shrub, a cactus and a rosette in terracotta pots, the third under a glass cloche, with the names and statuses on the soil. (The plants still look blocky here; Task 5 raises the resolution.)

- [ ] **Step 5: Add color-profile detection to `cmd/gag`**

`cmd/gag/color.go`:

```go
package main

import (
	"os"

	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// colorProfile uses true color when the terminal advertises it.
// GAG_COLOR=truecolor or 256 overrides detection, for terminals that
// support true color without saying so.
func colorProfile() pixel.Profile {
	switch os.Getenv("GAG_COLOR") {
	case "truecolor", "24bit":
		return pixel.TrueColor
	case "256":
		return pixel.ANSI256
	}
	if termenv.NewOutput(os.Stdout).EnvColorProfile() == termenv.TrueColor {
		return pixel.TrueColor
	}
	return pixel.ANSI256
}

func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 100
}
```

- [ ] **Step 6: Switch `gag garden` to `Compose`**

In `cmd/gag/main.go`:

1. In the import block, remove `"github.com/charmbracelet/lipgloss"` and `"golang.org/x/term"`, and add `"github.com/RursusAeternum/GitAGarden/internal/scene"`.
2. Delete the `layout` function entirely (`termWidth` now lives in `color.go`).
3. In `runGarden`, replace the whole `render := func() (string, error) { ... }` closure with:

```go
	prof := colorProfile()
	render := func() (string, error) {
		now := time.Now()
		at := now.Add(ahead) // the moment the garden is drawn at
		header := ""
		if ahead > 0 {
			header = fmt.Sprintf("simulating %s ahead: %s, nothing tended\n\n", *simulate, at.Format("2006-01-02"))
		}
		var plots []scene.Plot
		if *demo {
			plots = demoPlots(now, at, *decay)
		} else {
			repos, err := loadRepos(context.Background(), list, *user, *limit, *ttl)
			if err != nil {
				return "", err
			}
			for _, r := range repos {
				plots = append(plots, plotFor(garden.Grow(r.Name(), r.Species(), r.Events()), r.Name(), r.Finished(), at, *decay))
			}
		}
		return header + scene.Compose(termWidth(), plots, at, 1).Encode(prof) + "\n", nil
	}
```

4. Replace `demoCards` with `demoPlots` and add `plotFor`:

```go
// plotFor bundles a grown plant with its health and status at time at.
func plotFor(p *garden.Plant, name string, finished bool, at time.Time, decay float64) scene.Plot {
	h := garden.Health(p, at, decay)
	if finished {
		h = 1
	}
	return scene.Plot{Plant: p, Style: garden.Style{Health: h}, Finished: finished,
		Name: name, Status: garden.Status(p, at, finished)}
}

// demoPlots builds fake histories relative to now and draws them at at,
// which is later than now when simulating.
func demoPlots(now, at time.Time, decay float64) []scene.Plot {
	var plots []scene.Plot
	for _, r := range demo {
		// Shift the fake history so its last event lands idleDays ago.
		events := garden.FakeHistory(r.name, r.events, now)
		shift := now.Sub(events[len(events)-1].At) - time.Duration(r.idleDays)*24*time.Hour
		for i := range events {
			events[i].At = events[i].At.Add(shift)
		}
		p := garden.Grow(r.name, garden.SpeciesFor(r.lang), events)
		plots = append(plots, plotFor(p, r.name, r.finished, at, decay))
	}
	return plots
}
```

5. In `runReplay`, set the profile on the config: change `cfg := replay.Config{Name: *name, Species: sp, DecayDays: *decay, Finished: *finished}` to `cfg := replay.Config{Name: *name, Species: sp, DecayDays: *decay, Finished: *finished, Profile: colorProfile()}`.

- [ ] **Step 7: Switch `gag replay` to `Compose`**

In `internal/replay/model.go`:

1. Imports: add `"github.com/RursusAeternum/GitAGarden/internal/pixel"` and `"github.com/RursusAeternum/GitAGarden/internal/scene"`.
2. In `Config`, after `Finished bool`, add:

```go
	// Profile is the color depth for the plant drawing.
	Profile pixel.Profile
```

3. At the top of `View`, replace

```go
	events := m.cfg.Events[:m.applied]
	p := garden.Grow(m.cfg.Name, m.cfg.Species, events)
	opts := garden.RenderOpts{Now: m.clock, DecayDays: m.cfg.DecayDays, Finished: m.cfg.Finished}
	card := garden.Card(p, opts)
```

with

```go
	events := m.cfg.Events[:m.applied]
	p := garden.Grow(m.cfg.Name, m.cfg.Species, events)
	health := garden.Health(p, m.clock, m.cfg.DecayDays)
	if m.cfg.Finished {
		health = 1
	}
	plot := scene.Plot{Plant: p, Style: garden.Style{Health: health}, Finished: m.cfg.Finished,
		Name: m.cfg.Name, Status: garden.Status(p, m.clock, m.cfg.Finished)}
	card := scene.Compose(scene.BedCols, []scene.Plot{plot}, m.clock, 1).Encode(m.cfg.Profile)
```

4. Further down in `View`, delete the now-duplicate block:

```go
	health := garden.Health(p, m.clock, m.cfg.DecayDays)
	if m.cfg.Finished {
		health = 1
	}
```

- [ ] **Step 8: Delete the ASCII renderer and update its test**

```bash
git rm internal/garden/render.go
```

In `internal/garden/plant_test.go`, add `"github.com/RursusAeternum/GitAGarden/internal/pixel"` to the imports and replace `TestLongHistoriesDoNotPanic` with:

```go
func TestLongHistoriesDoNotPanic(t *testing.T) {
	for _, sp := range AllSpecies {
		for _, name := range []string{"a", "b", "c", "d", "e"} {
			p := Grow(name, sp, FakeHistory(name, 2000, t0))
			c := pixel.New(Width+4, Height+4)
			Paint(c, p, (Width+4)/2, Height, Style{Health: 0.2, Sway: 3})
		}
	}
}
```

- [ ] **Step 9: Tidy modules, run everything, look at it**

Run: `go mod tidy && gofmt -l . ; go vet ./... && go test ./... && go build -o gag ./cmd/gag && GAG_COLOR=truecolor ./gag garden -demo && GAG_COLOR=256 ./gag garden -demo | head -30`
Expected: all tests PASS. `go.mod` now lists `github.com/muesli/termenv` and `github.com/mattn/go-runewidth` as direct requirements, and `lipgloss` is still required (by `internal/replay`). The demo garden renders as pixel-art beds in both color modes.

Then run `./gag replay -events 120` in a real terminal, press `G` (jump to end) and `q`. Expected: the plant shows as pixel art next to the replay panel, and the panel doesn't wrap.

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "Render gardens as pixel-art beds; drop the ASCII renderer

gag garden and gag replay compose plants into scene beds (sky, soil,
terracotta pots, glass for finished repos) and encode them with the
half-block pixel canvas. GAG_COLOR=truecolor|256 overrides color
detection.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Pixel-resolution plants with log-scaled growth

This switches the plant grid from 21×11 character cells to 23×32 pixels. Growth scales with log(pushes), tiny repos become seedlings, the cactus trunk is 3 px wide, shrub leaves grow in clusters, and `Cell.Glyph` goes away (nothing reads it after Task 4). `Grow`, `Paint` and `Compose` keep their signatures; the bed geometry follows automatically because it derives from `garden.Height`.

**Files:**
- Modify: `internal/garden/plant.go`, `internal/garden/growers.go` (full rewrite shown below)
- Modify: `internal/scene/testdata/garden.golden` (regenerated)
- Test: `internal/garden/plant_test.go` (add tests)

**Interfaces:**
- Consumes: everything above.
- Produces (unchanged public API): `garden.Width = 23`, `garden.Height = 32`, `Grow`, `Paint`, `Species`, `SpeciesFor`. New unexported `growthFrac(pushes int) float64`, `heightCap(pushes int) int` and `seedlingHeight = 4`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/garden/plant_test.go`:

```go
func topRow(p *Plant) int {
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			if p.Grid[y][x].Kind != Empty {
				return y
			}
		}
	}
	return Height
}

func TestTinyReposAreSeedlings(t *testing.T) {
	for _, sp := range AllSpecies {
		for _, events := range [][]Event{nil, pushes(1)} {
			p := Grow("repo", sp, events)
			if n := len(p.cellsOf(Stem, Body, Leaf)); n < 3 {
				t.Errorf("%s with %d events: only %d cells, want a seedling", sp, len(events), n)
			}
			if h := ground - topRow(p); h > seedlingHeight+2 {
				t.Errorf("%s with %d events is %dpx tall, want a seedling", sp, len(events), h)
			}
		}
	}
}

func TestPlantsGrowWithTheLogOfPushes(t *testing.T) {
	for _, sp := range []Species{Shrub, Cactus} {
		small, big := Grow("repo", sp, pushes(8)), Grow("repo", sp, pushes(500))
		hs, hb := ground-topRow(small), ground-topRow(big)
		if hb <= hs {
			t.Errorf("%s: 500 pushes (%dpx) not taller than 8 (%dpx)", sp, hb, hs)
		}
		if hs > heightCap(8)+2 {
			t.Errorf("%s: 8 pushes reached %dpx, cap is %dpx", sp, hs, heightCap(8))
		}
	}
}

func TestCactusTrunkIsThreePixelsWide(t *testing.T) {
	p := Grow("repo", Cactus, pushes(40))
	for dx := -1; dx <= 1; dx++ {
		if k := p.Grid[ground][center+dx].Kind; k != Body {
			t.Errorf("trunk at dx=%d is kind %d, want Body", dx, k)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/garden/`
Expected: FAIL, `undefined: seedlingHeight` and `undefined: heightCap`.

- [ ] **Step 3: Update `plant.go`**

In `internal/garden/plant.go`:

1. Add `"math"` to the imports.
2. Replace the size constants with:

```go
// Size of the plant grid in pixels, excluding pot and scene.
const (
	Width  = 23
	Height = 32
	center = Width / 2
	ground = Height - 1 // bottom row of the grid, just above the pot soil
)
```

3. Replace `Cell` with:

```go
type Cell struct {
	Kind  CellKind
	Level uint8 // how lush a leaf is; grows once there is no room for new ones
}
```

4. Replace `set` with:

```go
func (p *Plant) set(x, y int, k CellKind) {
	p.Grid[y][x] = Cell{Kind: k}
}
```

5. Add after `seedOf`:

```go
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
```

- [ ] **Step 4: Rewrite `growers.go` at pixel resolution**

Replace the whole of `internal/garden/growers.go` with:

```go
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

// sprout places a cell in a free spot next to one of the anchors and says
// where. The ground row is left to the stem base and weeds.
func (b base) sprout(anchors []pt, k CellKind, upOnly bool) (pt, bool) {
	try := func(a, d pt) (pt, bool) {
		x, y := a.x+d.x, a.y+d.y
		if (upOnly && d.y > 0) || y >= ground || !b.p.free(x, y) {
			return pt{}, false
		}
		b.p.set(x, y, k)
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

// bloom places a flower or fruit near the anchors, falling back to anywhere on
// the plant, and finally to turning an existing leaf into it.
func (b base) bloom(anchors []pt, k CellKind) {
	if _, ok := b.sprout(anchors, k, true); ok {
		return
	}
	if _, ok := b.sprout(b.p.cellsOf(Stem, Body, Leaf), k, false); ok {
		return
	}
	if leaves := b.p.cellsOf(Leaf); len(leaves) > 0 {
		c := leaves[b.r.Intn(len(leaves))]
		b.p.set(c.x, c.y, k)
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
		c.p.set(center+dx, y, Body)
	}
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
		if y <= c.top+1 || !c.open(a.x, y) || !c.open(a.x+a.lean, y) {
			continue
		}
		a.y = y
		c.p.set(a.x, y, Body)
		c.p.set(a.x+a.lean, y, Body)
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
```

- [ ] **Step 5: Run the garden tests**

Run: `go test ./internal/garden/`
Expected: PASS, including the existing `TestGrowIsDeterministic`, `TestDifferentNamesGrowDifferently`, `TestEveryEarlyPushAddsSomething`, `TestHealthDecays`, `TestIssuesDoNotCountAsTending`, the paint tests and the three new tests.

- [ ] **Step 6: Regenerate the golden frame and review it by eye**

Run: `go test ./internal/scene -run TestComposeGolden -update && cat internal/scene/testdata/garden.golden; echo`
Expected: three clearly different plants at the new resolution. The shrub has branching stems with leaf clusters and flowers, the cactus has a 3-px trunk with arms and spines, and the rosette is a fan of leaves under glass. If something looks broken (plants cut off at the top, floating above the pots, overlapping the labels), fix it before continuing: the bed constants in `compose.go` derive from `garden.Height`, so pots and labels should already line up.

- [ ] **Step 7: Run everything and commit**

Run: `gofmt -l . ; go vet ./... && go test ./... && go build -o gag ./cmd/gag && GAG_COLOR=truecolor ./gag garden -demo`

```bash
git add -A
git commit -m "garden: pixel-resolution plants with log-scaled growth

23x32 px grid. Tiny repos are seedlings, growth scales with log(pushes),
cactus trunks are three pixels wide with two-pixel arms, and shrub leaves
grow in clusters.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Docs, real-world check, v0.2.0 release gate

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: the finished v0.2 build.
- Produces: an updated README and, **after the user approves**, the tag `v0.2.0`.

- [ ] **Step 1: Update the README**

In `README.md`, replace the intro paragraph under `# GAG — Git a Garden` with:

```markdown
A terminal garden of your git projects, drawn in true-color pixel art.
Each repo is a plant in a terracotta pot that grows a little with every
push and merge, gets weeds for open issues, wilts when neglected, and goes
under a glass cloche when it's finished, all under a sky that follows your
clock.
```

Then, directly after the `## Run` code block, add:

```markdown
GAG draws with half-block characters and true color. If your terminal
supports true color but GAG shows banded colors, force it with
`GAG_COLOR=truecolor gag`. Use `GAG_COLOR=256` to force 256 colors.
```

- [ ] **Step 2: Real-world check on the user's own garden**

Run: `go build -o gag ./cmd/gag && GAG_COLOR=truecolor ./gag garden -ttl 24h && GAG_COLOR=truecolor ./gag garden -simulate 30d -ttl 24h`
Expected: the user's own repos as pixel-art beds, and the `-simulate` run visibly wilted. Check narrow and wide windows by resizing the terminal and re-running: no wrapped lines.

- [ ] **Step 3: Commit and push**

```bash
gofmt -l . ; go vet ./... && go test ./...
git add README.md
git commit -m "docs: pixel-art garden and GAG_COLOR

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git push origin main
```

Wait for CI to pass (`gh run list -R RursusAeternum/GitAGarden --limit 1`).

- [ ] **Step 4: Release gate: ask the user before tagging**

Tagging publishes a public release. Show the user the result (a screenshot of their garden, or the real-world check output) and ask: "Tag v0.2.0?" Only after an explicit yes:

```bash
git tag -a v0.2.0 -m "v0.2.0: pixel-art garden"
git push origin v0.2.0
gh workflow run update-gag.yml -R RursusAeternum/homebrew-tap
```

---

## Later phases (separate plans)

These are written after v0.2 ships, against its real API. Their scope comes from the spec's Phasing section.

- **v0.3 Alive:** `internal/live` Bubble Tea app as plain `gag` (with `gag garden --once` keeping the static print), an animation clock (~8 fps, ~2 fps when idle), `Style.Sway` driven over time, moving sun and moon, drifting clouds, critters, layout with camera panning when plots overflow, background poller (`-refresh`, default 5m), and a ticker with neglect items and last-refresh state.
- **v0.4 Signals:** GitHub open PRs and `statusCheckRollup`; buds (max 5, drooping after 7 days); CI weather clouds; momentum shoots; flowers only for merges in the last 30 days with seed heads after; a snail when 3 or more issues were opened in 7 days; ticker attention ordering.
- **v0.5 Reactions:** snapshot diff → change events → animation queue (watering, bud burst plus butterfly, sparkle plus bees, storm roll-in and clearing with a rainbow, weed pop and pull), the detail card (`enter`/`esc`), and mouse selection.
