package main

import (
	"fmt"
	"os"

	"github.com/RursusAeternum/GitAGarden/internal/config"
)

// runConfig prints where the config file lives and the settings in effect.
func runConfig() error {
	path := config.Path()
	_, err := os.Stat(path)
	fmt.Print(loadConfig().Describe(path, err == nil))
	return nil
}
