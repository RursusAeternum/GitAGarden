package github

import (
	"errors"
	"testing"
)

func TestNewClientWithoutTokenIsErrNoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PATH", t.TempDir()) // no gh CLI to borrow a token from
	if _, err := NewClient(); !errors.Is(err, ErrNoToken) {
		t.Fatalf("err = %v, want ErrNoToken", err)
	}
}
