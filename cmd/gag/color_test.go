package main

import (
	"testing"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func TestColorProfileOverride(t *testing.T) {
	t.Setenv("GAG_COLOR", "truecolor")
	if colorProfile() != pixel.TrueColor {
		t.Error("GAG_COLOR=truecolor should force true color")
	}
	t.Setenv("GAG_COLOR", "256")
	if colorProfile() != pixel.ANSI256 {
		t.Error("GAG_COLOR=256 should force 256 colors")
	}
}
