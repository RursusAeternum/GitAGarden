package scene

import (
	"testing"
	"unicode"

	"github.com/mattn/go-runewidth"
)

func TestCardDropsZeroWidthRunes(t *testing.T) {
	card := &Card{Title: "♻️ gag", Note: "Go · shrub", Lines: []CardLine{
		{Label: "last", Value: "♻️ Refactor the pan"},
		{Label: "flag", Value: "🏴\U000E0067\U000E0062\U000E0073\U000E0063\U000E0074\U000E007F flag"},
		{Label: "accent", Value: "café and 👨‍💻, a​b"},
	}}
	v := View{Cols: 80, Rows: 23, Plots: demoPlots(), Now: at(14, 0), Seed: 1, Selected: 1, Card: card}
	for n, l := range frameLines(v) {
		for _, r := range l {
			if unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Variation_Selector) {
				t.Fatalf("row %d holds zero-width rune %U", n, r)
			}
		}
		if w := runewidth.StringWidth(l); w != 80 {
			t.Fatalf("row %d is %d wide", n, w)
		}
	}
}
