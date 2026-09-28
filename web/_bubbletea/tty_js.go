//go:build js

// Copied into a temporary copy of Bubble Tea v1 by web/build.sh: v1 has no
// js version of its terminal code, only unix and windows. These are the
// windows stubs, less the console calls. In the browser there is no TTY to
// put in raw mode or watch for SIGWINCH; the page wires xterm.js in with
// WithInput/WithOutput and sends WindowSizeMsg itself.

package tea

import (
	"errors"
	"os"
)

func (p *Program) initInput() error { return nil }

func openInputTTY() (*os.File, error) { return nil, errors.New("bubbletea: no TTY in a browser") }

const suspendSupported = false

func suspendProcess() {}

func (p *Program) listenForResize(done chan struct{}) { close(done) }
