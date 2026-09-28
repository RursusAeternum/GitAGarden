package replay

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func step(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

func key(s string) tea.KeyMsg {
	if s == " " {
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func newTestModel() Model {
	events := garden.FakeHistory("repo", 120, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	return New(Config{Name: "repo", Species: garden.Shrub, Events: events,
		Regenerate: func(n string) []garden.Event {
			return garden.FakeHistory(n, 50, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		}})
}

func TestPlaysThroughToTheEnd(t *testing.T) {
	m := newTestModel()
	for i := 0; i < 5000 && m.playing; i++ {
		m = step(m, tickMsg{})
		if i%250 == 0 && !strings.Contains(m.View(), "GAG · replay") {
			t.Fatal("view missing header")
		}
	}
	if m.playing || m.applied != len(m.cfg.Events) || !m.clock.Equal(m.end) {
		t.Fatalf("playing=%v applied=%d/%d", m.playing, m.applied, len(m.cfg.Events))
	}
}

func TestSteppingKeys(t *testing.T) {
	m := step(newTestModel(), key(" ")) // pause
	for i := 0; i < 3; i++ {
		m = step(m, key("l"))
	}
	if m.applied != 3 {
		t.Fatalf("after 3 steps applied=%d", m.applied)
	}
	m = step(m, key("h"))
	if m.applied != 2 {
		t.Fatalf("after stepping back applied=%d", m.applied)
	}
	for _, k := range []string{"s", "f", "+", "-", "G", "n", "r"} {
		m = step(m, key(k))
		_ = m.View()
	}
	if m.cfg.Name != "repo-1" || len(m.cfg.Events) != 50 {
		t.Fatalf("regenerate: name=%q events=%d", m.cfg.Name, len(m.cfg.Events))
	}
}

func TestReplaySkyIgnoresTheReplayClock(t *testing.T) {
	// The replay clock races hours per frame; drawing the sky from it strobes
	// day and night. The sky must come from Config.Now instead.
	fixed := time.Date(2026, 6, 1, 12, 0, 0, 0, time.Local)
	m := newTestModel()
	m.cfg.Now = func() time.Time { return fixed }
	m.seek(time.Date(2026, 1, 1, 0, 30, 0, 0, time.Local)) // replay clock at night
	night := strings.SplitN(m.View(), "\n", 2)[0]
	m.seek(time.Date(2026, 1, 1, 12, 30, 0, 0, time.Local)) // replay clock at noon
	noon := strings.SplitN(m.View(), "\n", 2)[0]
	if night != noon {
		t.Error("sky changed with the replay clock")
	}
}

func TestReplayDrawsTheConfiguredSky(t *testing.T) {
	night := time.Date(2026, 6, 1, 23, 0, 0, 0, time.Local)
	frame := func(stars int) string {
		m := newTestModel()
		m.cfg.Now = func() time.Time { return night }
		m.cfg.Sky, m.cfg.Stars = scene.SkyStars, stars
		return m.View()
	}
	if frame(0) == frame(40) {
		t.Error("replay ignores the star sky setting")
	}
}
