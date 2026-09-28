package scene

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func sampleCard() *Card {
	return &Card{Title: "gag-core", Note: "Go · shrub", Lines: []CardLine{
		{Value: "RursusAeternum/gag-core", Right: "★ 31"},
		{Label: "state", Value: "thriving · tended 2h ago"},
		{Label: "last", Value: "Make storms readable, keep CI errors local"},
		{Label: "CI", Value: "✓ passing on main", Tone: ToneGood},
		{Label: "PRs", Value: "2 open · oldest 10d"},
		{Sub: true, Value: "#12 Add sparkline", Right: "2d"},
		{Label: "release", Value: "v0.5.0 · 1h ago", Tone: ToneGold},
	}}
}

// frameLines is v's frame as plain text rows.
func frameLines(v View) []string {
	return strings.Split(visible(Draw(v).Encode(pixel.TrueColor)), "\n")
}

func TestCardStaysInsideTheWindow(t *testing.T) {
	for _, size := range [][2]int{{80, 23}, {24, 11}, {240, 64}} {
		plots := manyPlots(8)
		for i := range plots {
			v := View{Cols: size[0], Rows: size[1], Plots: plots, Now: at(14, 0), Seed: 1, Selected: i + 1, Card: sampleCard()}
			x, y, w, h, ok := CardRect(v)
			if !ok || x < 0 || y < 0 || x+w > size[0] || y+h > size[1] || w < 6 || h < 3 {
				t.Fatalf("%dx%d, plot %d: card at %d,%d size %dx%d", size[0], size[1], i, x, y, w, h)
			}
			lines := frameLines(v)
			if len(lines) != size[1] {
				t.Fatalf("%dx%d: %d rows", size[0], size[1], len(lines))
			}
			for n, l := range lines {
				if got := runewidth.StringWidth(l); got != size[0] {
					t.Fatalf("%dx%d, plot %d: row %d is %d wide", size[0], size[1], i, n, got)
				}
			}
			top, bottom := []rune(lines[y]), []rune(lines[y+h-1])
			if top[x] != '┌' || top[x+w-1] != '┐' || bottom[x] != '└' || bottom[x+w-1] != '┘' {
				t.Fatalf("%dx%d, plot %d: no box at %d,%d %dx%d:\n%s", size[0], size[1], i, x, y, w, h, strings.Join(lines, "\n"))
			}
		}
	}
}

func TestCardSitsBesideThePlant(t *testing.T) {
	v := View{Cols: 120, Plots: demoPlots(), Now: at(14, 0), Seed: 1, Card: sampleCard()} // slots 21-46, 47-72, 73-98
	v.Selected = 1
	if x, _, _, _, _ := CardRect(v); x < 47 {
		t.Errorf("the first plant's card starts at column %d, over the plant", x)
	}
	v.Selected = 3
	if x, _, w, _, _ := CardRect(v); x+w > 73 {
		t.Errorf("the last plant's card ends at column %d, over the plant", x+w)
	}
	v.Cols, v.Selected = 80, 2 // neither side of the middle plant has room
	if x, _, w, _, _ := CardRect(v); x != (80-w)/2 {
		t.Errorf("the card should be centred when no side fits; it starts at column %d", x)
	}
	two := View{Cols: 3 * BedCols, Plots: manyPlots(5), Now: at(14, 0), Seed: 1, Card: sampleCard(), Selected: 4}
	if _, y, _, _, _ := CardRect(two); y != BedRows {
		t.Errorf("a card for a plant in the second bed starts at row %d, want %d", y, BedRows)
	}
}

func TestCardTextIsOneColumnWide(t *testing.T) {
	card := &Card{Title: "日本語のリポジトリ🌱", Note: "Go · shrub", Lines: []CardLine{
		{Label: "last", Value: "fix:\tthe 🐛 in 中文 \x1b[31mred\x1b[0m"},
		{Label: "PRs", Value: strings.Repeat("a very long title ", 10), Right: "10d"},
		{Label: "a-very-long-label", Value: "x", Right: strings.Repeat("r", 60)},
	}}
	v := View{Cols: 80, Rows: 23, Plots: demoPlots(), Now: at(14, 0), Seed: 1, Selected: 1, Card: card}
	for n, l := range frameLines(v) {
		for _, r := range l {
			if runewidth.RuneWidth(r) != 1 {
				t.Fatalf("row %d holds %q, %d columns wide", n, r, runewidth.RuneWidth(r))
			}
		}
		if w := runewidth.StringWidth(l); w != 80 {
			t.Fatalf("row %d is %d wide", n, w)
		}
	}
}

func TestTallCardsAreCut(t *testing.T) {
	card := &Card{Title: "t"}
	for i := 0; i < 30; i++ {
		card.Lines = append(card.Lines, CardLine{Label: "x", Value: "y"})
	}
	v := View{Cols: 80, Rows: 11, Plots: demoPlots(), Now: at(14, 0), Seed: 1, Selected: 1, Card: card}
	if _, y, _, h, _ := CardRect(v); y != 0 || h != 11 {
		t.Errorf("a card taller than the window is at row %d, %d tall; want 0 and 11", y, h)
	}
	if lines := frameLines(v); !strings.Contains(lines[10], "└") {
		t.Error("a cut card keeps its bottom border")
	}
}
