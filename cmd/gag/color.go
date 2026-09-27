package main

import (
	"os"

	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// colorProfile uses true color when the terminal advertises it.
// GAG_COLOR=truecolor or 256 overrides detection, for terminals that
// support true color without saying so.
func colorProfile() pixel.Profile {
	switch os.Getenv("GAG_COLOR") {
	case "truecolor", "24bit":
		return pixel.TrueColor
	case "256":
		return pixel.ANSI256
	}
	if termenv.NewOutput(os.Stdout).EnvColorProfile() == termenv.TrueColor {
		return pixel.TrueColor
	}
	return pixel.ANSI256
}

func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 100
}
