package main

import (
	"fmt"
	"io"
	"os"

	"github.com/RursusAeternum/GitAGarden/internal/config"
)

// runConfig writes where the config file lives and the settings in effect.
func runConfig(w io.Writer) error {
	path := config.Path()
	_, err := os.Stat(path)
	_, werr := fmt.Fprint(w, loadConfig().Describe(path, err == nil))
	return werr
}
