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
			line = strings.TrimPrefix(line, "\ufeff")
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
			owner, name, ok := strings.Cut(r, "/")
			if !ok || owner == "" || name == "" || strings.Contains(name, "/") || strings.ContainsAny(r, " \t") {
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
