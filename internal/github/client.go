// Package github fetches repo histories from the GitHub GraphQL API and
// caches them locally, so the garden only asks for what changed.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const endpoint = "https://api.github.com/graphql"

type Client struct {
	token string
	hc    *http.Client
}

// ErrNoToken means no GitHub token was found in the environment or the gh CLI.
var ErrNoToken = errors.New("no GitHub token: set GITHUB_TOKEN or run `gh auth login` (or try `gag garden -demo`)")

// NewClient authenticates with GITHUB_TOKEN or GH_TOKEN, falling back to the
// token of the active `gh` CLI account.
func NewClient() (*Client, error) {
	tok := os.Getenv("GITHUB_TOKEN")
	if tok == "" {
		tok = os.Getenv("GH_TOKEN")
	}
	if tok == "" {
		out, err := exec.Command("gh", "auth", "token").Output()
		if err != nil {
			return nil, ErrNoToken
		}
		tok = strings.TrimSpace(string(out))
	}
	return &Client{token: tok, hc: &http.Client{Timeout: 90 * time.Second}}, nil
}

func (c *Client) query(ctx context.Context, q string, vars map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{"query": q, "variables": vars})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("github: %s: %s", resp.Status, bytes.TrimSpace(msg))
	}
	var env struct {
		Data   json.RawMessage
		Errors []struct{ Message string }
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("github: decoding response: %w", err)
	}
	if len(env.Errors) > 0 {
		return fmt.Errorf("github: %s", env.Errors[0].Message)
	}
	return json.Unmarshal(env.Data, out)
}
