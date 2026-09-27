package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeGitHub answers GraphQL queries with whatever answer returns for the
// query text.
func fakeGitHub(t *testing.T, answer func(query string) string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Query string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		io.WriteString(w, answer(body.Query))
	}))
	t.Cleanup(srv.Close)
	return &Client{token: "test", hc: srv.Client(), url: srv.URL}
}

const emptyConn = `{"data":{"repository":{"conn":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}`

func TestFetchReadsOpenPRsAndCI(t *testing.T) {
	c := fakeGitHub(t, func(q string) string {
		switch {
		case strings.Contains(q, "statusCheckRollup"):
			return `{"data":{"repository":{"defaultBranchRef":{"name":"main","target":{"statusCheckRollup":{"state":"FAILURE"}}}}}}`
		case strings.Contains(q, "history("):
			return `{"data":{"repository":{"defaultBranchRef":null}}}`
		case strings.Contains(q, "states:OPEN"):
			return `{"data":{"repository":{"conn":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[
				{"number":7,"title":"Add bees","createdAt":"2026-09-01T10:00:00Z","isDraft":false},
				{"number":8,"title":"WIP","createdAt":"2026-09-20T10:00:00Z","isDraft":true}]}}}}`
		}
		return emptyConn
	})
	r := &Repo{NameWithOwner: "me/garden"}
	if err := c.fetch(context.Background(), r, nil); err != nil {
		t.Fatal(err)
	}
	if r.Branch != "main" || r.CI != "FAILURE" {
		t.Errorf("branch %q, ci %q; want main, FAILURE", r.Branch, r.CI)
	}
	if len(r.OpenPRs) != 2 || r.OpenPRs[0].Number != 7 || r.OpenPRs[0].Draft || !r.OpenPRs[1].Draft {
		t.Errorf("open PRs = %+v", r.OpenPRs)
	}
}

func TestFetchWithoutChecksHasNoCI(t *testing.T) {
	c := fakeGitHub(t, func(q string) string {
		switch {
		case strings.Contains(q, "statusCheckRollup"):
			return `{"data":{"repository":{"defaultBranchRef":{"name":"trunk","target":{"statusCheckRollup":null}}}}}`
		case strings.Contains(q, "history("):
			return `{"data":{"repository":{"defaultBranchRef":null}}}`
		}
		return emptyConn
	})
	r := &Repo{NameWithOwner: "me/quiet"}
	if err := c.fetch(context.Background(), r, nil); err != nil {
		t.Fatal(err)
	}
	if r.Branch != "trunk" || r.CI != "" || len(r.OpenPRs) != 0 {
		t.Errorf("branch %q, ci %q, open PRs %d; want trunk, no CI, none", r.Branch, r.CI, len(r.OpenPRs))
	}
}

func TestSyncKeepsSignalsFromCache(t *testing.T) {
	cached := &Repo{NameWithOwner: "me/x", FetchedAt: time.Now(), CI: "SUCCESS", Branch: "main",
		OpenPRs: []OpenPR{{Number: 1, CreatedAt: time.Now()}}}
	s := &Store{Repos: map[string]*Repo{"me/x": cached}}
	repos, err := Sync(context.Background(), nil, s, []*Repo{{NameWithOwner: "me/x"}}, time.Hour, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r := repos[0]; r.CI != "SUCCESS" || r.Branch != "main" || len(r.OpenPRs) != 1 {
		t.Errorf("cached signals lost: ci %q, branch %q, open PRs %d", r.CI, r.Branch, len(r.OpenPRs))
	}
}

func TestOldCachesLoadWithoutSignals(t *testing.T) {
	old := `{"repos":{"me/x":{"nameWithOwner":"me/x","commits":[],"fetchedAt":"2026-09-01T00:00:00Z"}}}`
	var s Store
	if err := json.Unmarshal([]byte(old), &s); err != nil {
		t.Fatal(err)
	}
	if r := s.Repos["me/x"]; r.CI != "" || r.Branch != "" || r.OpenPRs != nil {
		t.Errorf("a v0.3 cache should load with no signals, got %+v", r)
	}
}

func TestSyncReportsProgress(t *testing.T) {
	s := &Store{Repos: map[string]*Repo{}}
	var metas []*Repo
	for _, n := range []string{"me/a", "me/b"} {
		s.Repos[n] = &Repo{NameWithOwner: n, FetchedAt: time.Now()}
		metas = append(metas, &Repo{NameWithOwner: n})
	}
	var got []string
	Sync(context.Background(), nil, s, metas, time.Hour, nil, func(done, total int, current string) {
		got = append(got, fmt.Sprintf("%d/%d %s", done, total, current))
	})
	if want := "0/2 me/a|1/2 me/b|2/2 "; strings.Join(got, "|") != want {
		t.Errorf("progress = %q, want %q", strings.Join(got, "|"), want)
	}
}
