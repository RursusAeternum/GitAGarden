package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	src := "\ufeff# my garden\r\nsky = Stars  # opt in\r\n\r\nLIMIT=12\nrepos = me/a, other/b\nuser = octocat\nrefresh = 2m\ndecay = 30\n"
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

func TestReposMustBeOwnerSlashName(t *testing.T) {
	for _, line := range []string{"repos = me/a me/b", "repos = /a", "repos = me/", "repos = me/a/b", "repos = me/a, other"} {
		c, warns := Parse(strings.NewReader(line))
		if len(warns) != 1 || c.Repos != nil || c.From["repos"] != Default {
			t.Errorf("%q: repos %v from %q, warnings %v; want one warning and no repos", line, c.Repos, c.From["repos"], warns)
		}
	}
}

func TestDecayMustBeAFiniteNumber(t *testing.T) {
	for _, v := range []string{"nan", "NaN", "inf", "+Inf", "-inf"} {
		c, warns := Parse(strings.NewReader("decay = " + v))
		if len(warns) != 1 || c.Decay != 45 {
			t.Errorf("decay = %s: decay %v, warnings %v; want one warning and 45", v, c.Decay, warns)
		}
	}
}
