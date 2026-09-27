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
