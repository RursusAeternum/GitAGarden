package garden

import (
	"fmt"
	"hash/fnv"
	"math"
	"time"
)

const defaultDecayDays = 45

// Health is 1 for a recently tended plant, falling to 0 after DecayDays of
// neglect (with a two-day grace period).
func Health(p *Plant, now time.Time, decayDays float64) float64 {
	if p.LastTended.IsZero() {
		return 1
	}
	if decayDays <= 0 {
		decayDays = defaultDecayDays
	}
	idle := now.Sub(p.LastTended).Hours()/24 - 2
	if idle <= 0 {
		return 1
	}
	return math.Max(0, 1-idle/decayDays)
}

// hash01 gives a stable pseudo-random value per cell, so the same leaves fall
// first every time rather than flickering between frames.
func hash01(name string, x, y int) float64 {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s:%d:%d", name, x, y)
	return float64(h.Sum32()) / math.MaxUint32
}
