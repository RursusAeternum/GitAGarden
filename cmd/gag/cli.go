//go:build !js

package main

import (
	"fmt"
	"os"
	"strings"
)

// main is the command line. The browser build has its own, in main_js.go.
func main() {
	cmd, args := "garden", os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "replay":
		err = runReplay(args)
	case "garden":
		err = runGarden(args)
	case "config":
		err = runConfig(os.Stdout)
	case "version", "--version", "-v":
		fmt.Println("gag", version)
	default:
		err = fmt.Errorf("unknown command %q (want garden, replay, config or version)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gag:", err)
		os.Exit(1)
	}
}
