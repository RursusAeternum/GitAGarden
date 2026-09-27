package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestStderrProgress(t *testing.T) {
	var buf bytes.Buffer
	p := stderrProgress(&buf)
	p(0, 0, "")
	p(3, 8, "me/garden")
	p(8, 8, "")
	out := buf.String()
	for _, want := range []string{"finding your repos", "3/8", "me/garden"} {
		if !strings.Contains(out, want) {
			t.Errorf("progress output lacks %q: %q", want, out)
		}
	}
	if !strings.HasSuffix(out, "\r\x1b[K") {
		t.Errorf("the finished bar should be erased: %q", out)
	}
}
