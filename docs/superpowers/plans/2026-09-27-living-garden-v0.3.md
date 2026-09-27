# Living Garden v0.3 "Alive" Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make plain `gag` an always-on, animated, fullscreen garden: swaying plants, drifting clouds, a sky that follows the clock, critters around healthy blooms, glints on glass, panning when the garden overflows, a ticker, and background refreshes. `gag garden --once` keeps the static print.

**Architecture:** `internal/scene` grows from "draw a static garden" into "draw one frame of a view". A `View` (window size, plots, wall clock, pan, motion flag) goes into `Draw`, which lays out beds bottom-anchored in exactly the window's cells. Clouds, glints and critters are pure functions of time. `Compose` becomes a thin wrapper, so the static print and replay keep working unchanged. A new `internal/live` package is the Bubble Tea app. It holds the repos in a stable order, runs an animation clock (~8 fps when busy, ~2 fps when idle), reloads data in the background through an injected `Load` function, and renders a one-line ticker under the garden. `cmd/gag` wires `gag`/`gag garden` to the live view when stdout is a terminal, and builds snapshots (with a demo fallback when there's no GitHub token).

**Tech Stack:** Go 1.26, Bubble Tea v1.3 (`tea.Tick`, `tea.WindowSizeMsg`, alt screen), lipgloss (ticker style, centering), go-runewidth (ticker width), golang.org/x/term (`IsTerminal`).

**Spec:** `docs/superpowers/specs/2026-09-27-living-garden-design.md`. This plan covers phase 2, "v0.3 Alive". It builds on v0.2 as shipped (tag `v0.2.0`). v0.4 (Signals) and v0.5 (Reactions) get their own plans.

**Rulings on points the spec leaves open (deliberate):**
- **Short windows crop from the top.** A bed is 24 rows tall, so a default 80×24 terminal (23 rows under the ticker) is one row short. Beds sit at the bottom of the window and anything that doesn't fit is cropped from the top: first sky, then plant tops. Nothing wraps or scrolls. Below 24×12 the view shows the "make the window a bit bigger" message the spec asks for.
- **`-watch` is removed.** The live view replaces it. `gag garden --once` (or piping the output) prints a static frame.
- **The cache TTL follows the refresh interval in the live view.** Unless `-ttl` is passed explicitly, the live view uses `ttl = refresh/2`, so each background refresh really fetches instead of hitting the 15-minute cache.
- **Glints on glass come in this phase.** The spec's scene section lists "periodic glint sweep" without assigning it a phase, and it's ambient motion.

## Global Constraints

- Module `github.com/RursusAeternum/GitAGarden`, `go 1.26.0`. Go is at `~/sdk/go/bin`: run `export PATH=$HOME/sdk/go/bin:$PATH` before any `go` command.
- No new third-party modules. Bubble Tea, lipgloss, go-runewidth, termenv and x/term are already direct dependencies.
- Frame rate: `fastFrame = 125ms` (~8 fps) while critters fly or the camera slides, `slowFrame = 500ms` (~2 fps) otherwise.
- Frame budget: rendering a 240×65-cell live frame (`View()`) must stay under 10 ms.
- Motion comes from the **wall clock** (plus the `-simulate` offset), never from data timestamps: sky, sun, clouds, sway, glints, critters, pan.
- `scene.Draw` is a pure function of its `View`: the same `View` gives a byte-identical frame.
- While the live view runs it must write nothing to stdout or stderr except through Bubble Tea's `View`. Pass a `nil` logger to `github.Sync` in live mode.
- Plants never reorder during a live session. The order is set by the first load; new repos are appended and vanished repos are dropped.
- After every task, `go vet ./... && go test ./...` passes and `gofmt -l .` prints nothing.
- Every commit message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Match the existing code style: short doc comments on exported identifiers, focused files. Small case lists in tests are fine.

## Review Focus

1. **Default 80×24 terminal** (23 rows under the ticker, one row short of a bed): plants and labels must stay visible, with only sky cropped, and nothing may wrap or scroll. Pinned by Task 2 `TestShortWindowKeepsThePlantsAndLabels` and Task 5 `TestViewFitsTheWindow` (which includes 80×24).
2. **Resizing the window mid-session**, including below 24×12 and back: every frame matches the new size exactly. The small size shows the message, and the garden returns when the window grows. Pinned by Task 5 `TestResizeFollowsTheWindow`.
3. **GitHub failing after a good load** (network drop, rate limit): the last garden stays on screen, the ticker says `refresh failed · data from …`, and refreshes continue on schedule. Pinned by Task 5 `TestFailedRefreshKeepsGarden`.
4. **Long sessions and laptop sleep** (elapsed time jumps by hours or days): pan stays in range, clouds and sway wrap, and nothing overflows or panics. Pinned by Task 2 `TestPanAtHoldsThenSlides` (the 1000-hour case) and Task 5 `TestLongSleepKeepsDrawing`.
5. **No GitHub token while live:** show the demo garden with a clear note instead of exiting with an error. Pinned by Task 6 `TestSnapshotFallsBackToDemoWithoutToken` and `TestNewClientWithoutTokenIsErrNoToken`.

---

### Task 1: Drifting clouds and glints on glass

**Files:**
- Create: `internal/scene/clouds.go`, `internal/scene/clouds_test.go`
- Modify: `internal/scene/cloche.go` (add `DrawGlint`), `internal/scene/compose.go` (static frames get clouds)
- Modify: `internal/scene/testdata/garden.golden` (regenerated)

**Interfaces:**
- Consumes (v0.2): `pixel.Canvas`, `(*Canvas).Blend/At`, `scene.Darkness`, the `highlight` color in `cloche.go`, and test helpers `at(h, min)` and `painted(c)` from `scene_test.go`.
- Produces:
  - `func DrawClouds(c *pixel.Canvas, t time.Time, y0, y1 int, seed int64)` (draws inside pixel rows `[y0, y1)` only; no-op for bands under 8 px)
  - `func DrawGlint(c *pixel.Canvas, t time.Time, cx, top, bottom, w int, seed int64)` (same geometry as `DrawCloche`; draws for 1.5 s out of every 20 s)

- [ ] **Step 1: Write the failing tests**

`internal/scene/clouds_test.go`:

```go
package scene

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func cloudFrame(at time.Time, seed int64) *pixel.Canvas {
	c := pixel.New(60, 24)
	DrawClouds(c, at, 0, 24, seed)
	return c
}

func TestCloudsAreDeterministicAndDrift(t *testing.T) {
	noon := at(12, 0)
	a, b := cloudFrame(noon, 1), cloudFrame(noon, 1)
	if a.Encode(pixel.TrueColor) != b.Encode(pixel.TrueColor) {
		t.Error("the same moment drew different clouds")
	}
	if painted(a) == 0 {
		t.Fatal("no clouds drawn")
	}
	later := cloudFrame(noon.Add(20*time.Second), 1)
	if a.Encode(pixel.TrueColor) == later.Encode(pixel.TrueColor) {
		t.Error("clouds did not drift in 20 seconds")
	}
}

func TestCloudsStayInTheirBand(t *testing.T) {
	c := pixel.New(80, 40)
	for s := 0; s < 20; s++ {
		DrawClouds(c, at(12, 0).Add(time.Duration(s)*time.Minute), 10, 30, int64(s))
	}
	for x := 0; x < c.W; x++ {
		for _, y := range []int{9, 30} {
			if c.At(x, y) != (pixel.RGB{}) {
				t.Fatalf("cloud pixel outside its band at %d,%d", x, y)
			}
		}
	}
}

func TestCloudsFadeAtNight(t *testing.T) {
	brightest := func(c *pixel.Canvas) uint8 {
		var m uint8
		for y := 0; y < c.H; y++ {
			for x := 0; x < c.W; x++ {
				m = max(m, c.At(x, y).R)
			}
		}
		return m
	}
	// Day clouds blend at 0.85, night ones at 0.25; two separate clouds may
	// overlap, so allow for a double blend at night.
	day, night := brightest(cloudFrame(at(12, 0), 3)), brightest(cloudFrame(at(0, 0), 3))
	if night == 0 || int(night)*4 >= int(day)*3 {
		t.Errorf("night clouds (%d) should be much fainter than day clouds (%d)", night, day)
	}
}

func TestGlintSweepsNowAndThen(t *testing.T) {
	base := time.UnixMilli(20_000 * 1000) // a whole number of 20 s glint periods
	glint := func(at time.Time) int {
		c := pixel.New(30, 30)
		DrawGlint(c, at, 15, 2, 28, 21, 0)
		return painted(c)
	}
	if glint(base.Add(700*time.Millisecond)) == 0 {
		t.Error("no glint during the sweep")
	}
	if n := glint(base.Add(5 * time.Second)); n != 0 {
		t.Errorf("glint drawn outside the sweep: %d px", n)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/scene/`
Expected: FAIL, `undefined: DrawClouds` and `undefined: DrawGlint`.

- [ ] **Step 3: Implement clouds**

`internal/scene/clouds.go`:

```go
package scene

import (
	"math/rand"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

var cloudColor = rgb(250, 250, 255)

// cloudDrift is how long a cloud takes to move one pixel to the right.
const cloudDrift = 4 * time.Second

// DrawClouds draws seeded, puffy clouds in the upper part of pixel rows
// [y0, y1). They drift right with the wall clock and wrap around; at night
// they fade so the stars show through. Bands under 8 px get no clouds.
func DrawClouds(c *pixel.Canvas, t time.Time, y0, y1 int, seed int64) {
	if c.W == 0 || y1-y0 < 8 {
		return
	}
	r := rand.New(rand.NewSource(seed ^ 0x5eed))
	shift := int(t.Unix() / int64(cloudDrift/time.Second))
	alpha := 0.85 - 0.6*Darkness(t)
	band := (y1 - y0) / 2
	for i := 0; i < max(1, c.W/28); i++ {
		w := 8 + r.Intn(9)
		span := c.W + w
		x0 := (r.Intn(span)+shift)%span - w
		y := y0 + 3 + r.Intn(band) // bumps reach 2 px above y, the base 1 px below
		drawCloud(c, x0, y, w, r, alpha)
	}
}

// drawCloud is a two-row base with rounded bumps along its top. The shape
// is collected first so overlapping puffs blend each pixel only once.
func drawCloud(c *pixel.Canvas, x0, y, w int, r *rand.Rand, alpha float64) {
	shape := map[[2]int]bool{}
	for dy := 0; dy < 2; dy++ {
		for x := x0; x < x0+w; x++ {
			shape[[2]int{x, y + dy}] = true
		}
	}
	for x := x0 + 1; x < x0+w-1; x += 3 + r.Intn(2) {
		rad := 1 + r.Intn(2)
		for dy := -rad; dy < 0; dy++ {
			for dx := -rad; dx <= rad; dx++ {
				if dx*dx+dy*dy <= rad*rad {
					shape[[2]int{x + dx, y + dy}] = true
				}
			}
		}
	}
	for p := range shape {
		c.Blend(p[0], p[1], cloudColor, alpha)
	}
}
```

- [ ] **Step 4: Implement the glint**

In `internal/scene/cloche.go`, add `"time"` to the imports and append:

```go
// A streak of light sweeps across each cloche for glintSweep out of every
// glintEvery.
const glintEvery, glintSweep = 20 * time.Second, 1500 * time.Millisecond

// DrawGlint draws the moving streak of light across a cloche with the same
// geometry as DrawCloche. seed staggers cloches so they don't flash in
// unison.
func DrawGlint(c *pixel.Canvas, t time.Time, cx, top, bottom, w int, seed int64) {
	period := glintEvery.Milliseconds()
	offset := (seed%period + period) % period
	ms := (t.UnixMilli() + offset) % period
	if ms >= glintSweep.Milliseconds() {
		return
	}
	half := w / 2
	x := cx - half + int(float64(w)*float64(ms)/float64(glintSweep.Milliseconds()))
	for y := top + half/2; y < bottom-2; y++ {
		c.Blend(x, y, highlight, 0.45)
		c.Blend(x+1, y, highlight, 0.2)
	}
}
```

- [ ] **Step 5: Give static frames clouds**

In `internal/scene/compose.go`, inside `Compose`, change

```go
		if b == 0 {
			drawSunMoon(c, t, oy, oy+groundTop) // one sun for the whole garden
		}
		DrawGround(c, oy+groundTop, oy+bedPx, seed)
```

to

```go
		if b == 0 {
			drawSunMoon(c, t, oy, oy+groundTop) // one sun for the whole garden
		}
		DrawClouds(c, t, oy, oy+groundTop, seed+int64(b))
		DrawGround(c, oy+groundTop, oy+bedPx, seed)
```

- [ ] **Step 6: Run the tests, regenerate the golden frame, look at it**

Run: `go test ./internal/scene/ -run 'Cloud|Glint' && go test ./internal/scene -run TestComposeGolden -update && go test ./... && cat internal/scene/testdata/garden.golden; echo`
Expected: the new tests pass. The golden frame now shows a few white clouds in the daytime sky, with the plants, pots and labels unchanged.

- [ ] **Step 7: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/scene
git commit -m "scene: drifting clouds and glints on glass

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Draw frames for a window: layout, bottom anchoring, panning

**Files:**
- Create: `internal/scene/draw.go`, `internal/scene/draw_test.go`
- Modify: `internal/scene/compose.go` (rewritten: `Compose` becomes a wrapper around `Draw`)

**Interfaces:**
- Consumes: Task 1 (`DrawClouds`, `DrawGlint`) and v0.2 (`DrawSky`, `drawSunMoon`, `DrawGround`, `DrawPot`, `PotH`, `DrawCloche`, `garden.Paint`, bed constants, `label`, `statusColor`, `Plot`, `PerRow`), plus test helpers `demoPlots()`, `visible()` and `at()`.
- Produces:
  - `type View struct { Cols, Rows int; Plots []Plot; Now time.Time; Pan float64; Seed int64; Motion bool }`
  - `type Layout struct { PerRow, Beds, Columns int; Overflow bool }`
  - `func LayoutFor(cols, rows, n int) Layout` (`rows <= 0` means unlimited height)
  - `func Draw(v View) *pixel.Canvas` (with `Rows > 0`: exactly `Cols × 2·Rows` pixels, beds bottom-anchored and cropped from the top)
  - `func PanAt(elapsed time.Duration, columns int) float64`
  - `func Sliding(elapsed time.Duration) bool`
  - `Compose(cols, plots, t, seed)` keeps its signature and output.

- [ ] **Step 1: Write the failing tests**

`internal/scene/draw_test.go`:

```go
package scene

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// manyPlots repeats the demo plots under distinct names.
func manyPlots(n int) []Plot {
	base := demoPlots()
	out := make([]Plot, n)
	for i := range out {
		p := base[i%len(base)]
		p.Name = fmt.Sprintf("%s-%d", p.Name, i)
		out[i] = p
	}
	return out
}

func TestLayoutFor(t *testing.T) {
	cases := []struct {
		cols, rows, n int
		want          Layout
	}{
		{3 * BedCols, 0, 8, Layout{PerRow: 3, Beds: 3, Columns: 3}},
		{3 * BedCols, 2*BedRows + 5, 5, Layout{PerRow: 3, Beds: 2, Columns: 3}},
		{3 * BedCols, BedRows - 1, 8, Layout{PerRow: 3, Beds: 1, Columns: 8, Overflow: true}},
		{3 * BedCols, 2 * BedRows, 8, Layout{PerRow: 3, Beds: 2, Columns: 4, Overflow: true}},
	}
	for _, c := range cases {
		if got := LayoutFor(c.cols, c.rows, c.n); got != c.want {
			t.Errorf("LayoutFor(%d, %d, %d) = %+v, want %+v", c.cols, c.rows, c.n, got, c.want)
		}
	}
}

func TestDrawFillsTheWindowExactly(t *testing.T) {
	for _, size := range [][2]int{{80, 23}, {30, 12}, {200, 60}, {BedCols, BedRows}} {
		cols, rows := size[0], size[1]
		v := View{Cols: cols, Rows: rows, Plots: manyPlots(8), Now: at(12, 0), Seed: 1, Motion: true}
		lines := strings.Split(Draw(v).Encode(pixel.TrueColor), "\n")
		if len(lines) != rows {
			t.Errorf("%dx%d: %d lines", cols, rows, len(lines))
			continue
		}
		for i, l := range lines {
			if n := utf8.RuneCountInString(visible(l)); n != cols {
				t.Fatalf("%dx%d: line %d is %d cells wide", cols, rows, i, n)
			}
		}
	}
}

func TestShortWindowKeepsThePlantsAndLabels(t *testing.T) {
	// A default 80x24 terminal leaves 23 rows under the ticker, one short of
	// a bed: only sky should be cropped.
	plots := manyPlots(1)
	out := Draw(View{Cols: 80, Rows: BedRows - 1, Plots: plots, Now: at(12, 0), Seed: 1}).Encode(pixel.TrueColor)
	lines := strings.Split(out, "\n")
	if l := visible(lines[len(lines)-2]); !strings.Contains(l, plots[0].Name) {
		t.Errorf("name label missing from the second-to-last row: %q", l)
	}
}

func TestSpareHeightIsSky(t *testing.T) {
	v := View{Cols: BedCols, Rows: BedRows + 10, Plots: manyPlots(1), Now: at(12, 0), Seed: 1}
	c := Draw(v)
	top, _ := SkyAt(at(12, 0))
	if c.At(0, 0) != top {
		t.Errorf("top-left pixel = %v, want the sky's top color %v", c.At(0, 0), top)
	}
}

func TestPanAtHoldsThenSlides(t *testing.T) {
	if p := PanAt(29*time.Second, 5); p != 0 {
		t.Errorf("before the first slide: %v", p)
	}
	if p := PanAt(30*time.Second+600*time.Millisecond, 5); p <= 0 || p >= 1 {
		t.Errorf("mid-slide: %v, want between 0 and 1", p)
	}
	if p := PanAt(32*time.Second, 5); p != 1 {
		t.Errorf("after the first slide: %v", p)
	}
	if p := PanAt(5*30*time.Second+2*time.Second, 5); p != 0 {
		t.Errorf("should wrap after 5 columns: %v", p)
	}
	if p := PanAt(1000*time.Hour+7*time.Second, 5); p < 0 || p >= 5 {
		t.Errorf("long session: pan %v out of range", p)
	}
	if PanAt(time.Hour, 1) != 0 {
		t.Error("nothing to pan with one column")
	}
	if !Sliding(30*time.Second+500*time.Millisecond) || Sliding(10*time.Second) || Sliding(500*time.Millisecond) {
		t.Error("Sliding should be true only during a slide")
	}
}

func TestPanningSlidesTheGarden(t *testing.T) {
	v := View{Cols: 3 * BedCols, Rows: BedRows, Plots: manyPlots(8), Now: at(12, 0), Seed: 1}
	frame := func(pan float64) string {
		v.Pan = pan
		return Draw(v).Encode(pixel.TrueColor)
	}
	if frame(0) == frame(1) {
		t.Error("pan did not move the garden")
	}
	if frame(0) != frame(8) {
		t.Error("panning a full circle should come back to the start")
	}
	if half := frame(0.5); half == frame(0) || half == frame(1) {
		t.Error("half a pan should sit in between")
	}
}

func TestGlassGlintsOnlyInLiveFrames(t *testing.T) {
	var glass []Plot
	for _, p := range demoPlots() {
		if p.Finished {
			glass = append(glass, p)
		}
	}
	base := time.UnixMilli(20_000 * 1000)
	for i := 0; i < 40; i++ { // sample a whole 20 s glint period
		now := base.Add(time.Duration(i) * 500 * time.Millisecond)
		still := Draw(View{Cols: BedCols, Plots: glass, Now: now, Seed: 1}).Encode(pixel.TrueColor)
		live := Draw(View{Cols: BedCols, Plots: glass, Now: now, Seed: 1, Motion: true}).Encode(pixel.TrueColor)
		if still != live {
			return
		}
	}
	t.Error("no glint in any live frame across a glint period")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/scene/`
Expected: FAIL, `undefined: View`, `undefined: LayoutFor`, `undefined: Draw`, `undefined: PanAt`.

- [ ] **Step 3: Implement `Draw`**

`internal/scene/draw.go`:

```go
package scene

import (
	"math"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// View describes one frame of the garden.
type View struct {
	Cols, Rows int       // terminal cells to fill; Rows 0 means as tall as the beds need
	Plots      []Plot
	Now        time.Time // wall clock: sky, sun, clouds, glints
	Pan        float64   // camera offset in plot columns when the garden overflows
	Seed       int64
	Motion     bool // a live frame: glints on glass
}

// Layout is how plots are arranged in a frame.
type Layout struct {
	PerRow   int  // plot columns visible side by side
	Beds     int  // bed rows on screen
	Columns  int  // plot columns in the whole garden; more than PerRow when it pans
	Overflow bool // the garden is wider than the window and pans
}

// LayoutFor fits n plots into cols×rows terminal cells. Beds stack while the
// height allows; beyond that plots go into columns that pan. rows <= 0 means
// unlimited height (static prints), so nothing pans.
func LayoutFor(cols, rows, n int) Layout {
	per := PerRow(cols)
	need := max(1, (n+per-1)/per)
	fit := need
	if rows > 0 {
		fit = max(1, rows/BedRows)
	}
	if need <= fit {
		return Layout{PerRow: per, Beds: need, Columns: per}
	}
	return Layout{PerRow: per, Beds: fit, Columns: (n + fit - 1) / fit, Overflow: true}
}

// Draw renders a frame. With Rows set the canvas is exactly Cols×Rows
// cells: beds sit at the bottom, spare height above them is sky, and a
// window shorter than the beds crops them from the top.
func Draw(v View) *pixel.Canvas {
	cols := max(v.Cols, 1)
	lay := LayoutFor(cols, v.Rows, len(v.Plots))
	h := lay.Beds * bedPx
	if v.Rows > 0 {
		h = v.Rows * 2
	}
	c := pixel.New(cols, h)
	top := h - lay.Beds*bedPx // negative when the top is cropped
	for b := 0; b < lay.Beds; b++ {
		oy := top + b*bedPx
		skyTop := oy
		if b == 0 && oy > 0 {
			skyTop = 0
		}
		DrawSky(c, v.Now, skyTop, oy+groundTop, v.Seed+int64(b))
		if b == 0 {
			drawSunMoon(c, v.Now, skyTop, oy+groundTop) // one sun for the whole garden
		}
		DrawClouds(c, v.Now, skyTop, oy+groundTop, v.Seed+int64(b))
		DrawGround(c, oy+groundTop, oy+bedPx, v.Seed)
		for _, s := range slots(lay, b, len(v.Plots), cols, v.Pan) {
			drawPlot(c, v, v.Plots[s.index], s.cx, oy)
		}
	}
	return c
}

type slot struct{ index, cx int }

// slots lists the plots bed b shows and the columns they're centered on.
// Without overflow plots fill beds row by row, centered. With overflow they
// go into columns (column j holds plots j*Beds … j*Beds+Beds-1), and the
// camera shows PerRow of them plus one sliding in, wrapping around.
func slots(lay Layout, b, n, cols int, pan float64) []slot {
	var out []slot
	if !lay.Overflow {
		lo, hi := b*lay.PerRow, min(n, (b+1)*lay.PerRow)
		left := (cols - (hi-lo)*BedCols) / 2
		for i := lo; i < hi; i++ {
			out = append(out, slot{i, left + (i-lo)*BedCols + BedCols/2})
		}
		return out
	}
	pan = math.Mod(pan, float64(lay.Columns))
	if pan < 0 {
		pan += float64(lay.Columns)
	}
	first := int(pan)
	shift := int((pan - float64(first)) * BedCols)
	left := (cols - lay.PerRow*BedCols) / 2
	for k := 0; k <= lay.PerRow; k++ {
		j := (first + k) % lay.Columns
		if i := j*lay.Beds + b; i < n {
			out = append(out, slot{i, left + k*BedCols + BedCols/2 - shift})
		}
	}
	return out
}

// The camera holds each view for panHold, then slides one column over
// panSlide.
const panHold, panSlide = 30 * time.Second, 1200 * time.Millisecond

// PanAt is the camera offset in plot columns, elapsed time into a live
// session, for a garden of the given number of columns. It wraps around.
func PanAt(elapsed time.Duration, columns int) float64 {
	if columns <= 1 || elapsed < 0 {
		return 0
	}
	k := int64(elapsed / panHold)
	into := elapsed - time.Duration(k)*panHold
	pos := float64(k)
	if k > 0 && into < panSlide {
		f := float64(into) / float64(panSlide)
		pos = float64(k-1) + f*f*(3-2*f) // ease in and out
	}
	return math.Mod(pos, float64(columns))
}

// Sliding reports whether the camera is mid-slide at elapsed.
func Sliding(elapsed time.Duration) bool {
	return elapsed >= panHold && elapsed%panHold < panSlide
}
```

- [ ] **Step 4: Rewrite `compose.go` around `Draw`**

Replace the whole of `internal/scene/compose.go` with:

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

// Compose draws a static garden as tall as its beds need: every plot has a
// place and nothing moves. Used for one-shot prints and replay.
func Compose(cols int, plots []Plot, t time.Time, seed int64) *pixel.Canvas {
	return Draw(View{Cols: cols, Plots: plots, Now: t, Seed: seed})
}

// drawPlot draws one plot centered on column cx of the bed whose top is
// pixel row oy.
func drawPlot(c *pixel.Canvas, v View, pl Plot, cx, oy int) {
	DrawPot(c, cx, oy+potTop)
	garden.Paint(c, pl.Plant, cx, oy+plantBaseY, pl.Style)
	if pl.Finished {
		DrawCloche(c, cx, oy+2, oy+potTop+PotH-1, BedCols-3)
		if v.Motion {
			DrawGlint(c, v.Now, cx, oy+2, oy+potTop+PotH-1, BedCols-3, nameSeed(pl.Name))
		}
	}
	labelRow := oy/2 + BedRows - 2
	label(c, cx, labelRow, pl.Name, nameColor)
	label(c, cx, labelRow+1, pl.Status, statusColor(pl))
}

// nameSeed turns a plot name into a stable seed, so each cloche glints on
// its own schedule.
func nameSeed(name string) int64 {
	var h int64
	for _, r := range name {
		h = h*31 + int64(r)
	}
	return h
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

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/scene/ && go test ./...`
Expected: PASS, and `TestComposeGolden` passes **without** `-update`. `Compose` now goes through `Draw` but must produce byte-identical static frames. If the golden test fails, the refactor changed drawing order or seeds: fix the code, not the golden file.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/scene
git commit -m "scene: draw frames for a window with bottom-anchored beds and panning

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Critters around healthy blooms

**Files:**
- Create: `internal/scene/critters.go`, `internal/scene/critters_test.go`
- Modify: `internal/garden/plant.go` (add `Flowers`), `internal/garden/plant_test.go`, `internal/scene/draw.go` (collect hosts, draw critters in live frames)

**Interfaces:**
- Consumes: Task 2 (`Draw`, `View`, `slots`, bed constants), v0.2 `garden.Plant`, and the test helpers `pushes(n)` and `t0` (garden) and `demoPlots()`, `at()` and `painted()` (scene).
- Produces:
  - `func (p *Plant) Flowers() int`
  - `type Host struct{ X, Y int }`
  - `func DrawCritters(c *pixel.Canvas, t time.Time, hosts []Host, seed int64)` (daylight only, at most 3 critters)
  - `func Flowering(pl Plot) bool` (not finished, health ≥ 0.6, at least one flower)
  - `Draw` draws critters when `v.Motion`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/garden/plant_test.go`:

```go
func TestFlowers(t *testing.T) {
	if n := Grow("r", Shrub, pushes(20)).Flowers(); n != 0 {
		t.Errorf("flowers without merges = %d", n)
	}
	events := append(pushes(20),
		Event{Kind: Merge, At: t0.Add(30 * time.Hour)},
		Event{Kind: Merge, At: t0.Add(31 * time.Hour)})
	if n := Grow("r", Shrub, events).Flowers(); n != 2 {
		t.Errorf("flowers after 2 merges = %d, want 2", n)
	}
}
```

`internal/scene/critters_test.go`:

```go
package scene

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func TestCrittersNeedDaylightAndHosts(t *testing.T) {
	hosts := []Host{{20, 10}}
	count := func(at time.Time, hs []Host) int {
		c := pixel.New(40, 20)
		DrawCritters(c, at, hs, 1)
		return painted(c)
	}
	if count(at(12, 0), nil) != 0 {
		t.Error("critters without any flowering plant")
	}
	if count(at(23, 0), hosts) != 0 {
		t.Error("critters at night")
	}
	if count(at(12, 0), hosts) == 0 {
		t.Error("no critters around a flowering plant at noon")
	}
}

func TestCrittersFly(t *testing.T) {
	hosts := []Host{{20, 10}, {30, 10}}
	frame := func(at time.Time) string {
		c := pixel.New(50, 20)
		DrawCritters(c, at, hosts, 1)
		return c.Encode(pixel.TrueColor)
	}
	if frame(at(12, 0)) == frame(at(12, 0).Add(1500*time.Millisecond)) {
		t.Error("critters did not move in 1.5 s")
	}
}

func TestLiveFramesHaveCritters(t *testing.T) {
	plots := demoPlots()[:1] // gag-core: healthy and flowering
	if !Flowering(plots[0]) {
		t.Fatal("test plot should be flowering")
	}
	v := View{Cols: BedCols, Plots: plots, Now: at(12, 0), Seed: 1}
	still := Draw(v).Encode(pixel.TrueColor)
	v.Motion = true
	if Draw(v).Encode(pixel.TrueColor) == still {
		t.Error("no critters in a live frame at noon")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/garden/ ./internal/scene/`
Expected: FAIL, `p.Flowers undefined`, `undefined: Host`, `undefined: DrawCritters`, `undefined: Flowering`.

- [ ] **Step 3: Implement `Flowers`**

Append to `internal/garden/plant.go`:

```go
// Flowers is how many flowers the plant has.
func (p *Plant) Flowers() int { return len(p.cellsOf(Flower)) }
```

- [ ] **Step 4: Implement critters**

`internal/scene/critters.go`:

```go
package scene

import (
	"math"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

var (
	beeBody    = rgb(250, 200, 40)
	critterInk = rgb(40, 30, 20)
	beeWing    = rgb(230, 240, 255)
	wingColors = []pixel.RGB{rgb(255, 160, 60), rgb(120, 180, 255), rgb(250, 250, 250)}
)

// Host is the pixel a critter circles: the middle of a flowering plant.
type Host struct{ X, Y int }

// Flowering reports whether a plot attracts critters: a healthy, blooming
// plant that isn't under glass.
func Flowering(pl Plot) bool {
	return !pl.Finished && pl.Style.Health >= 0.6 && pl.Plant.Flowers() > 0
}

// DrawCritters draws up to three bees and butterflies looping around the
// hosts, in daylight only. Their paths are pure functions of time, so a
// frame is reproducible.
func DrawCritters(c *pixel.Canvas, t time.Time, hosts []Host, seed int64) {
	if Darkness(t) > 0.5 {
		return
	}
	secs := float64(t.UnixMilli()) / 1000
	for i := 0; i < min(3, len(hosts)); i++ {
		h := hosts[i]
		phase := noise(seed, i, 7) * 2 * math.Pi
		speed := 0.35 + 0.2*noise(seed, i, 8) // radians per second
		x := h.X + int(math.Round(7*math.Sin(speed*secs+phase)))
		y := h.Y + int(math.Round(4*math.Sin(2*speed*secs+phase)))
		if i%2 == 0 {
			c.Set(x, y, beeBody)
			c.Set(x+1, y, critterInk)
			c.Set(x, y-1, beeWing)
			continue
		}
		wing := wingColors[i%len(wingColors)]
		c.Set(x, y, critterInk)
		if int(secs*6)%2 == 0 { // wings open
			c.Set(x-1, y-1, wing)
			c.Set(x+1, y-1, wing)
		} else {
			c.Set(x-1, y, wing)
			c.Set(x+1, y, wing)
		}
	}
}
```

- [ ] **Step 5: Draw critters in live frames**

In `internal/scene/draw.go`:

1. Add `"github.com/RursusAeternum/GitAGarden/internal/garden"` to the imports.
2. In `Draw`, just before `for b := 0; b < lay.Beds; b++ {`, add `var hosts []Host`.
3. Replace

```go
		for _, s := range slots(lay, b, len(v.Plots), cols, v.Pan) {
			drawPlot(c, v, v.Plots[s.index], s.cx, oy)
		}
```

with

```go
		for _, s := range slots(lay, b, len(v.Plots), cols, v.Pan) {
			pl := v.Plots[s.index]
			drawPlot(c, v, pl, s.cx, oy)
			if v.Motion && Flowering(pl) {
				hosts = append(hosts, Host{X: s.cx, Y: oy + plantBaseY - garden.Height/2})
			}
		}
```

4. Replace the final `return c` of `Draw` with:

```go
	if v.Motion {
		DrawCritters(c, v.Now, hosts, v.Seed)
	}
	return c
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/garden/ ./internal/scene/ && go test ./...`
Expected: PASS. The golden frame is unchanged, since static frames have no motion.

- [ ] **Step 7: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/garden internal/scene
git commit -m "scene: bees and butterflies around healthy, blooming plants

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: `internal/live` snapshot types and the ticker

**Files:**
- Create: `internal/live/snapshot.go`, `internal/live/ticker.go`, `internal/live/ticker_test.go`

**Interfaces:**
- Consumes: `garden.Plant`, `garden.Health`, `runewidth.StringWidth/Truncate/FillRight`.
- Produces:
  - `type Repo struct { Name string; Plant *garden.Plant; Finished bool }`
  - `type Snapshot struct { Repos []Repo; Commits7d int; FetchedAt time.Time; Offline bool; Note string }`
  - `type Item struct{ Icon, Text string }`
  - `func Items(repos []Repo, now time.Time, decay float64, commits7d int) []Item` (wilting plants, most wilted first, or one calm line)
  - `func Line(items []Item, elapsed time.Duration, status string, width int) string` (exactly `width` cells, a new item every 4 s)
  - `func Status(s Snapshot, now time.Time, loading, failed bool) string`
  - Unexported `fit(left, right string, width int) string` (used by Task 5) and `ago(time.Duration) string`
  - The test file owns `TestMain` (pins `time.Local` to UTC for the whole package) and the helpers `t0` and `grow(name, pushes, merges, last)`, which Task 5 reuses.

- [ ] **Step 1: Write the failing tests**

`internal/live/ticker_test.go`:

```go
package live

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
)

func TestMain(m *testing.M) {
	time.Local = time.UTC // daylight and sky follow the local clock; pin it
	os.Exit(m.Run())
}

var t0 = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

// grow makes a shrub Repo whose last push was at last; merges add flowers.
func grow(name string, pushes, merges int, last time.Time) Repo {
	var ev []garden.Event
	for i := 0; i < pushes; i++ {
		ev = append(ev, garden.Event{Kind: garden.Push, At: last.Add(-time.Duration(pushes-1-i) * time.Hour)})
	}
	for i := 0; i < merges; i++ {
		ev = append(ev, garden.Event{Kind: garden.Merge, At: last.Add(-time.Duration(merges-i) * time.Minute)})
	}
	return Repo{Name: name, Plant: garden.Grow(name, garden.Shrub, ev)}
}

func TestItemsCallOutWiltingMostWiltedFirst(t *testing.T) {
	done := grow("done", 30, 0, t0.Add(-90*24*time.Hour))
	done.Finished = true
	repos := []Repo{
		grow("fresh", 30, 0, t0.Add(-24*time.Hour)),
		grow("quiet", 30, 0, t0.Add(-30*24*time.Hour)),
		grow("ancient", 30, 0, t0.Add(-60*24*time.Hour)),
		done,
	}
	items := Items(repos, t0, 45, 12)
	want := []string{"ancient: 60d quiet", "quiet: 30d quiet"}
	if len(items) != len(want) {
		t.Fatalf("items = %+v, want %v", items, want)
	}
	for i, w := range want {
		if items[i].Text != w || items[i].Icon != "🥀" {
			t.Errorf("item %d = %+v, want 🥀 %q", i, items[i], w)
		}
	}
}

func TestItemsAreCalmWhenAllIsWell(t *testing.T) {
	repos := []Repo{grow("fresh", 30, 0, t0.Add(-time.Hour))}
	for commits, want := range map[int]string{
		12: "Garden thriving: 12 commits this week",
		1:  "Garden thriving: 1 commit this week",
		0:  "All quiet in the garden",
	} {
		got := Items(repos, t0, 45, commits)
		if len(got) != 1 || got[0].Text != want || got[0].Icon != "🌱" {
			t.Errorf("commits=%d: %+v, want 🌱 %q", commits, got, want)
		}
	}
}

func TestLineCyclesAndFitsExactly(t *testing.T) {
	items := []Item{{"🥀", "a: 3d quiet"}, {"🥀", "b: 9d quiet"}}
	if l := Line(items, 0, "updated just now", 60); !strings.Contains(l, "a: 3d quiet") {
		t.Errorf("first item missing: %q", l)
	}
	if l := Line(items, 4*time.Second, "updated just now", 60); !strings.Contains(l, "b: 9d quiet") {
		t.Errorf("second item missing after 4s: %q", l)
	}
	if l := Line(items, 8*time.Second, "", 60); !strings.Contains(l, "a: 3d quiet") {
		t.Errorf("items should wrap around: %q", l)
	}
	for _, w := range []int{60, 25, 10, 3} {
		if got := runewidth.StringWidth(Line(items, 0, "updated just now", w)); got != w {
			t.Errorf("width %d: line is %d cells", w, got)
		}
	}
}

func TestLineTruncatesTheItemNotTheStatus(t *testing.T) {
	long := []Item{{"🥀", strings.Repeat("very-long-repo-name-", 5) + ": 9d quiet"}}
	l := Line(long, 0, "updated 3m ago", 50)
	if !strings.HasSuffix(l, "updated 3m ago ") {
		t.Errorf("status lost: %q", l)
	}
	if !strings.Contains(l, "…") {
		t.Errorf("long item should be truncated with …: %q", l)
	}
}

func TestStatus(t *testing.T) {
	fetched := Snapshot{FetchedAt: t0.Add(-3 * time.Minute)}
	cases := []struct{ got, want string }{
		{Status(Snapshot{}, t0, true, false), "loading…"},
		{Status(Snapshot{}, t0, false, false), ""},
		{Status(fetched, t0, false, false), "updated 3m ago"},
		{Status(fetched, t0, true, false), "refreshing…"},
		{Status(fetched, t0, false, true), "refresh failed · data from 3m ago"},
		{Status(Snapshot{FetchedAt: t0.Add(-2 * time.Hour), Offline: true}, t0, false, false), "offline · cached 2h ago"},
		{Status(Snapshot{FetchedAt: t0.Add(-20 * time.Second)}, t0, false, false), "updated just now"},
		{Status(Snapshot{Note: "demo garden", FetchedAt: t0}, t0, false, false), "demo garden"},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("case %d: %q, want %q", i, c.got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/live/`
Expected: FAIL, `undefined: Repo`, `undefined: Items` and similar.

- [ ] **Step 3: Implement the types**

`internal/live/snapshot.go`:

```go
// Package live is the always-on garden: an animated, fullscreen view of the
// repos that reloads in the background and runs until you quit it.
package live

import (
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
)

// Repo is one plant in the live garden.
type Repo struct {
	Name     string
	Plant    *garden.Plant
	Finished bool
}

// Snapshot is one load of the garden's data.
type Snapshot struct {
	Repos     []Repo
	Commits7d int       // commits across all repos in the last 7 days
	FetchedAt time.Time // when the data was fetched (the oldest repo's fetch)
	Offline   bool      // GitHub was unreachable; this is cached data
	Note      string    // replaces the ticker's freshness status, e.g. demo mode
}
```

- [ ] **Step 4: Implement the ticker**

`internal/live/ticker.go`:

```go
package live

import (
	"fmt"
	"sort"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
)

// Item is one message the ticker cycles through.
type Item struct{ Icon, Text string }

const (
	wiltThreshold = 0.5             // plants below this health get called out
	itemEvery     = 4 * time.Second // how long each ticker item shows
)

// Items lists what needs attention, most urgent first. In this phase that's
// neglect: plants whose health fell below wiltThreshold, most wilted first.
// When nothing needs attention it returns one calm line.
func Items(repos []Repo, now time.Time, decay float64, commits7d int) []Item {
	type wilting struct {
		name   string
		health float64
		idle   time.Duration
	}
	var ws []wilting
	for _, r := range repos {
		if r.Finished || r.Plant.LastTended.IsZero() {
			continue
		}
		if h := garden.Health(r.Plant, now, decay); h < wiltThreshold {
			ws = append(ws, wilting{r.Name, h, now.Sub(r.Plant.LastTended)})
		}
	}
	sort.SliceStable(ws, func(i, j int) bool { return ws[i].health < ws[j].health })
	var items []Item
	for _, w := range ws {
		items = append(items, Item{"🥀", fmt.Sprintf("%s: %dd quiet", w.name, int(w.idle.Hours()/24))})
	}
	if len(items) == 0 {
		items = append(items, calm(commits7d))
	}
	return items
}

func calm(commits7d int) Item {
	switch commits7d {
	case 0:
		return Item{"🌱", "All quiet in the garden"}
	case 1:
		return Item{"🌱", "Garden thriving: 1 commit this week"}
	}
	return Item{"🌱", fmt.Sprintf("Garden thriving: %d commits this week", commits7d)}
}

// Line renders the ticker in exactly width cells: the current item on the
// left (a new one every itemEvery) and status on the right.
func Line(items []Item, elapsed time.Duration, status string, width int) string {
	left := ""
	if len(items) > 0 {
		it := items[int(elapsed/itemEvery)%len(items)]
		left = " " + it.Icon + " " + it.Text
	}
	right := ""
	if status != "" {
		right = status + " "
	}
	return fit(left, right, width)
}

// fit lays out left and right text in exactly width terminal cells,
// truncating the left side first.
func fit(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	rw := runewidth.StringWidth(right)
	if rw >= width {
		return runewidth.FillRight(runewidth.Truncate(right, width, ""), width)
	}
	left = runewidth.Truncate(left, width-rw, "…")
	return runewidth.FillRight(left, width-rw) + right
}

// Status is the ticker's right side: how fresh the data is, or why it's
// stale.
func Status(s Snapshot, now time.Time, loading, failed bool) string {
	switch {
	case s.Note != "":
		return s.Note
	case s.FetchedAt.IsZero():
		if loading {
			return "loading…"
		}
		return ""
	case failed:
		return "refresh failed · data from " + ago(now.Sub(s.FetchedAt))
	case s.Offline:
		return "offline · cached " + ago(now.Sub(s.FetchedAt))
	case loading:
		return "refreshing…"
	}
	return "updated " + ago(now.Sub(s.FetchedAt))
}

func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/live/ && go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/live
git commit -m "live: snapshot types and the ticker

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: The live Bubble Tea model

**Files:**
- Create: `internal/live/model.go`, `internal/live/model_test.go`

**Interfaces:**
- Consumes: Task 2 (`scene.View`, `scene.Draw`, `scene.LayoutFor`, `scene.PanAt`, `scene.Sliding`), Task 3 (`scene.Flowering`), Task 4 (`Repo`, `Snapshot`, `Items`, `Line`, `Status`, `fit`, and the test helpers `t0` and `grow`), plus `garden.Health`, `garden.Status`, `garden.Style`, `scene.Darkness` and `pixel.Profile`.
- Produces:
  - `type Config struct { Load func(ctx context.Context) (Snapshot, error); Refresh time.Duration; DecayDays float64; Ahead time.Duration; Profile pixel.Profile; Now func() time.Time }`
  - `func New(cfg Config) Model` (`Model` implements `tea.Model`)
  - Unexported messages `tickMsg`, `loadedMsg{snap Snapshot; err error}` and `refreshMsg{gen int}`, used by the tests.

- [ ] **Step 1: Write the failing tests**

`internal/live/model_test.go`:

```go
package live

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

func visible(s string) string { return ansi.ReplaceAllString(s, "") }

func garden3() Snapshot {
	return Snapshot{
		Repos: []Repo{
			grow("bloom", 60, 4, t0.Add(-time.Hour)),
			grow("quiet", 40, 0, t0.Add(-40*24*time.Hour)),
			grow("seed", 1, 0, t0.Add(-2*time.Hour)),
		},
		Commits7d: 5,
		FetchedAt: t0.Add(-2 * time.Minute),
	}
}

func newModel(snap Snapshot, err error, now time.Time) Model {
	return New(Config{
		Load:      func(context.Context) (Snapshot, error) { return snap, err },
		DecayDays: 45,
		Profile:   pixel.TrueColor,
		Now:       func() time.Time { return now },
	})
}

func step(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

// ready sizes the window and runs the first load.
func ready(m Model, cols, rows int) Model {
	m, _ = step(m, tea.WindowSizeMsg{Width: cols, Height: rows})
	m, _ = step(m, m.load()())
	return m
}

func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func checkSize(t *testing.T, view string, cols, rows int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != rows {
		t.Fatalf("%dx%d: view has %d lines", cols, rows, len(lines))
	}
	for i, l := range lines {
		if w := runewidth.StringWidth(visible(l)); w != cols {
			t.Fatalf("%dx%d: line %d is %d cells wide", cols, rows, i, w)
		}
	}
}

func TestViewFitsTheWindow(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {30, 12}, {240, 65}} {
		m := ready(newModel(garden3(), nil, t0), size[0], size[1])
		checkSize(t, m.View(), size[0], size[1])
	}
}

func TestTickerShowsWiltingAndFreshness(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	lines := strings.Split(m.View(), "\n")
	last := visible(lines[len(lines)-1])
	if !strings.Contains(last, "🥀 quiet: 40d quiet") || !strings.Contains(last, "updated 2m ago") {
		t.Errorf("ticker = %q", last)
	}
}

func TestResizeFollowsTheWindow(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 80, 24)
	m, _ = step(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	checkSize(t, m.View(), 100, 30)
	m, _ = step(m, tea.WindowSizeMsg{Width: 20, Height: 10})
	if !strings.Contains(m.View(), "bigger") {
		t.Error("tiny window should ask to be bigger")
	}
	m, _ = step(m, tea.WindowSizeMsg{Width: 90, Height: 26})
	checkSize(t, m.View(), 90, 26)
}

func TestFailedRefreshKeepsGarden(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	m, cmd := step(m, loadedMsg{err: errors.New("rate limited")})
	if cmd == nil {
		t.Error("a failed refresh should schedule the next one")
	}
	view := m.View()
	checkSize(t, view, 100, 30)
	if !strings.Contains(visible(view), "refresh failed · data from 2m ago") {
		t.Error("ticker should say the refresh failed")
	}
}

func TestFirstLoadFailureOffersRetry(t *testing.T) {
	m := ready(newModel(Snapshot{}, errors.New("no network"), t0), 80, 24)
	if v := m.View(); !strings.Contains(v, "couldn't load your garden") || !strings.Contains(v, "press r") {
		t.Errorf("view = %q", visible(v))
	}
}

func TestKeys(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 80, 24)
	m, _ = step(m, key("t"))
	if v := visible(m.View()); strings.Contains(v, "updated") {
		t.Error("t should hide the ticker")
	}
	checkSize(t, m.View(), 80, 24)
	m, _ = step(m, key("?"))
	if v := visible(m.View()); !strings.Contains(v, "q quit") {
		t.Error("? should show the keys")
	}
	_, cmd := step(m, key("q"))
	if cmd == nil {
		t.Fatal("q should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q should send tea.QuitMsg")
	}
}

func TestStaleRefreshTimersAreIgnored(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 80, 24) // one load done: gen 1
	if _, cmd := step(m, refreshMsg{gen: 0}); cmd != nil {
		t.Error("a timer from an older load should be ignored")
	}
	m, cmd := step(m, refreshMsg{gen: 1})
	if cmd == nil || !m.loading {
		t.Error("the current timer should start a load")
	}
}

func TestMergeKeepsFirstSeenOrder(t *testing.T) {
	a, b, c := Repo{Name: "a"}, Repo{Name: "b"}, Repo{Name: "c"}
	got := merge([]Repo{a, b}, []Repo{c, b, a})
	if len(got) != 3 || got[0].Name != "a" || got[1].Name != "b" || got[2].Name != "c" {
		t.Errorf("merge = %v", got)
	}
	if got := merge([]Repo{a, b}, []Repo{b}); len(got) != 1 || got[0].Name != "b" {
		t.Errorf("vanished repos should be dropped: %v", got)
	}
}

func TestFrameRate(t *testing.T) {
	if m := ready(newModel(garden3(), nil, t0), 80, 24); m.frameInterval() != fastFrame {
		t.Error("a blooming garden at noon has critters: want the fast frame rate")
	}
	night := t0.Add(11 * time.Hour) // 23:00
	if m := ready(newModel(garden3(), nil, night), 80, 24); m.frameInterval() != slowFrame {
		t.Error("at night nothing flies: want the slow frame rate")
	}
	if m := New(Config{Now: func() time.Time { return t0 }}); m.frameInterval() != slowFrame {
		t.Error("an empty garden should idle")
	}
}

func TestLongSleepKeepsDrawing(t *testing.T) {
	now := t0
	m := New(Config{
		Load:      func(context.Context) (Snapshot, error) { return garden3(), nil },
		DecayDays: 45,
		Now:       func() time.Time { return now },
	})
	m = ready(m, 80, 24)
	now = t0.Add(10*24*time.Hour + 37*time.Minute) // the laptop slept for ten days
	checkSize(t, m.View(), 80, 24)
}

func BenchmarkLiveFrame(b *testing.B) {
	snap := garden3()
	for i := 0; len(snap.Repos) < 12; i++ {
		r := snap.Repos[i%3]
		r.Name += strings.Repeat("x", i+1)
		snap.Repos = append(snap.Repos, r)
	}
	m := ready(newModel(snap, nil, t0), 240, 65)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/live/`
Expected: FAIL, `undefined: New`, `undefined: Config`, `undefined: loadedMsg` and similar.

- [ ] **Step 3: Implement the model**

`internal/live/model.go`:

```go
package live

import (
	"context"
	"hash/fnv"
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

const (
	fastFrame        = 125 * time.Millisecond // ~8 fps while critters fly or the camera slides
	slowFrame        = 500 * time.Millisecond // ~2 fps when only clouds and sway move
	defaultRefresh   = 5 * time.Minute
	swayPeriod       = 5 * time.Second
	minCols, minRows = 24, 12
	helpLine         = "q quit · r refresh · t ticker · ? help"
)

var tickerStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#d0d0c8")).
	Background(lipgloss.Color("#1c1a17"))

// Config is what the live view needs from the outside world.
type Config struct {
	Load      func(ctx context.Context) (Snapshot, error)
	Refresh   time.Duration // how often to reload; default 5m
	DecayDays float64
	Ahead     time.Duration    // added to the wall clock, for -simulate
	Profile   pixel.Profile
	Now       func() time.Time // wall clock; default time.Now
}

type Model struct {
	cfg        Config
	cols, rows int
	start      time.Time
	repos      []Repo // stable order: first seen first
	snap       Snapshot
	loading    bool
	loadErr    error // the last load's error, if it failed
	gen        int   // bumps on every finished load; older refresh timers are ignored
	ticker     bool
	help       bool
}

type (
	tickMsg   struct{}
	loadedMsg struct {
		snap Snapshot
		err  error
	}
	refreshMsg struct{ gen int }
)

func New(cfg Config) Model {
	if cfg.Refresh <= 0 {
		cfg.Refresh = defaultRefresh
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return Model{cfg: cfg, start: cfg.Now(), loading: true, ticker: true}
}

func (m Model) Init() tea.Cmd { return tea.Batch(m.load(), m.tick()) }

func (m Model) load() tea.Cmd {
	load := m.cfg.Load
	return func() tea.Msg {
		s, err := load(context.Background())
		return loadedMsg{s, err}
	}
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(m.frameInterval(), func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.cols, m.rows = msg.Width, msg.Height
	case tickMsg:
		return m, m.tick()
	case loadedMsg:
		m.loading = false
		m.gen++
		m.loadErr = msg.err
		if msg.err == nil {
			m.snap = msg.snap
			m.repos = merge(m.repos, msg.snap.Repos)
		}
		gen := m.gen
		return m, tea.Tick(m.cfg.Refresh, func(time.Time) tea.Msg { return refreshMsg{gen} })
	case refreshMsg:
		if msg.gen != m.gen || m.loading {
			return m, nil
		}
		m.loading = true
		return m, m.load()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "r":
			if !m.loading {
				m.loading = true
				return m, m.load()
			}
		case "t":
			m.ticker = !m.ticker
		case "?":
			m.help = !m.help
		}
	}
	return m, nil
}

// merge keeps the order plants first appeared in: known repos are updated in
// place, new ones are appended, and ones no longer loaded are dropped.
func merge(old, fresh []Repo) []Repo {
	byName := make(map[string]Repo, len(fresh))
	for _, r := range fresh {
		byName[r.Name] = r
	}
	var out []Repo
	seen := map[string]bool{}
	for _, r := range old {
		if f, ok := byName[r.Name]; ok {
			out = append(out, f)
			seen[r.Name] = true
		}
	}
	for _, r := range fresh {
		if !seen[r.Name] {
			out = append(out, r)
		}
	}
	return out
}

func (m Model) now() time.Time         { return m.cfg.Now().Add(m.cfg.Ahead) }
func (m Model) elapsed() time.Duration { return m.cfg.Now().Sub(m.start) }
func (m Model) tickerShown() bool      { return m.ticker || m.help }

func (m Model) gardenRows() int {
	if m.tickerShown() {
		return m.rows - 1
	}
	return m.rows
}

func (m Model) plots(at time.Time) []scene.Plot {
	out := make([]scene.Plot, len(m.repos))
	for i, r := range m.repos {
		st := garden.Style{Health: 1} // under glass: full health, still air
		if !r.Finished {
			h := garden.Health(r.Plant, at, m.cfg.DecayDays)
			st = garden.Style{Health: h, Sway: sway(r.Name, h, at)}
		}
		out[i] = scene.Plot{Plant: r.Plant, Style: st, Finished: r.Finished,
			Name: r.Name, Status: garden.Status(r.Plant, at, r.Finished)}
	}
	return out
}

// sway is a plant's gentle lean at time at, in pixels at its top. Each plant
// has its own phase; wilted plants barely move.
func sway(name string, health float64, at time.Time) float64 {
	h := fnv.New32a()
	h.Write([]byte(name))
	phase := float64(h.Sum32()%1000) / 1000 * 2 * math.Pi
	period := swayPeriod.Milliseconds()
	return 1.5 * health * math.Sin(2*math.Pi*float64(at.UnixMilli()%period)/float64(period)+phase)
}

// frameInterval is fast while critters fly or the camera slides (or is about
// to), slow otherwise, so an idle garden costs little.
func (m Model) frameInterval() time.Duration {
	if m.busy() {
		return fastFrame
	}
	return slowFrame
}

func (m Model) busy() bool {
	if len(m.repos) == 0 || m.cols == 0 {
		return false
	}
	lay := scene.LayoutFor(m.cols, m.gardenRows(), len(m.repos))
	if lay.Overflow && (scene.Sliding(m.elapsed()) || scene.Sliding(m.elapsed()+slowFrame)) {
		return true
	}
	at := m.now()
	if scene.Darkness(at) > 0.5 {
		return false
	}
	for _, pl := range m.plots(at) {
		if scene.Flowering(pl) {
			return true
		}
	}
	return false
}

func (m Model) View() string {
	switch {
	case m.cols == 0:
		return "" // waiting for the first window size
	case m.cols < minCols || m.rows < minRows:
		return center(m.cols, m.rows, "make the window a bit bigger 🌱")
	case len(m.repos) == 0 && m.loadErr != nil:
		return center(m.cols, m.rows, "couldn't load your garden: "+m.loadErr.Error()+"\npress r to retry")
	case len(m.repos) == 0:
		return center(m.cols, m.rows, "🌱 growing your garden…")
	}
	at := m.now()
	rows := m.gardenRows()
	plots := m.plots(at)
	v := scene.View{Cols: m.cols, Rows: rows, Plots: plots, Now: at, Seed: 1, Motion: true}
	if lay := scene.LayoutFor(m.cols, rows, len(plots)); lay.Overflow {
		v.Pan = scene.PanAt(m.elapsed(), lay.Columns)
	}
	out := scene.Draw(v).Encode(m.cfg.Profile)
	if m.tickerShown() {
		out += "\n" + tickerStyle.Render(m.tickerLine(at))
	}
	return out
}

func (m Model) tickerLine(at time.Time) string {
	status := Status(m.snap, m.cfg.Now(), m.loading, m.loadErr != nil)
	if m.help {
		return fit(" "+helpLine, status+" ", m.cols)
	}
	return Line(Items(m.repos, at, m.cfg.DecayDays, m.snap.Commits7d), m.elapsed(), status, m.cols)
}

// center wraps text to the window's width and centers it in the window.
func center(cols, rows int, text string) string {
	wrapped := lipgloss.NewStyle().Width(cols).Align(lipgloss.Center).Render(text)
	return lipgloss.Place(cols, rows, lipgloss.Center, lipgloss.Center, wrapped)
}
```

- [ ] **Step 4: Run the tests and the benchmark**

Run: `go test ./internal/live/ && go test ./internal/live/ -run '^$' -bench LiveFrame -benchtime 20x && go test ./...`
Expected: PASS, and `BenchmarkLiveFrame` reports well under `10000000 ns/op`.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/live
git commit -m "live: animated fullscreen garden with background refresh and ticker

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Wire the live view into `gag`

**Files:**
- Create: `cmd/gag/garden.go`, `cmd/gag/garden_test.go`, `internal/github/client_test.go`
- Modify: `internal/github/client.go` (add `ErrNoToken`), `cmd/gag/main.go` (`loadRepos` reports offline instead of logging; `runGarden` and demo code move to `garden.go`), `README.md`

**Interfaces:**
- Consumes: Task 5 (`live.New`, `live.Config`), Task 4 (`live.Repo`, `live.Snapshot`), v0.2 `scene.Compose`, `colorProfile()`, `termWidth()`, `parseSpan()`, `github.*`, `garden.*`.
- Produces:
  - `var github.ErrNoToken error`
  - `cmd/gag`: `func loadRepos(ctx, names []string, owner string, limit int, ttl time.Duration, log func(string)) ([]*github.Repo, bool, error)` (the bool means offline or partly stale)
  - `type source struct{ names []string; owner string; limit int; ttl time.Duration; demo bool; decay float64 }` with `func (s source) snapshot(ctx context.Context) (live.Snapshot, error)`
  - `func demoGarden(now time.Time) []live.Repo`
  - `gag` and `gag garden` open the live view when stdout is a terminal. `-once` (or a non-terminal stdout) prints one static frame. `-refresh` sets the live reload interval. `-watch` is gone.

- [ ] **Step 1: Write the failing tests**

`internal/github/client_test.go`:

```go
package github

import (
	"errors"
	"testing"
)

func TestNewClientWithoutTokenIsErrNoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PATH", t.TempDir()) // no gh CLI to borrow a token from
	if _, err := NewClient(); !errors.Is(err, ErrNoToken) {
		t.Fatalf("err = %v, want ErrNoToken", err)
	}
}
```

`cmd/gag/garden_test.go`:

```go
package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSnapshotFallsBackToDemoWithoutToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PATH", t.TempDir())
	snap, err := source{limit: 8, ttl: time.Minute, decay: 45}.snapshot(context.Background())
	if err != nil {
		t.Fatalf("a missing token should fall back to the demo, got error %v", err)
	}
	if !strings.Contains(snap.Note, "no GitHub token") {
		t.Errorf("note = %q, want it to explain the missing token", snap.Note)
	}
	if len(snap.Repos) != len(demo) {
		t.Errorf("repos = %d, want the %d demo repos", len(snap.Repos), len(demo))
	}
}

func TestDemoSnapshot(t *testing.T) {
	snap, err := source{demo: true, decay: 45}.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Note != "demo garden" || len(snap.Repos) != len(demo) || snap.FetchedAt.IsZero() {
		t.Errorf("snapshot = note %q, %d repos, fetched %v", snap.Note, len(snap.Repos), snap.FetchedAt)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/github/ ./cmd/gag/`
Expected: FAIL, `undefined: ErrNoToken` and `undefined: source`.

- [ ] **Step 3: Add `ErrNoToken`**

In `internal/github/client.go`, add above `NewClient`:

```go
// ErrNoToken means no GitHub token was found in the environment or the gh CLI.
var ErrNoToken = errors.New("no GitHub token: set GITHUB_TOKEN or run `gh auth login` (or try `gag garden -demo`)")
```

and change the error return in `NewClient` to `return nil, ErrNoToken`.

- [ ] **Step 4: Make `loadRepos` report offline instead of logging**

In `cmd/gag/main.go`:

1. Replace the `loadRepos` and `cachedRepos` functions with:

```go
// loadRepos returns synced repos: the named ones, or the limit most recently
// pushed by owner (your own repos when owner is empty). If GitHub is
// unreachable it falls back to the local cache and reports offline. log gets
// progress lines ("fetching owner/repo"); nil discards them.
func loadRepos(ctx context.Context, names []string, owner string, limit int, ttl time.Duration, log func(string)) ([]*github.Repo, bool, error) {
	store, err := github.OpenStore()
	if err != nil {
		return nil, false, err
	}
	c, err := github.NewClient()
	if err != nil {
		return nil, false, err
	}

	var metas []*github.Repo
	if len(names) > 0 {
		for _, n := range names {
			m, err := c.LookupRepo(ctx, n)
			if err != nil {
				return cachedRepos(store, names, "", limit, err)
			}
			metas = append(metas, m)
		}
	} else {
		if owner != "" {
			metas, err = c.ListOwnerRepos(ctx, owner, limit)
		} else {
			metas, err = c.ListRepos(ctx, limit)
		}
		if err != nil {
			return cachedRepos(store, nil, owner, limit, err)
		}
	}

	repos, err := github.Sync(ctx, c, store, metas, ttl, log)
	if err != nil {
		if len(repos) == 0 {
			return nil, false, err
		}
		return repos, true, nil // some repos are stale cached copies
	}
	return repos, false, nil
}

// cachedRepos is loadRepos' offline fallback: the cached copies of the
// requested repos, or cause if there are none.
func cachedRepos(store *github.Store, names []string, owner string, limit int, cause error) ([]*github.Repo, bool, error) {
	var repos []*github.Repo
	if len(names) > 0 {
		for _, n := range names {
			if r := store.Repos[n]; r != nil {
				repos = append(repos, r)
			}
		}
	} else {
		for _, r := range store.Repos {
			if owner == "" || strings.EqualFold(strings.SplitN(r.NameWithOwner, "/", 2)[0], owner) {
				repos = append(repos, r)
			}
		}
		sort.Slice(repos, func(i, j int) bool { return repos[i].PushedAt.After(repos[j].PushedAt) })
		repos = repos[:min(limit, len(repos))]
	}
	if len(repos) == 0 {
		return nil, false, cause
	}
	return repos, true, nil
}
```

2. In `runReplay`, change `repos, err := loadRepos(context.Background(), []string{*repo}, "", 1, *ttl)` to `repos, _, err := loadRepos(context.Background(), []string{*repo}, "", 1, *ttl, logf)`.
3. Delete everything from `func runGarden(args []string) error {` to the end of the file (`runGarden`, `demoRepo`, `demo`, `plotFor`, `demoPlots`). They move to `garden.go` in Step 5.
4. Remove the now-unused `"github.com/RursusAeternum/GitAGarden/internal/scene"` import.
5. Update the doc comment at the top of the file to:

```go
// Command gag renders your git projects as a garden in the terminal.
//
//	gag          the live, animated garden of your GitHub repos (= gag garden)
//	gag garden   the same; -once prints one static frame, -demo uses fake repos
//	gag replay   time-lapse of one plant growing from its history
//	gag version
```

- [ ] **Step 5: Write `garden.go`**

`cmd/gag/garden.go`:

```go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/github"
	"github.com/RursusAeternum/GitAGarden/internal/live"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func runGarden(args []string) error {
	fs := flag.NewFlagSet("garden", flag.ExitOnError)
	demoFlag := fs.Bool("demo", false, "show fake demo repos instead of GitHub")
	limit := fs.Int("limit", 8, "how many recently pushed repos to show")
	names := fs.String("repos", "", "comma-separated owner/name list to show instead (any public repo works)")
	user := fs.String("user", "", "show another GitHub user's or organization's public garden")
	ttl := fs.Duration("ttl", 15*time.Minute, "reuse cached GitHub data younger than this (live view: half of -refresh)")
	decay := fs.Float64("decay", 45, "days of neglect until fully wilted")
	simulate := fs.String("simulate", "", "fast-forward: show the garden as it would look after this long untouched, e.g. 30d, 2w, 36h")
	once := fs.Bool("once", false, "print one static frame and exit (the default when output isn't a terminal)")
	refresh := fs.Duration("refresh", 5*time.Minute, "how often the live view reloads GitHub data")
	fs.Parse(args)

	var ahead time.Duration
	if *simulate != "" {
		var err error
		if ahead, err = parseSpan(*simulate); err != nil || ahead < 0 {
			return fmt.Errorf("-simulate %q: want a positive span like 30d, 2w or 36h", *simulate)
		}
	}
	var list []string
	for _, s := range strings.Split(*names, ",") {
		if s = strings.TrimSpace(s); s != "" {
			list = append(list, s)
		}
	}
	src := source{names: list, owner: *user, limit: *limit, ttl: *ttl, demo: *demoFlag, decay: *decay}

	if *once || !term.IsTerminal(int(os.Stdout.Fd())) {
		return printOnce(src, ahead, *simulate)
	}
	ttlSet := false
	fs.Visit(func(f *flag.Flag) { ttlSet = ttlSet || f.Name == "ttl" })
	if !ttlSet {
		src.ttl = *refresh / 2 // so each background refresh really fetches
	}
	m := live.New(live.Config{Load: src.snapshot, Refresh: *refresh, DecayDays: *decay, Ahead: ahead, Profile: colorProfile()})
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// source says which repos the garden shows and how to load them.
type source struct {
	names []string
	owner string
	limit int
	ttl   time.Duration
	demo  bool
	decay float64
}

// snapshot loads the repos and grows their plants for the live view. It
// writes nothing to the terminal. Without a GitHub token it falls back to
// the demo garden with a note saying why.
func (s source) snapshot(ctx context.Context) (live.Snapshot, error) {
	now := time.Now()
	if s.demo {
		return live.Snapshot{Repos: demoGarden(now), FetchedAt: now, Note: "demo garden"}, nil
	}
	repos, offline, err := loadRepos(ctx, s.names, s.owner, s.limit, s.ttl, nil)
	if errors.Is(err, github.ErrNoToken) {
		return live.Snapshot{Repos: demoGarden(now), FetchedAt: now, Note: "demo · no GitHub token: run gh auth login"}, nil
	}
	if err != nil {
		return live.Snapshot{}, err
	}
	snap := live.Snapshot{Offline: offline, FetchedAt: now}
	weekAgo := now.Add(-7 * 24 * time.Hour)
	for _, r := range repos {
		snap.Repos = append(snap.Repos, live.Repo{Name: r.Name(), Plant: garden.Grow(r.Name(), r.Species(), r.Events()), Finished: r.Finished()})
		if !r.FetchedAt.IsZero() && r.FetchedAt.Before(snap.FetchedAt) {
			snap.FetchedAt = r.FetchedAt
		}
		for _, c := range r.Commits {
			if c.At.After(weekAgo) {
				snap.Commits7d++
			}
		}
	}
	return snap, nil
}

// printOnce prints one static frame: for pipes, scripts and -once.
func printOnce(src source, ahead time.Duration, simulate string) error {
	now := time.Now()
	at := now.Add(ahead) // the moment the garden is drawn at
	header := ""
	if ahead > 0 {
		header = fmt.Sprintf("simulating %s ahead: %s, nothing tended\n\n", simulate, at.Format("2006-01-02"))
	}
	var plots []scene.Plot
	if src.demo {
		for _, r := range demoGarden(now) {
			plots = append(plots, plotFor(r.Plant, r.Name, r.Finished, at, src.decay))
		}
	} else {
		repos, offline, err := loadRepos(context.Background(), src.names, src.owner, src.limit, src.ttl, logf)
		if err != nil {
			return err
		}
		if offline {
			logf("GitHub unreachable or partly stale; showing cached data")
		}
		for _, r := range repos {
			plots = append(plots, plotFor(garden.Grow(r.Name(), r.Species(), r.Events()), r.Name(), r.Finished(), at, src.decay))
		}
	}
	fmt.Print(header + scene.Compose(termWidth(), plots, at, 1).Encode(colorProfile()) + "\n")
	return nil
}

// plotFor bundles a grown plant with its health and status at time at.
func plotFor(p *garden.Plant, name string, finished bool, at time.Time, decay float64) scene.Plot {
	h := garden.Health(p, at, decay)
	if finished {
		h = 1
	}
	return scene.Plot{Plant: p, Style: garden.Style{Health: h}, Finished: finished,
		Name: name, Status: garden.Status(p, at, finished)}
}

type demoRepo struct {
	name, lang string
	events     int
	idleDays   int
	finished   bool
}

var demo = []demoRepo{
	{"gag-core", "go", 160, 0, false},
	{"rustyfs", "rust", 90, 12, false},
	{"notebook-api", "python", 70, 3, false},
	{"old-blog", "go", 120, 400, true},
	{"dotfiles", "shell", 40, 35, false},
	{"tiny-cli", "rust", 12, 1, false},
	{"site-v2", "typescript", 110, 70, false},
	{"lsystem", "c", 60, 200, true},
}

// demoGarden grows the demo repos, with fake histories whose last event
// lands idleDays before now.
func demoGarden(now time.Time) []live.Repo {
	var out []live.Repo
	for _, r := range demo {
		events := garden.FakeHistory(r.name, r.events, now)
		shift := now.Sub(events[len(events)-1].At) - time.Duration(r.idleDays)*24*time.Hour
		for i := range events {
			events[i].At = events[i].At.Add(shift)
		}
		out = append(out, live.Repo{Name: r.name, Plant: garden.Grow(r.name, garden.SpeciesFor(r.lang), events), Finished: r.finished})
	}
	return out
}
```

- [ ] **Step 6: Update the README**

In `README.md`, replace

```sh
gag                                  # your GitHub repos as a garden
gag replay                           # watch one (fake) plant grow from its history
gag replay -species cactus -name rustyfs -events 300
```

with

```sh
gag                                  # your GitHub repos as a live, animated garden
gag garden --once                    # print one static frame instead (also when piped)
gag replay                           # watch one (fake) plant grow from its history
gag replay -species cactus -name rustyfs -events 300
```

After the paragraph ending in `Use `GAG_COLOR=256` to force 256 colors.`, add:

```markdown
The live view runs until you quit it. Plants sway, clouds drift, the sky
follows your clock, and bees visit healthy flowering plants. If the garden is
wider than the window it pans slowly, and a ticker at the bottom calls out
wilting projects and says how fresh the data is. It reloads from GitHub every
5 minutes (`-refresh`).

Live keys: `q` quit · `r` refresh now · `t` hide/show the ticker · `?` help.
```

In the `## Your GitHub repos` code block, replace the line

```
gag garden -watch 5m                         # always-on display, e.g. on a Pi
```

with

```
gag garden -refresh 2m                       # live view that reloads every 2 minutes
```

- [ ] **Step 7: Run everything**

Run: `go mod tidy && gofmt -l . ; go vet ./... && go test ./... && go build -o gag ./cmd/gag && ./gag garden -demo --once | head -5 && ./gag garden -demo | head -3`
Expected: all tests PASS. Both commands print pixel-art rows: the second pipes into `head`, so stdout isn't a terminal and it prints a static frame instead of opening the live view. `./gag garden -watch 1m` now fails with `flag provided but not defined: -watch`.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "gag opens the live garden; --once prints a static frame

Plain gag and gag garden run the animated live view when stdout is a
terminal, reloading every -refresh (5m). Without a GitHub token it shows
the demo garden with a note instead of failing. -watch is replaced by the
live view.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Real-world check and v0.3.0 release gate

**Files:**
- None, unless the check turns up bugs.

**Interfaces:**
- Consumes: the finished v0.3 build.
- Produces: after the user approves, a merge to `main` and the tag `v0.3.0`.

- [ ] **Step 1: Check the static output on real data**

Run: `go build -o gag ./cmd/gag && GAG_COLOR=truecolor ./gag garden --once -ttl 24h`
Expected: the user's garden with clouds in the sky, plants, pots and labels as in v0.2.

- [ ] **Step 2: Hand the live view to the user**

The live view needs a real terminal, which an agent's shell doesn't have. Ask the user to run `./gag` (or `go run ./cmd/gag`) for a minute or two and check:
- plants sway gently, clouds drift, and bees or butterflies loop around healthy, flowering plants in daylight
- the ticker cycles through wilting plants and shows `updated … ago`
- `r` shows `refreshing…` then `updated just now`, `t` hides the ticker, `?` shows the keys, `q` quits cleanly
- resizing the window redraws to fit, and a tiny window shows the "make the window a bit bigger" message
- with more repos than fit (`./gag garden -limit 12` in a small window), the garden pans every 30 seconds

Fix anything they report, with a failing test first, before continuing.

- [ ] **Step 3: Release gate**

Merging to `main` and tagging publish a public release. Ask the user, and only on an explicit yes, merge the branch to `main`, push, and tag:

```bash
git tag -a v0.3.0 -m "v0.3.0: the garden comes alive"
git push origin main v0.3.0
gh workflow run update-gag.yml -R RursusAeternum/homebrew-tap
```
