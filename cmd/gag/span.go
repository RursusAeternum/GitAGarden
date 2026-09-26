package main

import (
	"strconv"
	"strings"
	"time"
)

// parseSpan is time.ParseDuration plus the units a garden actually cares
// about: d (days) and w (weeks), e.g. "30d", "2w", "1w3d", "36h".
func parseSpan(s string) (time.Duration, error) {
	var total time.Duration
	rest := strings.TrimSpace(s)
	for rest != "" {
		i := strings.IndexAny(rest, "dw")
		if i < 0 {
			d, err := time.ParseDuration(rest)
			return total + d, err
		}
		n, err := strconv.Atoi(rest[:i])
		if err != nil {
			// Not a plain day/week count, e.g. "1.5h": let ParseDuration judge.
			d, err := time.ParseDuration(rest)
			return total + d, err
		}
		unit := 24 * time.Hour
		if rest[i] == 'w' {
			unit *= 7
		}
		total += time.Duration(n) * unit
		rest = rest[i+1:]
	}
	return total, nil
}
