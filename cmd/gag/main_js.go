//go:build js

package main

import (
	"context"
	"io"
	"syscall/js"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/live"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
	"github.com/RursusAeternum/GitAGarden/internal/replay"
)

// main is the browser demo: the demo garden or a replay, drawn into the
// xterm.js terminal on web/index.html. There is no GitHub token in a
// browser, so the garden is always the fake one.
//
// The page sets window.gag = {mode, cols, rows, write(bytes), exited()}
// before starting this; main adds gag.input(string) for keystrokes and
// gag.resize(cols, rows).
func main() {
	page := js.Global().Get("gag")
	// Nothing can be detected through xterm, and it does true color.
	lipgloss.SetColorProfile(termenv.TrueColor)

	var m tea.Model
	if page.Get("mode").String() == "replay" {
		n := 150
		cfg := replay.Config{Name: "gag-core", Species: garden.Shrub, DecayDays: 45, Profile: pixel.TrueColor}
		cfg.Regenerate = func(name string) []garden.Event {
			return garden.FakeHistory(name, n, historyStart(n))
		}
		cfg.Events = cfg.Regenerate(cfg.Name)
		m = replay.New(cfg)
	} else {
		// Not source.snapshot: that reaches the GitHub client, and net/http
		// with it, which doubles the size of the wasm.
		world := newDemoWorld(time.Now())
		load := func(context.Context, func(live.Progress)) (live.Snapshot, error) {
			return world.snapshot(time.Now()), nil
		}
		m = live.New(live.Config{Load: load, Refresh: demoRefresh, DecayDays: 45, Profile: pixel.TrueColor})
	}

	in, keys := io.Pipe()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(in),
		tea.WithOutput(xtermWriter{page.Get("write")}), tea.WithoutSignalHandler())

	// Callbacks run on the JS event loop and must not block it, so both
	// hand off to goroutines.
	page.Set("input", js.FuncOf(func(_ js.Value, args []js.Value) any {
		b := []byte(args[0].String())
		go keys.Write(b)
		return nil
	}))
	resize := func(cols, rows int) { go p.Send(tea.WindowSizeMsg{Width: cols, Height: rows}) }
	page.Set("resize", js.FuncOf(func(_ js.Value, args []js.Value) any {
		resize(args[0].Int(), args[1].Int())
		return nil
	}))
	resize(page.Get("cols").Int(), page.Get("rows").Int()) // no TTY to ask

	if _, err := p.Run(); err != nil {
		js.Global().Get("console").Call("error", "gag: "+err.Error())
	}
	page.Call("exited")
}

// xtermWriter hands Bubble Tea's output to the page's write(bytes).
type xtermWriter struct{ write js.Value }

func (w xtermWriter) Write(b []byte) (int, error) {
	arr := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(arr, b)
	w.write.Invoke(arr)
	return len(b), nil
}
