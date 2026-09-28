package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func TestGagConfigShowsTheFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "gag", "config")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("sky = stars\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := runConfig(&b); err != nil {
		t.Fatal(err)
	}
	if want := "config: " + path + "\nsky      stars   (file)\n"; !strings.HasPrefix(b.String(), want) {
		t.Errorf("gag config printed\n%s\nwant it to start\n%s", b.String(), want)
	}
}

func TestOnceDrawsTheConfiguredSky(t *testing.T) {
	now := time.Now()
	v, err := onceView(source{demo: true, decay: 45}, now, now, scene.SkyStars, 100)
	if err != nil {
		t.Fatal(err)
	}
	if v.Sky != scene.SkyStars || v.StarTotal != 48 || v.Cols != 100 || len(v.Plots) != len(demo) {
		t.Errorf("--once view: sky %v, %d stars, %d cols, %d plots", v.Sky, v.StarTotal, v.Cols, len(v.Plots))
	}
}
