// Package scene draws the world around the plants: sky, ground, pots and
// glass, and lays plants out in garden beds.
package scene

import "github.com/RursusAeternum/GitAGarden/internal/pixel"

func rgb(r, g, b uint8) pixel.RGB { return pixel.RGB{R: r, G: g, B: b} }

// noise is a stable pseudo-random value in [0, 1) per seed and position,
// used for texture so frames don't flicker.
func noise(seed int64, x, y int) float64 {
	h := uint64(seed) ^ uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xC2B2AE3D27D4EB4F
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	return float64(h>>11) / float64(1<<53)
}
