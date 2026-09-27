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

func TestOnlyTheTopBedHasASun(t *testing.T) {
	c := Compose(BedCols, demoPlots()[:2], at(12, 0), 1) // 2 plots, 1 per row: 2 beds
	suns := func(y0, y1 int) int {
		n := 0
		for y := y0; y < y1; y++ {
			for x := 0; x < c.W; x++ {
				if c.At(x, y) == sunColor {
					n++
				}
			}
		}
		return n
	}
	if suns(0, bedPx) == 0 {
		t.Error("top bed has no sun at noon")
	}
	if n := suns(bedPx, 2*bedPx); n != 0 {
		t.Errorf("second bed has %d sun pixels, want none", n)
	}
}
