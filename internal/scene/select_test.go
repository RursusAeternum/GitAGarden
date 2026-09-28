package scene

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// litBy lists the pixels that change when plot i is selected in v.
func litBy(v View, i int) [][2]int {
	plain := Draw(v)
	v.Selected = i + 1
	lit := Draw(v)
	var out [][2]int
	for y := 0; y < plain.H; y++ {
		for x := 0; x < plain.W; x++ {
			if plain.At(x, y) != lit.At(x, y) {
				out = append(out, [2]int{x, y})
			}
		}
	}
	return out
}

func TestSelectionLightsItsGroundStrip(t *testing.T) {
	v := View{Cols: 3*BedCols + 10, Plots: demoPlots(), Now: at(14, 0), Seed: 1}
	for i := range v.Plots {
		px := litBy(v, i)
		if len(px) == 0 {
			t.Fatalf("selecting plot %d changed nothing", i)
		}
		for _, p := range px {
			if got, ok := PlotAt(v, p[0], p[1]/2); !ok || got != i {
				t.Fatalf("plot %d lit pixel %v, but PlotAt there says %d, %v", i, p, got, ok)
			}
			if p[1]%bedPx < groundTop {
				t.Fatalf("plot %d lit pixel %v, above its ground strip", i, p)
			}
		}
	}
	plain := Draw(v).Encode(pixel.TrueColor)
	v.Selected = 2
	lit := Draw(v).Encode(pixel.TrueColor)
	if white := "38;2;255;255;255m"; strings.Count(lit, white) <= strings.Count(plain, white) {
		t.Error("the selected plant's name label should turn white")
	}
}

// nameLine is the frame's text row holding name, and the column name starts at.
func nameLine(t *testing.T, v View, name string) (string, int) {
	t.Helper()
	for _, line := range strings.Split(visible(Draw(v).Encode(pixel.TrueColor)), "\n") {
		if i := strings.Index(line, name); i >= 0 {
			return line, utf8.RuneCountInString(line[:i])
		}
	}
	t.Fatalf("no row shows %q", name)
	return "", 0
}

func TestSelectedNameIsMarked(t *testing.T) {
	v := View{Cols: 3*BedCols + 10, Plots: demoPlots(), Now: at(14, 0), Seed: 1}
	name := v.Plots[1].Name
	_, col := nameLine(t, v, name)
	v.Selected = 2
	line, litCol := nameLine(t, v, name)
	if !strings.Contains(line, "▸ "+name+" ◂") {
		t.Errorf("selected name row = %q, want %q in it", line, "▸ "+name+" ◂")
	}
	if litCol != col {
		t.Errorf("the selected name moved from column %d to %d", col, litCol)
	}
	text := visible(Draw(v).Encode(pixel.TrueColor))
	if strings.Count(text, "▸") != 1 || strings.Count(text, "◂") != 1 {
		t.Error("only the selected plant should be marked")
	}
}

func TestLongSelectedNameIsTrimmedInsideItsMarks(t *testing.T) {
	plots := demoPlots()[:1]
	plots[0].Name = strings.Repeat("x", 30)
	v := View{Cols: BedCols, Plots: plots, Now: at(14, 0), Seed: 1, Selected: 1}
	want := "▸ " + strings.Repeat("x", BedCols-7) + "… ◂"
	if line, _ := nameLine(t, v, "▸"); !strings.Contains(line, want) {
		t.Errorf("long selected name row = %q, want %q in it", line, want)
	}
}

func TestPlotAtFollowsThePanningCamera(t *testing.T) {
	v := View{Cols: 80, Rows: 23, Plots: manyPlots(8), Now: at(14, 0), Seed: 1, Pan: 2.5}
	lit := 0
	for i := range v.Plots {
		for _, p := range litBy(v, i) {
			lit++
			if got, ok := PlotAt(v, p[0], p[1]/2); !ok || got != i {
				t.Fatalf("mid-pan, plot %d lit pixel %v, but PlotAt there says %d, %v", i, p, got, ok)
			}
		}
	}
	if lit == 0 {
		t.Fatal("no plant on screen lit up")
	}
	if _, ok := PlotAt(v, 10, 23); ok {
		t.Error("a row below the frame holds no plant")
	}
}

func TestPlotAtIsEmptyBesideTheSlots(t *testing.T) {
	v := View{Cols: 3*BedCols + 10, Rows: 30, Plots: demoPlots(), Now: at(14, 0), Seed: 1} // 5-column margins
	for _, col := range []int{0, 4, 3*BedCols + 5, 3*BedCols + 9, -1, 3*BedCols + 10} {
		if i, ok := PlotAt(v, col, 20); ok {
			t.Errorf("column %d holds plot %d; want open ground", col, i)
		}
	}
	if i, ok := PlotAt(v, 5, 20); !ok || i != 0 {
		t.Errorf("column 5 = %d, %v; want the first plant", i, ok)
	}
	if i, ok := PlotAt(v, 5, 1); !ok || i != 0 {
		t.Errorf("the sky above the top bed = %d, %v; want the first plant's", i, ok)
	}
}

func TestOnScreenAndReadingOrder(t *testing.T) {
	cases := []struct {
		v    View
		want []int
	}{
		{View{Cols: 100, Plots: manyPlots(3)}, []int{0, 1, 2}},
		{View{Cols: 80, Rows: 23, Plots: manyPlots(8), Pan: 2}, []int{2, 3, 4}},
		{View{Cols: 80, Rows: 23, Plots: manyPlots(8), Pan: 7}, []int{7, 0, 1}},
		{View{Cols: 80, Plots: manyPlots(5)}, []int{0, 1, 2, 3, 4}},
	}
	for _, c := range cases {
		if got := OnScreen(c.v); !reflect.DeepEqual(got, c.want) {
			t.Errorf("OnScreen(%d cols, pan %v) = %v, want %v", c.v.Cols, c.v.Pan, got, c.want)
		}
	}
	panning := Layout{PerRow: 3, Beds: 2, Columns: 4, Overflow: true}
	if got, want := ReadingOrder(panning, 7), []int{0, 2, 4, 6, 1, 3, 5}; !reflect.DeepEqual(got, want) {
		t.Errorf("ReadingOrder(panning) = %v, want %v", got, want)
	}
	if got, want := ReadingOrder(Layout{PerRow: 3, Beds: 2, Columns: 3}, 5), []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(got, want) {
		t.Errorf("ReadingOrder(fitting) = %v, want %v", got, want)
	}
}

func TestCameraHelpers(t *testing.T) {
	for _, c := range []struct {
		pan            float64
		col, per, cols int
		want           float64
	}{
		{0, 1, 3, 8, 0},   // already on screen
		{0, 7, 3, 8, 7},   // one step left, round the ring
		{0, 4, 3, 8, 2},   // two steps right
		{2.6, 1, 3, 8, 1}, // mid-slide: nearest from where the camera is
		{5, 0, 3, 3, 0},   // the garden fits: no panning
	} {
		if got := PanShowing(c.pan, c.col, c.per, c.cols); got != c.want {
			t.Errorf("PanShowing(%v, %d, %d, %d) = %v, want %v", c.pan, c.col, c.per, c.cols, got, c.want)
		}
	}
	for _, c := range []struct{ from, to, f, want float64 }{
		{7, 1, 1, 1}, {7, 1, 0.5, 0}, {1, 7, 0.5, 0}, {0, 3, 0, 0}, {0, 3, 2, 3},
	} {
		if got := SlidePan(c.from, c.to, c.f, 8); got != c.want {
			t.Errorf("SlidePan(%v → %v, %v) = %v, want %v", c.from, c.to, c.f, got, c.want)
		}
	}
	for col := 0; col < 8; col++ {
		if got := PanAt(ResumeAt(col), 8); got != float64(col) || Sliding(ResumeAt(col)) {
			t.Errorf("PanAt(ResumeAt(%d)) = %v, sliding %v; want resting on %d", col, got, Sliding(ResumeAt(col)), col)
		}
	}
}
