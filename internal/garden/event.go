package garden

import "time"

// EventKind is something that happened to a repo. Every tending event adds
// a little something to the plant; issues add or clear weeds.
type EventKind int

const (
	Push EventKind = iota
	Merge
	Release
	IssueOpened
	IssueClosed
)

func (k EventKind) String() string {
	switch k {
	case Push:
		return "push"
	case Merge:
		return "merge"
	case Release:
		return "release"
	case IssueOpened:
		return "issue opened"
	case IssueClosed:
		return "issue closed"
	}
	return "unknown"
}

// Icon is a one-cell marker used in event logs.
func (k EventKind) Icon() string {
	switch k {
	case Push:
		return "•"
	case Merge:
		return "✿"
	case Release:
		return "★"
	case IssueOpened:
		return "!"
	case IssueClosed:
		return "✓"
	}
	return "?"
}

// Tends reports whether the event counts as the owner tending the plant.
// Someone else opening an issue doesn't mean you watered anything.
func (k EventKind) Tends() bool {
	return k != IssueOpened
}

type Event struct {
	Kind EventKind
	At   time.Time
	Note string
}
