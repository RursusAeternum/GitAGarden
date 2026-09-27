package live

import "testing"

func TestBar(t *testing.T) {
	cases := []struct {
		done, total, width int
		want               string
	}{
		{3, 8, 8, "███░░░░░"},
		{0, 0, 4, "░░░░"},
		{9, 8, 4, "████"},
		{1, 2, 0, ""},
	}
	for _, c := range cases {
		if got := Bar(c.done, c.total, c.width); got != c.want {
			t.Errorf("Bar(%d, %d, %d) = %q, want %q", c.done, c.total, c.width, got, c.want)
		}
	}
}
