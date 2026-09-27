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
