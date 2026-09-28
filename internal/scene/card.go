package scene

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// Card is a detail card: a title row over labelled lines, drawn in a box
// beside the selected plant.
type Card struct {
	Title, Note string // the title row's left and right: the repo, its language and species
	Lines       []CardLine
}

// CardLine is one row of a card.
type CardLine struct {
	Label string // the label column; "" on a line without one
	Sub   bool   // indented to the value column, continuing the line above
	Value string
	Right string // right-aligned after the value, e.g. an age
	Tone  Tone   // the value's colour
}

// Tone colours a card value.
type Tone int

const (
	TonePlain Tone = iota
	ToneGood       // CI passing
	ToneBad        // CI failing
	ToneGold       // the latest release
)

const (
	cardMaxW   = 38 // columns
	cardLabelW = 9  // the label column
)

var (
	cardBG     = rgb(28, 26, 23) // the ticker's background
	cardBorder = rgb(110, 104, 94)
	cardLabel  = rgb(150, 142, 128)
	cardText   = rgb(226, 220, 206)
	cardGood   = rgb(130, 210, 120)
	cardBad    = rgb(235, 100, 90)
	cardGold   = rgb(240, 200, 90)
)

func (t Tone) color() pixel.RGB {
	switch t {
	case ToneGood:
		return cardGood
	case ToneBad:
		return cardBad
	case ToneGold:
		return cardGold
	}
	return cardText
}

// CardRect is where v's card is drawn, in terminal cells. It sits beside the
// selected plant's slot on the side with more room, level with the top of
// its row of beds, and is centred when neither side fits. It always stays
// inside the frame. ok is false when v has no card.
func CardRect(v View) (x, y, w, h int, ok bool) {
	if v.Card == nil {
		return 0, 0, 0, 0, false
	}
	g := geometryOf(v)
	rows := g.h / 2
	w = max(6, min(cardMaxW, g.cols-2))
	h = min(len(v.Card.Lines)+2, rows)
	x = (g.cols - w) / 2
	for b := 0; b < g.lay.Beds; b++ {
		for _, s := range g.slots(v, b) {
			if s.index+1 != v.Selected {
				continue
			}
			left, right := slotLeft(s.cx), slotLeft(s.cx)+BedCols
			roomL, roomR := left, g.cols-right
			switch {
			case roomR >= w && roomR >= roomL:
				x = right
			case roomL >= w:
				x = left - w
			}
			y = max(0, g.bedTop(b)/2)
		}
	}
	x = max(0, min(x, g.cols-w))
	y = max(0, min(y, rows-h))
	return x, y, w, h, true
}

// drawCard draws v's card over the frame, at CardRect.
func drawCard(c *pixel.Canvas, v View) {
	x, y, w, h, ok := CardRect(v)
	if !ok || h < 2 {
		return
	}
	c.Rect(x, 2*y, w, 2*h, cardBG)
	room := w - 4
	note := cut(cardRunes(v.Card.Note), room/2)
	noteText := ""
	if len(note) > 0 {
		room -= len(note) + 1 // a space before the note
		noteText = " " + string(note)
	}
	title := cut(cardRunes(v.Card.Title), max(0, room-2)) // then a space and at least one dash
	cardRow(c, x, y,
		seg{"┌ ", cardBorder}, seg{string(title), cardText},
		seg{" " + strings.Repeat("─", max(0, room-len(title)-1)), cardBorder},
		seg{noteText, cardLabel}, seg{" ┐", cardBorder})
	lines := v.Card.Lines
	if len(lines) > h-2 {
		lines = lines[:h-2]
	}
	for i, ln := range lines {
		drawCardLine(c, x, y+1+i, w, ln)
	}
	c.Text(x, y+h-1, "└"+strings.Repeat("─", w-2)+"┘", cardBorder)
}

// drawCardLine draws one line of a w-wide card on terminal row row.
func drawCardLine(c *pixel.Canvas, x, row, w int, ln CardLine) {
	room := w - 4
	var label []rune
	if ln.Label != "" || ln.Sub {
		n := min(cardLabelW, room)
		label = pad(cut(cardRunes(ln.Label), n-1), n)
	}
	room -= len(label)
	right := cut(cardRunes(ln.Right), room)
	vroom := room - len(right)
	if len(right) > 0 {
		vroom-- // a space before the right part
	}
	value := cut(cardRunes(ln.Value), max(0, vroom))
	cardRow(c, x, row,
		seg{"│ ", cardBorder}, seg{string(label), cardLabel}, seg{string(value), ln.Tone.color()},
		seg{strings.Repeat(" ", max(0, room-len(value)-len(right))), cardText},
		seg{string(right), cardText}, seg{" │", cardBorder})
}

// seg is a run of card text in one colour.
type seg struct {
	text string
	col  pixel.RGB
}

// cardRow writes segs one after another along terminal row row, from
// column x.
func cardRow(c *pixel.Canvas, x, row int, segs ...seg) {
	for _, s := range segs {
		c.Text(x, row, s.text, s.col)
		x += utf8.RuneCountInString(s.text)
	}
}

// cardRunes cleans text for the card. Control characters go (a tab becomes
// a space), and runes that don't take exactly one terminal column become
// '?', so they can't shift the row.
func cardRunes(s string) []rune {
	var out []rune
	for _, r := range s {
		switch {
		case r == '\t':
			out = append(out, ' ')
		case unicode.IsControl(r):
		case runewidth.RuneWidth(r) != 1:
			out = append(out, '?')
		default:
			out = append(out, r)
		}
	}
	return out
}

// cut shortens r to n runes, ending in '…' when it had to cut.
func cut(r []rune, n int) []rune {
	if len(r) <= n {
		return r
	}
	if n <= 0 {
		return nil
	}
	return append(r[:n-1:n-1], '…')
}

// pad fills r with spaces up to n runes.
func pad(r []rune, n int) []rune {
	for len(r) < n {
		r = append(r, ' ')
	}
	return r
}
