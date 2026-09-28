package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/live"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func TestDemoWorldActsOutEveryKind(t *testing.T) {
	now := time.Now()
	w := newDemoWorld(now)
	prev := w.snapshot(now)
	var kinds []scene.ReactKind
	var agents []string
	for i := 0; i < 14; i++ {
		now = now.Add(demoRefresh)
		next := w.snapshot(now)
		changes := live.ChangesBetween(prev.Repos, next.Repos)
		if len(changes) != 1 {
			t.Fatalf("step %d made %d changes: %+v", i, len(changes), changes)
		}
		kinds = append(kinds, changes[0].Kind)
		if changes[0].Kind == scene.ReactPush {
			agents = append(agents, changes[0].Agent)
		}
		prev = next
	}
	cycle := []scene.ReactKind{scene.ReactPush, scene.ReactMerge, scene.ReactRelease, scene.ReactWeedIn,
		scene.ReactWeedOut, scene.ReactStorm, scene.ReactClear}
	if want := append(append([]scene.ReactKind{}, cycle...), cycle...); !reflect.DeepEqual(kinds, want) {
		t.Errorf("kinds = %v, want %v", kinds, want)
	}
	if !reflect.DeepEqual(agents, []string{"", "Claude"}) {
		t.Errorf("the demo's pushes were by %q; want a person, then Claude", agents)
	}
}

func TestDemoWorldStartsAsTheDemo(t *testing.T) {
	now := time.Now()
	first, still := newDemoWorld(now).snapshot(now), demoGarden(now)
	if !first.Demo || len(first.Repos) != len(still) {
		t.Fatalf("first snapshot: demo %v, %d repos", first.Demo, len(first.Repos))
	}
	for i := range still {
		if first.Repos[i].Name != still[i].Name || first.Repos[i].Plant.Pushes != still[i].Plant.Pushes {
			t.Errorf("repo %d: %s with %d pushes, want %s with %d", i, first.Repos[i].Name,
				first.Repos[i].Plant.Pushes, still[i].Name, still[i].Plant.Pushes)
		}
	}
}

func TestNoTokenFallbackStaysStill(t *testing.T) {
	now := time.Now()
	for _, st := range []*sourceState{{}, nil} { // a live session, and one-shot use
		first := st.fallback(now)
		if changes := live.ChangesBetween(first, st.fallback(now.Add(5*time.Minute))); st != nil && len(changes) != 0 {
			t.Errorf("the no-token garden changed between loads: %+v", changes)
		}
		if len(first) != len(demo) {
			t.Errorf("the fallback shows %d repos, want the %d demo ones", len(first), len(demo))
		}
	}
}
