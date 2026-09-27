# Star Sky, Shooting Stars and Config File Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the night sky show the garden's real GitHub stars (`sky = stars`), mark new stars with shooting stars for everyone, and give GAG a config file at `~/.config/gag/config` with a `gag config` command.

**Architecture:** A new pure package, `internal/config`, parses the file into typed settings with a source per key; `cmd/gag` merges it under the flags. `internal/github` reads `stargazerCount` in the repo listing it already makes. `internal/scene` gains a star-sky mode, where star *i*'s place depends only on *i*, and shooting stars. Both are pure functions of the `View`. `internal/live` compares star counts between online loads, queues shooting stars, and adds ticker notes.

**Tech Stack:** Go 1.26 standard library (bufio, strconv, filepath), Bubble Tea v1.3, go-runewidth.

**Spec:** `docs/superpowers/specs/2026-09-28-star-sky-and-config-design.md`. It builds on v0.4.0 as shipped, and ships as v0.5.0; the living-garden "Reactions" phase follows as v0.6.

## Global Constraints

- Module `github.com/RursusAeternum/GitAGarden`, `go 1.26.0`. Go is at `~/sdk/go/bin`: run `export PATH=$HOME/sdk/go/bin:$PATH` before any `go` command.
- No new third-party modules. The config parser is hand-written.
- The random sky stays the default and must look exactly as before: `internal/scene/testdata/garden.golden` passes unchanged in every task.
- Config problems never stop GAG. Warnings go to stderr **before** the live view opens, and nothing is printed while it is open.
- Config path: `$XDG_CONFIG_HOME/gag/config` if set, else `~/.config/gag/config`, on every OS.
- Precedence: a flag given on the command line, then the config file, then the defaults.
- Shooting stars are on for everyone, whatever the sky setting, and happen only in the live view.
- Ticker icons are two-cell default-emoji glyphs, and ⭐ joins them.
- Frame budget: a 240×65 live frame renders in under 10 ms. The frame rate is fast (125 ms) while a shooting star flies.
- After every task, `go vet ./... && go test ./...` passes and `gofmt -l .` prints nothing.
- Every commit message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Messy config files**: typos, bad values, Windows line endings, a byte-order mark from some editors, keys in other cases. GAG is never stopped, each bad line gets one warning with its line number, and the good lines still apply. Pinned by Task 1 `TestParse` and `TestParseWarnsAndCarriesOn`.
2. **Extreme star counts**: a repo with 50,000 stars, or none at all. The sky stays capped at its capacity, drawing stays fast, and brightness stays in range; 0 gives an empty night sky. Pinned by Task 3 `TestStarSkyCapsAndBrightens` and `TestStarSkyOnlyAtNightAndOnlyStars`.
3. **Stars that shouldn't count**: an unstar, a repo leaving or joining the garden, an offline reload. None of them produce shooting stars or negative notes. Pinned by Task 5 `TestStarsThatDontCountDontShoot`.
4. **Stars arriving across many refreshes, or with `r` pressed repeatedly**: at most 3 shooting stars per refresh, landed ones are pruned, and notes expire after a minute. Pinned by Task 5 `TestNewStarsShootAndGetNoted` and `TestShotsAndNotesExpire`.
5. **Where the config file lives**: with `XDG_CONFIG_HOME` set, unset, or empty. The path is predictable, and a missing file silently gives the defaults. Pinned by Task 1 `TestPath` and `TestLoadMissingFileIsDefaults`.

---

### Task 1: `internal/config`: parse `~/.config/gag/config`

**Files:**
- Create: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Source string` with `const Default Source = "default"; File Source = "file"`
  - `type Config struct { Sky string; Limit int; Repos []string; User string; Refresh time.Duration; Decay float64; From map[string]Source }`
  - `var Keys = []string{"sky", "limit", "repos", "user", "refresh", "decay"}`
  - `func Defaults() Config` (sky `random`, limit 8, refresh 5m, decay 45)
  - `type Warning struct{ Line int; Msg string }` with `String()`, which gives `config line N: …`, or `config: …` when there's no line
  - `func Path() string`
  - `func Load(path string) (Config, []Warning)`
  - `func Parse(r io.Reader) (Config, []Warning)`
  - `func (c Config) Describe(path string, exists bool) string`

- [ ] **Step 1: Write the failing tests**

`internal/config/config_test.go`:

```go
package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	src := "﻿# my garden\r\nsky = Stars  # opt in\r\n\r\nLIMIT=12\nrepos = me/a, other/b\nuser = octocat\nrefresh = 2m\ndecay = 30\n"
	c, warns := Parse(strings.NewReader(src))
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	want := Config{Sky: "stars", Limit: 12, Repos: []string{"me/a", "other/b"}, User: "octocat",
		Refresh: 2 * time.Minute, Decay: 30}
	got := c
	got.From = nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	for _, k := range Keys {
		if c.From[k] != File {
			t.Errorf("%s came from %q, want file", k, c.From[k])
		}
	}
}

func TestParseWarnsAndCarriesOn(t *testing.T) {
	src := strings.Join([]string{
		"sky = sparkly",
		"limit = 0",
		"refresh = 5",
		"decay = -1",
		"colour = green",
		"just words",
		"repos = a/b, nope",
		"user = two words",
		"limit = 3",
	}, "\n")
	c, warns := Parse(strings.NewReader(src))
	if len(warns) != 8 {
		t.Fatalf("got %d warnings, want 8: %v", len(warns), warns)
	}
	for i, w := range warns {
		if w.Line != i+1 {
			t.Errorf("warning %d is for line %d, want %d", i, w.Line, i+1)
		}
	}
	if got, want := warns[0].String(), `config line 1: sky "sparkly" isn't random or stars; using random`; got != want {
		t.Errorf("warning text = %q, want %q", got, want)
	}
	if c.Sky != "random" || c.From["sky"] != Default {
		t.Errorf("a bad sky should keep the default, got %q from %q", c.Sky, c.From["sky"])
	}
	if c.Limit != 3 || c.From["limit"] != File {
		t.Errorf("a later good line should still apply: limit %d from %q", c.Limit, c.From["limit"])
	}
	if c.Repos != nil {
		t.Errorf("a repos line with a bad entry should be ignored, got %v", c.Repos)
	}
}

func TestLoadMissingFileIsDefaults(t *testing.T) {
	c, warns := Load(filepath.Join(t.TempDir(), "nope"))
	if len(warns) != 0 {
		t.Errorf("a missing file should not warn: %v", warns)
	}
	if !reflect.DeepEqual(c, Defaults()) {
		t.Errorf("got %+v, want the defaults", c)
	}
}

func TestPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/x")
	if got := Path(); got != "/x/gag/config" {
		t.Errorf("with XDG_CONFIG_HOME: %q", got)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/h")
	if got := Path(); got != "/h/.config/gag/config" {
		t.Errorf("without XDG_CONFIG_HOME: %q", got)
	}
}

func TestDescribe(t *testing.T) {
	c := Defaults()
	c.Sky, c.From["sky"] = "stars", File
	want := strings.Join([]string{
		"config: /h/.config/gag/config",
		"sky      stars   (file)",
		"limit    8       (default)",
		"repos    -       (default)",
		"user     -       (default)",
		"refresh  5m      (default)",
		"decay    45      (default)",
		"",
	}, "\n")
	if got := c.Describe("/h/.config/gag/config", true); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if got := Defaults().Describe("/p", false); !strings.HasPrefix(got, "config: /p (not found: using defaults)\n") {
		t.Errorf("missing file: %q", got)
	}
}

func TestShortDurations(t *testing.T) {
	for d, want := range map[time.Duration]string{
		5 * time.Minute: "5m", time.Hour: "1h", 90 * time.Second: "1m30s",
		30 * time.Second: "30s", 90 * time.Minute: "1h30m", 10 * time.Minute: "10m",
	} {
		if got := short(d); got != want {
			t.Errorf("short(%v) = %q, want %q", d, got, want)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config/`
Expected: FAIL, with build errors such as `undefined: Parse`.

- [ ] **Step 3: Implement**

`internal/config/config.go`:

```go
// Package config reads GAG's settings file, ~/.config/gag/config: one
// "key = value" per line. It never fails hard; problems become warnings.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Source says where a setting's value came from.
type Source string

const (
	Default Source = "default"
	File    Source = "file"
)

// Config holds the settings.
type Config struct {
	Sky     string // "random" or "stars"
	Limit   int
	Repos   []string
	User    string
	Refresh time.Duration
	Decay   float64
	From    map[string]Source // where each key's value came from
}

// Keys lists the settings in the order `gag config` shows them.
var Keys = []string{"sky", "limit", "repos", "user", "refresh", "decay"}

// Defaults are the settings when there is no config file.
func Defaults() Config {
	c := Config{Sky: "random", Limit: 8, Refresh: 5 * time.Minute, Decay: 45, From: map[string]Source{}}
	for _, k := range Keys {
		c.From[k] = Default
	}
	return c
}

// Warning is a problem with the file; GAG reports it and carries on.
type Warning struct {
	Line int // 0 when it's about the whole file
	Msg  string
}

func (w Warning) String() string {
	if w.Line == 0 {
		return "config: " + w.Msg
	}
	return fmt.Sprintf("config line %d: %s", w.Line, w.Msg)
}

// Path is where the config file lives: $XDG_CONFIG_HOME/gag/config, or
// ~/.config/gag/config, on macOS too, where CLI users expect it.
func Path() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "gag", "config")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "gag", "config")
	}
	return filepath.Join(home, ".config", "gag", "config")
}

// Load reads the config file at path. A missing file gives the defaults
// without complaint; anything else wrong becomes a warning.
func Load(path string) (Config, []Warning) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Defaults(), []Warning{{Msg: err.Error()}}
	}
	defer f.Close()
	return Parse(f)
}

// Parse reads key = value lines. # starts a comment; blank lines, spaces
// around keys and values, Windows line endings and a leading byte-order
// mark are ignored; keys are case-insensitive.
func Parse(r io.Reader) (Config, []Warning) {
	c := Defaults()
	var warns []Warning
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if n == 1 {
			line = strings.TrimPrefix(line, "﻿")
		}
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			warns = append(warns, Warning{n, fmt.Sprintf("%q isn't key = value", line)})
			continue
		}
		key, val = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(val)
		if msg := c.set(key, val); msg != "" {
			warns = append(warns, Warning{n, msg})
			continue
		}
		c.From[key] = File
	}
	if err := sc.Err(); err != nil {
		warns = append(warns, Warning{Msg: err.Error()})
	}
	return c, warns
}

// set applies one setting, or says why it can't.
func (c *Config) set(key, val string) string {
	switch key {
	case "sky":
		switch v := strings.ToLower(val); v {
		case "random", "stars":
			c.Sky = v
			return ""
		}
		return fmt.Sprintf("sky %q isn't random or stars; using %s", val, c.Sky)
	case "limit":
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 {
			return fmt.Sprintf("limit %q isn't a number of repos (1 or more); using %d", val, c.Limit)
		}
		c.Limit = n
	case "repos":
		var repos []string
		for _, r := range strings.Split(val, ",") {
			if r = strings.TrimSpace(r); r == "" {
				continue
			}
			if !strings.Contains(r, "/") {
				return fmt.Sprintf("repos entry %q isn't owner/name; ignoring this line", r)
			}
			repos = append(repos, r)
		}
		c.Repos = repos
	case "user":
		if val == "" || strings.ContainsAny(val, " /") {
			return fmt.Sprintf("user %q isn't a GitHub login; ignoring it", val)
		}
		c.User = val
	case "refresh":
		d, err := time.ParseDuration(val)
		if err != nil || d < 30*time.Second {
			return fmt.Sprintf("refresh %q isn't a duration of 30s or more (like 5m); using %s", val, short(c.Refresh))
		}
		c.Refresh = d
	case "decay":
		f, err := strconv.ParseFloat(val, 64)
		if err != nil || f <= 0 {
			return fmt.Sprintf("decay %q isn't a number of days above 0; using %g", val, c.Decay)
		}
		c.Decay = f
	default:
		return fmt.Sprintf("unknown setting %q (known: %s)", key, strings.Join(Keys, ", "))
	}
	return ""
}

// Describe is what `gag config` prints: where the file is, and each setting
// with its value and where that came from.
func (c Config) Describe(path string, exists bool) string {
	var b strings.Builder
	b.WriteString("config: " + path)
	if !exists {
		b.WriteString(" (not found: using defaults)")
	}
	b.WriteString("\n")
	for _, k := range Keys {
		fmt.Fprintf(&b, "%-8s %-7s (%s)\n", k, c.value(k), c.From[k])
	}
	return b.String()
}

func (c Config) value(key string) string {
	switch key {
	case "sky":
		return c.Sky
	case "limit":
		return strconv.Itoa(c.Limit)
	case "repos":
		if len(c.Repos) == 0 {
			return "-"
		}
		return strings.Join(c.Repos, ",")
	case "user":
		if c.User == "" {
			return "-"
		}
		return c.User
	case "refresh":
		return short(c.Refresh)
	case "decay":
		return strconv.FormatFloat(c.Decay, 'g', -1, 64)
	}
	return ""
}

// short prints a duration without trailing zero units: 5m, 1h, 1m30s.
func short(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/config/ && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/config
git commit -m "config: parse ~/.config/gag/config into settings with warnings

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: GitHub: star counts in the repo listing

**Files:**
- Modify: `internal/github/repo.go` (`Repo.Stars`, `stargazerCount` in `metaFields`)
- Test: `internal/github/sync_test.go` (append)

**Interfaces:**
- Consumes: the v0.4 test helper `fakeGitHub(t, answer)` (`sync_test.go` already imports `context`, `encoding/json` and `strings`).
- Produces: `github.Repo.Stars int` (JSON `stars`), filled by `ListRepos`, `ListOwnerRepos` and `LookupRepo`.

- [ ] **Step 1: Write the failing test**

Append to `internal/github/sync_test.go`:

```go
func TestListingReadsStarCounts(t *testing.T) {
	c := fakeGitHub(t, func(q string) string {
		if !strings.Contains(q, "stargazerCount") {
			t.Errorf("the listing query should ask for stargazerCount: %s", q)
		}
		return `{"data":{"viewer":{"repositories":{"nodes":[
			{"nameWithOwner":"me/a","isArchived":false,"pushedAt":"2026-09-01T00:00:00Z",
			 "primaryLanguage":{"name":"Go"},"repositoryTopics":{"nodes":[]},"stargazerCount":42}]}}}}`
	})
	repos, err := c.ListRepos(context.Background(), 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Stars != 42 {
		t.Errorf("repos = %+v, want me/a with 42 stars", repos)
	}
}

func TestStarsSurviveTheCache(t *testing.T) {
	data, err := json.Marshal(&Repo{NameWithOwner: "me/a", Stars: 42})
	if err != nil {
		t.Fatal(err)
	}
	var back, old Repo
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"nameWithOwner":"me/a"}`), &old); err != nil {
		t.Fatal(err)
	}
	if back.Stars != 42 || old.Stars != 0 {
		t.Errorf("stars after a cache round trip = %d, from an old cache = %d; want 42 and 0", back.Stars, old.Stars)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/github/ -run 'TestListingReadsStarCounts|TestStarsSurviveTheCache'`
Expected: FAIL (build error `repos[0].Stars undefined`).

- [ ] **Step 3: Implement**

In `internal/github/repo.go`:

1. In `type Repo struct`, add after `PushedAt      time.Time `json:"pushedAt"``:

```go
	Stars         int       `json:"stars,omitempty"`
```

2. Change `metaFields` to:

```go
const metaFields = `nameWithOwner isArchived pushedAt stargazerCount primaryLanguage{name} repositoryTopics(first:20){nodes{topic{name}}}`
```

3. In `type metaNode struct`, add after `PushedAt         time.Time` the line `	StargazerCount   int`.
4. In `func (m metaNode) repo()`, change the first line to `	r := &Repo{NameWithOwner: m.NameWithOwner, Archived: m.IsArchived, PushedAt: m.PushedAt, Stars: m.StargazerCount}`.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/github/ && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/github
git commit -m "github: read each repo's star count from the listing

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Scene: the star sky

**Files:**
- Create: `internal/scene/stars.go`, `internal/scene/stars_test.go`
- Modify: `internal/scene/sky.go` (split out `skyGradient`), `internal/scene/draw.go` (`View.Sky` and `View.StarTotal`; star mode draws `DrawStarSky` in the top band)

**Interfaces:**
- Consumes: v0.4's `SkyAt`, `Darkness`, `DrawStars`, `noise`, `starColor`, `Draw`, and the test helpers `at()`, `painted()` and `manyPlots()`.
- Produces:
  - `type SkyMode int` with `const SkyRandom SkyMode = iota; SkyStars`
  - `View.Sky SkyMode`, `View.StarTotal int`
  - `func DrawStarSky(c *pixel.Canvas, y0, y1, total int, darkness float64)`

- [ ] **Step 1: Write the failing tests**

`internal/scene/stars_test.go`:

```go
package scene

import (
	"testing"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

func starSky(total int, darkness float64) *pixel.Canvas {
	c := pixel.New(80, 30)
	DrawStarSky(c, 0, 30, total, darkness)
	return c
}

func TestStarSkyHasOneDotPerStar(t *testing.T) {
	if n := painted(starSky(17, 1)); n != 17 {
		t.Errorf("17 stars drew %d dots", n)
	}
}

func TestStarSkyKeepsItsStars(t *testing.T) {
	before, after := starSky(17, 1), starSky(18, 1)
	for y := 0; y < before.H; y++ {
		for x := 0; x < before.W; x++ {
			if px := before.At(x, y); px != (pixel.RGB{}) && after.At(x, y) != px {
				t.Fatalf("a new star moved or changed the star at %d,%d", x, y)
			}
		}
	}
	if n := painted(after); n != 18 {
		t.Errorf("18 stars drew %d dots", n)
	}
}

func TestStarSkyCapsAndBrightens(t *testing.T) {
	capacity := 80 * 30 / 40
	brightness := func(c *pixel.Canvas) int {
		sum := 0
		for y := 0; y < c.H; y++ {
			for x := 0; x < c.W; x++ {
				sum += int(c.At(x, y).R)
			}
		}
		return sum
	}
	full, huge := starSky(capacity, 1), starSky(50000, 1)
	if n := painted(huge); n != capacity {
		t.Errorf("50000 stars drew %d dots, want the sky's capacity %d", n, capacity)
	}
	if brightness(huge) <= brightness(full) {
		t.Error("more stars than the sky holds should make it brighter")
	}
}

func TestStarSkyOnlyAtNightAndOnlyStars(t *testing.T) {
	if n := painted(starSky(17, 0)); n != 0 {
		t.Errorf("%d stars in daylight", n)
	}
	if n := painted(starSky(0, 1)); n != 0 {
		t.Errorf("a garden with no stars drew %d", n)
	}
}

func TestStarModeShowsTheGardensStars(t *testing.T) {
	bright := func(v View) int {
		c := Draw(v)
		n := 0
		for y := 0; y < c.H; y++ {
			for x := 0; x < c.W; x++ {
				if c.At(x, y).G > 100 {
					n++
				}
			}
		}
		return n
	}
	v := View{Cols: 3 * BedCols, Plots: manyPlots(3), Now: at(23, 0), Seed: 1}
	random := bright(v)
	v.Sky = SkyStars
	none := bright(v)
	v.StarTotal = 40
	forty := bright(v)
	if none >= random || none >= forty {
		t.Errorf("bright pixels: random sky %d, no stars %d, 40 stars %d", random, none, forty)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/scene/`
Expected: FAIL (build errors: `undefined: DrawStarSky`, `unknown field Sky in struct literal of type View`).

- [ ] **Step 3: Split the sky gradient**

In `internal/scene/sky.go`, replace the whole `DrawSky` function with:

```go
func DrawSky(c *pixel.Canvas, t time.Time, y0, y1 int, seed int64) {
	skyGradient(c, t, y0, y1)
	DrawStars(c, y0, y1, seed, Darkness(t))
}

// skyGradient fills pixel rows [y0, y1) with the sky's colour at time t.
func skyGradient(c *pixel.Canvas, t time.Time, y0, y1 int) {
	top, bot := SkyAt(t)
	span := float64(max(1, y1-y0-1))
	for y := y0; y < y1; y++ {
		col := pixel.Lerp(top, bot, float64(y-y0)/span)
		for x := 0; x < c.W; x++ {
			c.Set(x, y, col)
		}
	}
}
```

- [ ] **Step 4: The star sky**

`internal/scene/stars.go`:

```go
package scene

import (
	"math"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// SkyMode picks what the night sky shows.
type SkyMode int

const (
	SkyRandom SkyMode = iota // seeded random stars
	SkyStars                 // one star per GitHub star in the garden
)

const starSeed = 0x5a7

// DrawStarSky draws one star per GitHub star in the upper part of pixel rows
// [y0, y1), as bright as the night is dark. Star i's place depends only on i
// and the stars before it, so a new star never moves the others. When there
// are more stars than the sky has room for (one per 40 px), the sky shows as
// many as fit and glows brighter instead of crowding.
func DrawStarSky(c *pixel.Canvas, y0, y1, total int, darkness float64) {
	y0 = max(y0, 0) // a short window crops the top of the sky; count only what shows
	if darkness <= 0 || total <= 0 || c.W == 0 || y1 <= y0 {
		return
	}
	band := max(1, (y1-y0)*2/3)
	capacity := max(1, c.W*(y1-y0)/40)
	n, boost := total, 0.0
	if total > capacity {
		n = capacity
		boost = math.Min(1, math.Log10(float64(total)/float64(capacity))/2)
	}
	taken := make(map[[2]int]bool, n)
	for i := 0; i < n; i++ {
		x := int(noise(starSeed, i, 1) * float64(c.W))
		y := y0 + int(noise(starSeed, i, 2)*float64(band))
		for tries := 0; taken[[2]int{x, y}] && tries < c.W*band; tries++ {
			if x++; x >= c.W { // the next free spot, left to right, top to bottom
				x, y = 0, y+1
				if y >= y0+band {
					y = y0
				}
			}
		}
		taken[[2]int{x, y}] = true
		bright := 0.4 + 0.6*noise(starSeed, i, 3)
		c.Blend(x, y, starColor, darkness*math.Min(1, bright+boost))
	}
}
```

- [ ] **Step 5: Draw it in star mode**

In `internal/scene/draw.go`:

1. In `type View struct`, add after the `Motion` field:

```go
	Sky       SkyMode // what the night sky shows
	StarTotal int     // the garden's GitHub stars, for SkyStars
```

2. In `Draw`, replace

```go
		DrawSky(c, v.Now, skyTop, oy+groundTop, v.Seed+int64(b))
```

with

```go
		if v.Sky == SkyStars {
			skyGradient(c, v.Now, skyTop, oy+groundTop)
			if b == 0 { // one sky of stars, in the top band
				DrawStarSky(c, skyTop, oy+groundTop, v.StarTotal, Darkness(v.Now))
			}
		} else {
			DrawSky(c, v.Now, skyTop, oy+groundTop, v.Seed+int64(b))
		}
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/scene/ && go test ./...`
Expected: PASS, and `TestComposeGolden` passes unchanged.

- [ ] **Step 7: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/scene
git commit -m "scene: a night sky of the garden's GitHub stars

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Scene: shooting stars

**Files:**
- Modify: `internal/scene/stars.go` (`Shooting`, `ShootingFor`, drawing), `internal/scene/draw.go` (`View.Shooting`, drawn in the top band)
- Test: `internal/scene/stars_test.go` (append)

**Interfaces:**
- Consumes: Task 3's `View`, `Draw` and `stars_test.go`; v0.4's `SkyAt` and `noise`; and the test helpers `at()`, `painted()`, `manyPlots()` and `dist(a, b pixel.RGB) float64` (from `weather_test.go`).
- Produces:
  - `type Shooting struct{ Start time.Time; Seed int64 }`
  - `const ShootingFor = 1500 * time.Millisecond`
  - `View.Shooting []Shooting`
  - Unexported colors `meteorHead` and `meteorTail`

- [ ] **Step 1: Write the failing tests**

Append to `internal/scene/stars_test.go`:

```go
// shot draws one shooting star, age into its flight, on an empty 80×40 sky.
func shot(age time.Duration) *pixel.Canvas {
	c := pixel.New(80, 40)
	now := at(23, 0)
	drawShootingStars(c, now, []Shooting{{Start: now.Add(-age), Seed: 7}}, 0, 40)
	return c
}

// rightmost is the rightmost painted column, or -1.
func rightmost(c *pixel.Canvas) int {
	for x := c.W - 1; x >= 0; x-- {
		for y := 0; y < c.H; y++ {
			if c.At(x, y) != (pixel.RGB{}) {
				return x
			}
		}
	}
	return -1
}

func TestShootingStarFliesThenFades(t *testing.T) {
	early, late := shot(300*time.Millisecond), shot(1200*time.Millisecond)
	if painted(early) == 0 {
		t.Fatal("no shooting star in flight")
	}
	if rightmost(late) <= rightmost(early) {
		t.Error("a shooting star should move across the sky")
	}
	for _, age := range []time.Duration{-time.Second, ShootingFor, 2 * time.Second} {
		if n := painted(shot(age)); n != 0 {
			t.Errorf("%v into its flight it drew %d pixels", age, n)
		}
	}
}

func TestShootingStarsStayInTheSky(t *testing.T) {
	now := at(23, 0)
	for seed := int64(0); seed < 20; seed++ {
		for age := time.Duration(0); age < ShootingFor; age += 100 * time.Millisecond {
			c := pixel.New(240, 60)
			drawShootingStars(c, now, []Shooting{{Start: now.Add(-age), Seed: seed}}, 0, 42)
			for y := 42; y < c.H; y++ {
				for x := 0; x < c.W; x++ {
					if c.At(x, y) != (pixel.RGB{}) {
						t.Fatalf("seed %d, %v in: drew below the sky at %d,%d", seed, age, x, y)
					}
				}
			}
		}
	}
}

func TestDrawShowsShootingStars(t *testing.T) {
	now := at(23, 0)
	v := View{Cols: 3 * BedCols, Plots: manyPlots(3), Now: now, Seed: 1}
	plain := Draw(v).Encode(pixel.TrueColor)
	shown := 0
	for seed := int64(1); seed <= 5; seed++ { // plants in front may hide one
		v.Shooting = []Shooting{{Start: now.Add(-700 * time.Millisecond), Seed: seed}}
		if Draw(v).Encode(pixel.TrueColor) != plain {
			shown++
		}
	}
	if shown == 0 {
		t.Error("Draw showed none of the shooting stars in flight")
	}
	v.Shooting = []Shooting{{Start: now.Add(-2 * time.Second), Seed: 1}}
	if Draw(v).Encode(pixel.TrueColor) != plain {
		t.Error("a landed shooting star should leave no trace")
	}
}

func TestShootingStarsStandOut(t *testing.T) {
	for _, now := range []time.Time{at(12, 0), at(23, 30)} {
		top, bottom := SkyAt(now)
		for _, col := range []pixel.RGB{meteorHead, meteorTail} {
			if dist(col, top) < 60 || dist(col, bottom) < 60 {
				t.Errorf("shooting star colour %v blends into the %s sky", col, now.Format("15:04"))
			}
		}
	}
}
```

and add `"time"` to that file's imports.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/scene/`
Expected: FAIL (build errors: `undefined: Shooting`, `undefined: meteorHead`).

- [ ] **Step 3: Implement**

In `internal/scene/stars.go`:

1. Add `"time"` to the imports.
2. Append:

```go
// ShootingFor is how long a shooting star takes to cross the sky.
const ShootingFor = 1500 * time.Millisecond

// Shooting is a shooting star that starts at Start; Seed picks its path.
type Shooting struct {
	Start time.Time
	Seed  int64
}

var (
	meteorHead = rgb(255, 250, 225)
	meteorTail = rgb(255, 205, 120) // warm, so it shows against the pale day sky too
)

const meteorSlope = 0.35 // pixels down per pixel across

// drawShootingStars draws the shooting stars in flight at t across pixel
// rows [y0, y1): a bright head and a fading warm tail, sliding down and to
// the right. Each starts in the top third of the sky and ends above y1, so
// it never streaks into the plants.
func drawShootingStars(c *pixel.Canvas, t time.Time, shots []Shooting, y0, y1 int) {
	y0 = max(y0, 0) // a short window crops the top of the sky
	if y1-y0 < 6 {
		return
	}
	for _, s := range shots {
		age := t.Sub(s.Start)
		if age < 0 || age >= ShootingFor {
			continue
		}
		f := float64(age) / float64(ShootingFor)
		x0 := int(noise(s.Seed, 0, 1) * float64(c.W) * 0.6)
		ys := y0 + int(noise(s.Seed, 0, 2)*float64((y1-y0)/3))
		run := math.Min(float64(c.W)*0.4, float64(y1-1-ys)/meteorSlope)
		for k := 0; k < 8; k++ { // the head, then its tail
			d := f*run - float64(k)*1.5
			if d < 0 {
				break
			}
			col, a := meteorTail, 0.9*(1-float64(k)/8)
			if k == 0 {
				col, a = meteorHead, 1
			}
			c.Blend(x0+int(d), ys+int(d*meteorSlope), col, a)
		}
	}
}
```

In `internal/scene/draw.go`:

1. In `type View struct`, add after `StarTotal`:

```go
	Shooting  []Shooting // shooting stars, drawn while in flight
```

2. In `Draw`, directly after the line `		DrawClouds(c, v.Now, skyTop, oy+groundTop, v.Seed+int64(b))`, add:

```go
		if b == 0 {
			drawShootingStars(c, v.Now, v.Shooting, skyTop, oy+groundTop)
		}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/scene/ && go test ./...`
Expected: PASS, with the golden frame unchanged.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/scene
git commit -m "scene: shooting stars

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Live: shooting stars for new stars, ticker notes, star total

**Files:**
- Modify: `internal/live/snapshot.go` (`Repo.Stars`), `internal/live/ticker.go` (`calmIcon`), `internal/live/model.go` (sky config, star tracking, shots, notes, pruning, frame rate, ticker)
- Test: `internal/live/model_test.go` and `internal/live/ticker_test.go` (append and edit)

**Interfaces:**
- Consumes: Task 3's `scene.SkyMode`, `scene.SkyStars`, `View.Sky` and `View.StarTotal`; Task 4's `scene.Shooting`, `scene.ShootingFor` and `View.Shooting`; and the v0.4 helpers `ready`, `newModel`, `garden3`, `grow`, `step`, `visible` and `t0`.
- Produces:
  - `live.Repo.Stars int`
  - `live.Config.Sky scene.SkyMode`
  - Ticker items `⭐ New star on <repo>` / `⭐ N new stars on <repo>`, and in star mode the calm line gains ` · ⭐ <total>`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/live/model_test.go`:

```go
// starred copies snap with the given star counts on its repos, in order.
func starred(snap Snapshot, stars ...int) Snapshot {
	out := snap
	out.Repos = append([]Repo(nil), snap.Repos...)
	for i, s := range stars {
		out.Repos[i].Stars = s
	}
	return out
}

func TestNewStarsShootAndGetNoted(t *testing.T) {
	m := ready(newModel(starred(garden3(), 5, 0, 1), nil, t0), 100, 30)
	if len(m.shots) != 0 {
		t.Fatal("the first load only sets the baseline")
	}
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 7, 1, 1)})
	if len(m.shots) != 3 {
		t.Errorf("shots = %d, want 3 (2 + 1)", len(m.shots))
	}
	lines := strings.Split(m.View(), "\n")
	if last := visible(lines[len(lines)-1]); !strings.Contains(last, "⭐ 2 new stars on bloom") {
		t.Errorf("ticker = %q", last)
	}
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 57, 1, 1)})
	if len(m.shots) != 6 {
		t.Errorf("shots = %d; a refresh adds at most 3", len(m.shots))
	}
}

func TestStarsThatDontCountDontShoot(t *testing.T) {
	m := ready(newModel(starred(garden3(), 5, 5, 5), nil, t0), 100, 30)
	offline := starred(garden3(), 9, 9, 9)
	offline.Offline = true
	m, _ = step(m, loadedMsg{snap: offline})
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 4, 5, 5)}) // an unstar
	newcomer := starred(garden3(), 4, 5, 5)
	extra := grow("newcomer", 10, 0, t0)
	extra.Stars = 30
	newcomer.Repos = append(newcomer.Repos, extra)
	m, _ = step(m, loadedMsg{snap: newcomer}) // a repo joining brings its stars along
	if len(m.shots) != 0 || len(m.notes) != 0 {
		t.Errorf("shots %d, notes %d; want none", len(m.shots), len(m.notes))
	}
}

func TestShotsAndNotesExpire(t *testing.T) {
	now := t0
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return starred(garden3(), 1, 1, 1), nil },
		DecayDays: 45,
		Now:       func() time.Time { return now },
	})
	m = ready(m, 100, 30)
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 2, 1, 1)})
	if len(m.shots) != 1 || len(m.notes) != 1 {
		t.Fatalf("shots %d, notes %d; want 1 each", len(m.shots), len(m.notes))
	}
	now = now.Add(61 * time.Second)
	m, _ = step(m, tickMsg{})
	if len(m.shots) != 0 || len(m.notes) != 0 {
		t.Errorf("shots %d, notes %d; want none after a minute", len(m.shots), len(m.notes))
	}
}

func TestShootingStarsKeepFramesFast(t *testing.T) {
	night := t0.Add(11 * time.Hour)
	m := ready(newModel(starred(garden3(), 1, 1, 1), nil, night), 80, 24)
	if m.frameInterval() != slowFrame {
		t.Fatal("a quiet night should idle")
	}
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 2, 1, 1)})
	if m.frameInterval() != fastFrame {
		t.Error("a shooting star in flight needs the fast frame rate")
	}
}

func TestCalmLineShowsTheStarTotalInStarMode(t *testing.T) {
	snap := Snapshot{Repos: []Repo{grow("bloom", 60, 4, t0.Add(-time.Hour))}, Commits7d: 5, FetchedAt: t0}
	snap.Repos[0].Stars = 48
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return snap, nil },
		DecayDays: 45,
		Now:       func() time.Time { return t0 },
		Sky:       scene.SkyStars,
	})
	m = ready(m, 100, 30)
	lines := strings.Split(m.View(), "\n")
	if last := visible(lines[len(lines)-1]); !strings.Contains(last, "Garden thriving: 5 commits this week · ⭐ 48") {
		t.Errorf("ticker = %q", last)
	}
}
```

Add `"github.com/RursusAeternum/GitAGarden/internal/scene"` to `model_test.go`'s imports. In `internal/live/ticker_test.go`, change `[]string{"⚡", "🌷", "🥀", "🐌", "🌱"}` to `[]string{"⚡", "🌷", "🥀", "🐌", "🌱", "⭐"}`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/live/`
Expected: FAIL (build errors: `unknown field Stars`, `m.shots undefined`, `unknown field Sky in struct literal of type Config`).

- [ ] **Step 3: Implement**

In `internal/live/snapshot.go`, in `type Repo struct`, add after `Rising    bool …`:

```go
	Stars     int         // GitHub stars
```

In `internal/live/ticker.go`, replace the whole `calm` function with:

```go
// calmIcon marks the ticker's all-is-well line.
const calmIcon = "🌱"

func calm(commits7d int) Item {
	switch commits7d {
	case 0:
		return Item{calmIcon, "All quiet in the garden"}
	case 1:
		return Item{calmIcon, "Garden thriving: 1 commit this week"}
	}
	return Item{calmIcon, fmt.Sprintf("Garden thriving: %d commits this week", commits7d)}
}
```

In `internal/live/model.go`:

1. Add to the `const (...)` block:

```go
	maxShots         = 3                // shooting stars per refresh, however many stars arrive
	shotGap          = 2 * time.Second  // between queued shooting stars
	noteFor          = time.Minute      // how long the ticker names a new star
```

2. In `Config`, add `	Sky       scene.SkyMode    // what the night sky shows`.
3. In `Model`, add the fields:

```go
	stars      map[string]int   // star counts from the last online load; nil before the first
	shots      []scene.Shooting // queued and flying shooting stars
	notes      []starNote       // ticker notes about new stars
```

4. Add the type and methods:

```go
// starNote tells the ticker that repo got n new stars, until until.
type starNote struct {
	repo  string
	n     int
	until time.Time
}

// noticeNewStars compares star counts with the last online load: each repo
// that gained stars gets a ticker note, and up to maxShots shooting stars
// cross the sky. The first load only sets the baseline; repos that just
// joined the garden don't count, and neither do lost stars.
func (m *Model) noticeNewStars(repos []Repo) {
	counts := make(map[string]int, len(repos))
	for _, r := range repos {
		counts[r.Name] = r.Stars
	}
	if m.stars != nil {
		wall, at, shots := m.cfg.Now(), m.now(), 0
		for _, r := range repos {
			before, known := m.stars[r.Name]
			gained := r.Stars - before
			if !known || gained <= 0 {
				continue
			}
			m.notes = append(m.notes, starNote{repo: r.Name, n: gained, until: wall.Add(noteFor)})
			for i := 0; i < gained && shots < maxShots; i++ {
				m.shots = append(m.shots, scene.Shooting{Start: at.Add(time.Duration(shots) * shotGap), Seed: at.UnixMilli() + int64(len(m.shots))})
				shots++
			}
		}
	}
	m.stars = counts
}

// prune forgets shooting stars that have landed and notes that have expired.
func (m *Model) prune() {
	at, wall := m.now(), m.cfg.Now()
	var shots []scene.Shooting
	for _, s := range m.shots {
		if at.Sub(s.Start) < scene.ShootingFor {
			shots = append(shots, s)
		}
	}
	var notes []starNote
	for _, n := range m.notes {
		if wall.Before(n.until) {
			notes = append(notes, n)
		}
	}
	m.shots, m.notes = shots, notes
}

func (m Model) starTotal() int {
	total := 0
	for _, r := range m.repos {
		total += r.Stars
	}
	return total
}

// starItems are the ticker's notes about new stars.
func (m Model) starItems() []Item {
	var items []Item
	for _, n := range m.notes {
		if !m.cfg.Now().Before(n.until) {
			continue
		}
		text := "New star on " + n.repo
		if n.n > 1 {
			text = fmt.Sprintf("%d new stars on %s", n.n, n.repo)
		}
		items = append(items, Item{"⭐", text})
	}
	return items
}
```

5. In `Update`'s `case loadedMsg:`, replace

```go
			if msg.snap.Offline && len(m.repos) > 0 {
				m.repos = updateKnown(m.repos, msg.snap.Repos)
			} else {
				m.repos = merge(m.repos, msg.snap.Repos)
			}
```

with

```go
			if msg.snap.Offline && len(m.repos) > 0 {
				m.repos = updateKnown(m.repos, msg.snap.Repos)
			} else {
				m.repos = merge(m.repos, msg.snap.Repos)
			}
			if !msg.snap.Offline { // cached counts are no news
				m.noticeNewStars(msg.snap.Repos)
			}
```

6. In `Update`'s `case tickMsg:`, make the first statement `		m.prune()`.
7. In `busy`, directly after the line `	at := m.now()`, add:

```go
	for _, s := range m.shots {
		if at.Sub(s.Start) < scene.ShootingFor {
			return true // a shooting star is flying, or queued to
		}
	}
```

8. In `View`, change `	v := scene.View{Cols: m.cols, Rows: rows, Plots: plots, Now: at, Seed: 1, Motion: true}` to:

```go
	v := scene.View{Cols: m.cols, Rows: rows, Plots: plots, Now: at, Seed: 1, Motion: true,
		Sky: m.cfg.Sky, StarTotal: m.starTotal(), Shooting: m.shots}
```

9. In `tickerLine`, replace `	return Line(Items(m.repos, at, m.cfg.DecayDays, m.snap.Commits7d), m.elapsed(), status, m.cols)` with:

```go
	items := Items(m.repos, at, m.cfg.DecayDays, m.snap.Commits7d)
	if m.cfg.Sky == scene.SkyStars && len(items) == 1 && items[0].Icon == calmIcon {
		items[0].Text += fmt.Sprintf(" · ⭐ %d", m.starTotal())
	}
	return Line(append(m.starItems(), items...), m.elapsed(), status, m.cols)
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/live/ && go test -race ./internal/live/ && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./...
git add internal/live
git commit -m "live: shooting stars for new GitHub stars, ticker notes, star total

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: `cmd/gag`: config file, `gag config`, the sky everywhere

**Files:**
- Create: `cmd/gag/configcmd.go`
- Modify: `cmd/gag/garden.go` (`settings`, `merge`, `skyMode`, `runGarden` with config, `printOnce` with the sky, `repoFor` stars, demo stars), `cmd/gag/main.go` (the `config` command, `runReplay` sky), `internal/replay/model.go` (`Config.Sky/Stars`; draws with the sky), `README.md`
- Test: `cmd/gag/garden_test.go` (append)

**Interfaces:**
- Consumes: Task 1 (`config.Load`, `config.Path`, `config.Config`, `Describe`), Task 2 (`github.Repo.Stars`), Task 3 (`scene.SkyMode`, `View.Sky/StarTotal`, `scene.Draw`), and Task 5 (`live.Repo.Stars`, `live.Config.Sky`).
- Produces: `gag config`; `settings`, `merge(file config.Config, set map[string]bool, flags settings) settings` and `skyMode(string) scene.SkyMode` in `cmd/gag`; `replay.Config.Sky` and `replay.Config.Stars`.

- [ ] **Step 1: Write the failing tests**

Append to `cmd/gag/garden_test.go`:

```go
func TestMergePrefersFlagsThenTheFile(t *testing.T) {
	file := config.Defaults()
	file.Sky, file.Limit, file.Refresh, file.User = "stars", 12, 2*time.Minute, "octocat"
	flags := settings{limit: 3, refresh: 5 * time.Minute, decay: 45}
	got := merge(file, map[string]bool{"limit": true}, flags)
	if got.sky != scene.SkyStars || got.limit != 3 || got.refresh != 2*time.Minute || got.user != "octocat" || got.decay != 45 {
		t.Errorf("merged = %+v; want stars, limit 3 (flag), refresh 2m and user octocat (file), decay 45", got)
	}
}

func TestUserFlagBeatsTheFilesRepoList(t *testing.T) {
	file := config.Defaults()
	file.Repos = []string{"me/a"}
	got := merge(file, map[string]bool{"user": true}, settings{user: "octocat", limit: 8, refresh: 5 * time.Minute, decay: 45})
	if got.user != "octocat" || got.repos != nil {
		t.Errorf("merged = %+v; -user should show octocat's garden, not the file's repos", got)
	}
}

func TestRepoForCarriesStars(t *testing.T) {
	if lr := repoFor(&github.Repo{NameWithOwner: "me/x", Stars: 42}, time.Now()); lr.Stars != 42 {
		t.Errorf("stars = %d, want 42", lr.Stars)
	}
}

func TestDemoHasStars(t *testing.T) {
	total := 0
	for _, r := range demoGarden(time.Now()) {
		total += r.Stars
	}
	if total == 0 {
		t.Error("the demo should have stars for the star sky")
	}
}
```

and add `"github.com/RursusAeternum/GitAGarden/internal/config"` and `"github.com/RursusAeternum/GitAGarden/internal/scene"` to that file's imports.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./cmd/gag/`
Expected: FAIL (build errors: `undefined: merge`, `undefined: settings`, `unknown field Stars`).

- [ ] **Step 3: Settings and merge**

Append to `cmd/gag/garden.go`:

```go
// settings are the garden's options once the config file and flags are
// merged.
type settings struct {
	sky     scene.SkyMode
	limit   int
	repos   []string
	user    string
	refresh time.Duration
	decay   float64
}

// merge takes each option from its flag when the flag was given on the
// command line, and from the config file (which falls back to the defaults)
// otherwise. The sky is set only in the file.
func merge(file config.Config, set map[string]bool, flags settings) settings {
	s := settings{sky: skyMode(file.Sky), limit: file.Limit, repos: file.Repos, user: file.User,
		refresh: file.Refresh, decay: file.Decay}
	if set["limit"] {
		s.limit = flags.limit
	}
	if set["repos"] {
		s.repos = flags.repos
	}
	if set["user"] {
		s.user = flags.user
	}
	if set["refresh"] {
		s.refresh = flags.refresh
	}
	if set["decay"] {
		s.decay = flags.decay
	}
	if set["user"] && !set["repos"] {
		s.repos = nil // -user asks for that garden, not the file's repo list
	}
	return s
}

func skyMode(s string) scene.SkyMode {
	if s == "stars" {
		return scene.SkyStars
	}
	return scene.SkyRandom
}

// loadConfig reads the config file, reporting problems on stderr; it runs
// before any fullscreen view opens.
func loadConfig() config.Config {
	cfg, warns := config.Load(config.Path())
	for _, w := range warns {
		logf(w.String())
	}
	return cfg
}
```

and add `"github.com/RursusAeternum/GitAGarden/internal/config"` to `garden.go`'s imports.

- [ ] **Step 4: `runGarden` and `printOnce` use the merged settings and the sky**

In `cmd/gag/garden.go`, replace the whole `runGarden` function with:

```go
func runGarden(args []string) error {
	fs := flag.NewFlagSet("garden", flag.ExitOnError)
	demoFlag := fs.Bool("demo", false, "show fake demo repos instead of GitHub")
	limit := fs.Int("limit", 8, "how many recently pushed repos to show")
	names := fs.String("repos", "", "comma-separated owner/name list to show instead (any public repo works)")
	user := fs.String("user", "", "show another GitHub user's or organization's public garden")
	ttl := fs.Duration("ttl", 15*time.Minute, "reuse cached GitHub data younger than this (live view: half of -refresh)")
	decay := fs.Float64("decay", 45, "days of neglect until fully wilted")
	simulate := fs.String("simulate", "", "fast-forward: show the garden as it would look after this long untouched, e.g. 30d, 2w, 36h")
	once := fs.Bool("once", false, "print one static frame and exit (the default when output isn't a terminal)")
	refresh := fs.Duration("refresh", 5*time.Minute, "how often the live view reloads GitHub data")
	fs.Parse(args)

	var ahead time.Duration
	if *simulate != "" {
		var err error
		if ahead, err = parseSpan(*simulate); err != nil || ahead < 0 {
			return fmt.Errorf("-simulate %q: want a positive span like 30d, 2w or 36h", *simulate)
		}
	}
	var list []string
	for _, s := range strings.Split(*names, ",") {
		if s = strings.TrimSpace(s); s != "" {
			list = append(list, s)
		}
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	opts := merge(loadConfig(), set, settings{limit: *limit, repos: list, user: *user, refresh: *refresh, decay: *decay})

	src := source{names: opts.repos, owner: opts.user, limit: opts.limit, ttl: *ttl, demo: *demoFlag, decay: opts.decay, state: &sourceState{}}
	if *once || !term.IsTerminal(int(os.Stdout.Fd())) {
		return printOnce(src, ahead, *simulate, opts.sky)
	}
	if !set["ttl"] {
		src.ttl = opts.refresh / 2 // so each background refresh really fetches
	}
	label := ""
	if ahead > 0 {
		label = fmt.Sprintf("simulating %s ahead", *simulate)
	}
	m := live.New(live.Config{Load: src.snapshot, Refresh: opts.refresh, DecayDays: opts.decay, Ahead: ahead,
		Profile: colorProfile(), Label: label, Sky: opts.sky})
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}
```

In `printOnce`:

1. Change its signature to `func printOnce(src source, ahead time.Duration, simulate string, sky scene.SkyMode) error {`.
2. Directly after `	var plots []scene.Plot`, add `	stars := 0`.
3. Replace

```go
		for _, r := range demoGarden(now) {
			plots = append(plots, live.Plot(r, at, src.decay))
		}
```

with

```go
		for _, r := range demoGarden(now) {
			plots = append(plots, live.Plot(r, at, src.decay))
			stars += r.Stars
		}
```

4. Replace

```go
		for _, r := range repos {
			plots = append(plots, live.Plot(repoFor(r, now), at, src.decay))
		}
```

with

```go
		for _, r := range repos {
			plots = append(plots, live.Plot(repoFor(r, now), at, src.decay))
			stars += r.Stars
		}
```

5. Replace `	fmt.Print(header + scene.Compose(termWidth(), plots, at, 1).Encode(colorProfile()) + "\n")` with:

```go
	frame := scene.Draw(scene.View{Cols: termWidth(), Plots: plots, Now: at, Seed: 1, Sky: sky, StarTotal: stars})
	fmt.Print(header + frame.Encode(colorProfile()) + "\n")
```

In `repoFor`, change `		Finished: r.Finished(), Branch: r.Branch, CI: ciFor(r.CI)}` to `		Finished: r.Finished(), Branch: r.Branch, CI: ciFor(r.CI), Stars: r.Stars}`.

For the demo:

1. In `type demoRepo struct`, add the field `	stars      int`.
2. In the `demo` list, add stars to four repos: `stars: 31` to gag-core, `stars: 12` to rustyfs, `stars: 3` to notebook-api, and `stars: 2` to lsystem.
3. In `demoGarden`, change `			Finished: r.finished, Branch: "main", CI: r.ci, NewIssues: r.newIssues, Rising: r.rising}` to `			Finished: r.finished, Branch: "main", CI: r.ci, NewIssues: r.newIssues, Rising: r.rising, Stars: r.stars}`.

- [ ] **Step 5: `gag config` and replay's sky**

`cmd/gag/configcmd.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/RursusAeternum/GitAGarden/internal/config"
)

// runConfig prints where the config file lives and the settings in effect.
func runConfig() error {
	path := config.Path()
	_, err := os.Stat(path)
	fmt.Print(loadConfig().Describe(path, err == nil))
	return nil
}
```

In `cmd/gag/main.go`:

1. In the doc comment, add the line `//	gag config   where the config file is, and the settings in effect` before `//	gag version`.
2. In `main`'s switch, add before `	case "version", "--version", "-v":`:

```go
	case "config":
		err = runConfig()
```

3. Change the unknown-command error to `err = fmt.Errorf("unknown command %q (want garden, replay, config or version)", cmd)`.
4. In `runReplay`, change `	cfg := replay.Config{Name: *name, Species: sp, DecayDays: *decay, Finished: *finished, Profile: colorProfile()}` to:

```go
	cfg := replay.Config{Name: *name, Species: sp, DecayDays: *decay, Finished: *finished, Profile: colorProfile(),
		Sky: skyMode(loadConfig().Sky)}
```

5. In `runReplay`'s `-repo` branch, change `		cfg.Name, cfg.Events, cfg.End = r.Name(), r.Events(), time.Now()` to `		cfg.Name, cfg.Events, cfg.End, cfg.Stars = r.Name(), r.Events(), time.Now(), r.Stars`.

In `internal/replay/model.go`:

1. In `Config`, add after the `Now` field:

```go
	// Sky is what the night sky shows; Stars is the replayed repo's GitHub
	// stars, for the star sky.
	Sky   scene.SkyMode
	Stars int
```

2. Replace `	card := scene.Compose(scene.BedCols, []scene.Plot{plot}, now(), 1).Encode(m.cfg.Profile)` with:

```go
	card := scene.Draw(scene.View{Cols: scene.BedCols, Plots: []scene.Plot{plot}, Now: now(), Seed: 1,
		Sky: m.cfg.Sky, StarTotal: m.cfg.Stars}).Encode(m.cfg.Profile)
```

- [ ] **Step 6: README**

In `README.md`, directly before `## How plants grow`, add:

```markdown
## Config file

GAG reads `~/.config/gag/config` (or `$XDG_CONFIG_HOME/gag/config`): one
`key = value` per line, `#` for comments. Flags on the command line win over
the file. `gag config` shows where the file is and the settings in effect.

```
# ~/.config/gag/config
sky = stars          # the night sky shows your GitHub stars (default: random)
limit = 10
refresh = 2m
# repos = owner/a, owner/b
# user = someone
# decay = 30
```

With `sky = stars` the night sky has one star per GitHub star across the
repos in your garden. Whatever the sky, a shooting star crosses it when one
of your repos gets a new star while GAG is running, and the ticker says which.
```

- [ ] **Step 7: Run everything**

Run: `gofmt -l . ; go vet ./... && go test ./... && go build -o gag ./cmd/gag && ./gag config && XDG_CONFIG_HOME=$(mktemp -d) sh -c 'mkdir -p $XDG_CONFIG_HOME/gag && printf "sky = stars\nlimit = zero\n" > $XDG_CONFIG_HOME/gag/config && ./gag config'`
Expected:
- All tests PASS.
- The first `gag config` prints the real path (`… (not found: using defaults)` if you have no file) and every setting as `default`.
- The second prints `gag: config line 2: limit "zero" isn't a number of repos (1 or more); using 8` on stderr, then `sky      stars   (file)`.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "Config file, gag config, and the star sky everywhere

gag reads ~/.config/gag/config (sky, limit, repos, user, refresh, decay;
flags win), gag config shows it, and sky = stars reaches the live view,
--once and replay. The demo has stars.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Real-world check and v0.5.0 release gate

**Files:**
- None, unless the check turns up bugs.

- [ ] **Step 1: Static check**

Run: `go build -o gag ./cmd/gag && ./gag config && GAG_COLOR=truecolor ./gag garden --once -demo`
Expected: `gag config` lists the settings in effect, and the demo garden prints with whatever sky they select. Do **not** create or edit the user's config file or directory yourself.

- [ ] **Step 2: Hand it to the user**

Ask the user to:
- add `sky = stars` to `~/.config/gag/config`
- run `gag config`, then `./gag` at night (or `./gag -simulate 12h` if it's day, until it's night)
- star one of their own repos on GitHub and press `r`: a shooting star should cross the sky, and the ticker should say `⭐ New star on <repo>`

Fix anything they report, with a failing test first.

- [ ] **Step 3: Release gate**

Merging to `main` and tagging publish a public release. Only on the user's explicit yes:

```bash
git tag -a v0.5.0 -m "v0.5.0: your stars in the sky"
git push origin main v0.5.0
gh workflow run update-gag.yml -R RursusAeternum/homebrew-tap
```
