package github

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCommitsNameTheirAgents(t *testing.T) {
	c := fakeGitHub(t, func(q string) string {
		if !strings.Contains(q, "authors(first:3)") {
			t.Errorf("the history query should ask for authors: %s", q)
		}
		return `{"data":{"repository":{"defaultBranchRef":{"target":{"history":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[
			{"committedDate":"2026-09-28T10:00:00Z","messageHeadline":"pair work","authors":{"nodes":[
				{"name":"Kaine","email":"kaine@example.com","user":{"login":"kaine"}},
				{"name":"Claude","email":"noreply@anthropic.com","user":null}]}},
			{"committedDate":"2026-09-28T09:00:00Z","messageHeadline":"codex work","authors":{"nodes":[
				{"name":"chatgpt-codex-connector[bot]","email":"199175422+chatgpt-codex-connector[bot]@users.noreply.github.com","user":null}]}},
			{"committedDate":"2026-09-28T08:00:00Z","messageHeadline":"copilot work","authors":{"nodes":[
				{"name":"Copilot","email":"198982749+Copilot@users.noreply.github.com","user":{"login":"Copilot"}}]}},
			{"committedDate":"2026-09-28T07:00:00Z","messageHeadline":"a painting","authors":{"nodes":[
				{"name":"Claude Monet","email":"claude@monet.fr","user":{"login":"cmonet"}}]}},
			{"committedDate":"2026-09-28T06:00:00Z","messageHeadline":"aider work","authors":{"nodes":[
				{"name":"Paul (aider)","email":"paul@example.com","user":null}]}}
		]}}}}}}`
	})
	commits, _, err := c.commitsSince(context.Background(), "me", "x", time.Time{}, maxCommitPages)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Claude", "Codex", "Copilot", "", "aider"}
	if len(commits) != len(want) {
		t.Fatalf("%d commits, want %d", len(commits), len(want))
	}
	for i, w := range want {
		if commits[i].Agent != w {
			t.Errorf("%q: agent %q, want %q", commits[i].Message, commits[i].Agent, w)
		}
	}
}
