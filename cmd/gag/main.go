// Command gag renders your git projects as a garden in the terminal.
//
//	gag replay   time-lapse of one plant growing from its history (default)
//	gag garden   a static garden of demo repos
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/replay"
)

func main() {
	cmd, args := "replay", os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "replay":
		err = runReplay(args)
	case "garden":
		err = runGarden(args)
	default:
		err = fmt.Errorf("unknown command %q (want replay or garden)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gag:", err)
		os.Exit(1)
	}
}

func historyStart(n int) time.Time {
	// Roughly: enough runway that the fake history ends near today.
	return time.Now().Add(-time.Duration(n) * 18 * time.Hour)
}

func parseSpecies(s string) (garden.Species, error) {
	for _, sp := range garden.AllSpecies {
		if string(sp) == s {
			return sp, nil
		}
	}
	return "", fmt.Errorf("unknown species %q", s)
}

func runReplay(args []string) error {
	fs := flag.NewFlagSet("replay", flag.ExitOnError)
	name := fs.String("name", "gag-core", "repo name; seeds the plant's shape")
	species := fs.String("species", "shrub", "shrub, cactus or rosette")
	n := fs.Int("events", 150, "number of fake events to generate")
	decay := fs.Float64("decay", 45, "days of neglect until fully wilted")
	finished := fs.Bool("finished", false, "show the project under glass")
	fs.Parse(args)

	sp, err := parseSpecies(*species)
	if err != nil {
		return err
	}
	gen := func(name string) []garden.Event {
		return garden.FakeHistory(name, *n, historyStart(*n))
	}
	m := replay.New(replay.Config{
		Name:       *name,
		Species:    sp,
		Events:     gen(*name),
		DecayDays:  *decay,
		Finished:   *finished,
		Regenerate: gen,
	})
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

type demoRepo struct {
	name, lang string
	events     int
	idleDays   int
	finished   bool
}

var demo = []demoRepo{
	{"gag-core", "go", 160, 0, false},
	{"rustyfs", "rust", 90, 12, false},
	{"notebook-api", "python", 70, 3, false},
	{"old-blog", "go", 120, 400, true},
	{"dotfiles", "shell", 40, 35, false},
	{"tiny-cli", "rust", 12, 1, false},
	{"site-v2", "typescript", 110, 70, false},
	{"lsystem", "c", 60, 200, true},
}

func runGarden(args []string) error {
	fs := flag.NewFlagSet("garden", flag.ExitOnError)
	decay := fs.Float64("decay", 45, "days of neglect until fully wilted")
	fs.Parse(args)

	width := 100
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	}
	perRow := max(1, (width+1)/(garden.CardWidth+1))

	now := time.Now()
	var cards []string
	for _, r := range demo {
		// Shift the fake history so its last event lands idleDays ago.
		events := garden.FakeHistory(r.name, r.events, now)
		shift := now.Sub(events[len(events)-1].At) - time.Duration(r.idleDays)*24*time.Hour
		for i := range events {
			events[i].At = events[i].At.Add(shift)
		}
		p := garden.Grow(r.name, garden.SpeciesFor(r.lang), events)
		cards = append(cards, garden.Card(p, garden.RenderOpts{Now: now, DecayDays: *decay, Finished: r.finished}))
	}
	for i := 0; i < len(cards); i += perRow {
		row := cards[i:min(i+perRow, len(cards))]
		spaced := make([]string, 0, 2*len(row))
		for j, c := range row {
			if j > 0 {
				spaced = append(spaced, " ")
			}
			spaced = append(spaced, c)
		}
		fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top, spaced...))
		fmt.Println()
	}
	return nil
}
