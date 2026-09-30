package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
)

type gqlCall struct {
	Query     string
	Variables map[string]any
}

// recordingGitHub is fakeGitHub that also keeps every call, with its
// variables, for the test to inspect.
func recordingGitHub(t *testing.T, answer func(gqlCall) string) (*Client, func() []gqlCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []gqlCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body gqlCall
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		mu.Lock()
		calls = append(calls, body)
		mu.Unlock()
		io.WriteString(w, answer(body))
	}))
	t.Cleanup(srv.Close)
	return &Client{token: "test", hc: srv.Client(), url: srv.URL}, func() []gqlCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]gqlCall(nil), calls...)
	}
}

const totalsAnswer = `{"data":{"repository":{"defaultBranchRef":{"target":{"history":{"totalCount":4200}}},
	"merged":{"totalCount":900},"releases":{"totalCount":35},"openIssues":{"totalCount":120}}}}`

func conn(nodes string) string {
	return `{"data":{"repository":{"conn":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[` + nodes + `]}}}}`
}

func history(commits string) string {
	return `{"data":{"repository":{"defaultBranchRef":{"target":{"history":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[` + commits + `]}}}}}}`
}

func ts(d time.Duration) string { return time.Now().Add(-d).UTC().Format(time.RFC3339) }

const day = 24 * time.Hour

func TestRecentFetchesTotalsAndRecentHistory(t *testing.T) {
	c, calls := recordingGitHub(t, func(call gqlCall) string {
		q := call.Query
		switch {
		case strings.Contains(q, "totalCount"):
			return totalsAnswer
		case strings.Contains(q, "history("):
			return history(`{"committedDate":"` + ts(2*day) + `","messageHeadline":"recent","authors":{"nodes":[]}}`)
		case strings.Contains(q, "states:MERGED"):
			return conn(`{"number":41,"title":"in the window","mergedAt":"` + ts(3*day) + `","updatedAt":"` + ts(3*day) + `"},
				{"number":12,"title":"merged long ago, commented on lately","mergedAt":"` + ts(400*day) + `","updatedAt":"` + ts(5*day) + `"},
				{"number":9,"title":"past the window","mergedAt":"` + ts(200*day) + `","updatedAt":"` + ts(100*day) + `"}`)
		case strings.Contains(q, "states:CLOSED"):
			return conn(`{"number":30,"title":"closed lately","createdAt":"` + ts(300*day) + `","closedAt":"` + ts(4*day) + `"},
				{"number":20,"title":"closed long ago, edited lately","createdAt":"` + ts(300*day) + `","closedAt":"` + ts(200*day) + `"}`)
		case strings.Contains(q, "issues(") && strings.Contains(q, "states:OPEN"):
			return conn(`{"number":31,"title":"open","createdAt":"` + ts(1*day) + `","closedAt":null}`)
		case strings.Contains(q, "releases("):
			return conn(`{"tagName":"v2.0.0","createdAt":"` + ts(700*day) + `","isDraft":false}`)
		}
		return emptyConn
	})
	c.History = RecentHistory
	r := &Repo{NameWithOwner: "me/big"}
	if err := c.fetch(context.Background(), r, nil); err != nil {
		t.Fatal(err)
	}
	if want := (Totals{Commits: 4200, Merged: 900, Releases: 35, OpenIssues: 120}); r.Totals == nil || *r.Totals != want {
		t.Errorf("totals = %+v, want %+v", r.Totals, want)
	}
	if r.History != "recent" || r.Complete() {
		t.Errorf("history %q: a recent fetch is not complete", r.History)
	}
	if len(r.Commits) != 1 || len(r.PRs) != 1 || r.PRs[0].Number != 41 {
		t.Errorf("commits %d, PRs %+v; want the one commit and #41", len(r.Commits), r.PRs)
	}
	if len(r.Issues) != 2 || r.Issues[0].Number != 31 || r.Issues[1].Number != 30 {
		t.Errorf("issues %+v; want open #31 and #30 closed lately", r.Issues)
	}
	if len(r.Releases) != 1 || r.Releases[0].Tag != "v2.0.0" {
		t.Errorf("releases %+v; want the newest, however old", r.Releases)
	}
	for _, call := range calls() {
		q := call.Query
		switch {
		case strings.Contains(q, "history("):
			since, _ := call.Variables["since"].(string)
			if at, err := time.Parse(time.RFC3339, since); err != nil || time.Since(at) > RecentWindow+day || time.Since(at) < RecentWindow-day {
				t.Errorf("commits since %q; want about 90 days ago", since)
			}
		case strings.Contains(q, "direction:ASC") && !strings.Contains(q, "pullRequests(first:100,after:$after,states:OPEN"): // open PRs: as before
			t.Errorf("a recent fetch should not page through whole histories: %s", q)
		case strings.Contains(q, "releases(") && !strings.Contains(q, "first:10,"):
			t.Errorf("releases: want the ten newest: %s", q)
		}
	}
}

func TestRecentQuietRepoKeepsItsNewestCommit(t *testing.T) {
	c, _ := recordingGitHub(t, func(call gqlCall) string {
		switch q := call.Query; {
		case strings.Contains(q, "totalCount"):
			return totalsAnswer
		case strings.Contains(q, "history("):
			if call.Variables["since"] != nil {
				return history(``) // nothing in the last 90 days
			}
			return history(`{"committedDate":"` + ts(300*day) + `","messageHeadline":"last one","authors":{"nodes":[]}}`)
		}
		return emptyConn
	})
	c.History = RecentHistory
	r := &Repo{NameWithOwner: "me/quiet"}
	if err := c.fetch(context.Background(), r, nil); err != nil {
		t.Fatal(err)
	}
	if len(r.Commits) != 1 || r.Commits[0].Message != "last one" {
		t.Errorf("commits %+v; want the newest one even outside the window", r.Commits)
	}
}

func TestCompleteCacheServesRecent(t *testing.T) {
	newest := time.Now().Add(-10 * day).UTC().Truncate(time.Second)
	cached := &Repo{NameWithOwner: "me/x", Commits: []Commit{{At: newest.Add(-900 * day)}, {At: newest}}} // from before v0.8: complete
	c, calls := recordingGitHub(t, func(call gqlCall) string {
		if strings.Contains(call.Query, "totalCount") {
			return totalsAnswer
		}
		if strings.Contains(call.Query, "history(") {
			return history(``)
		}
		return emptyConn
	})
	c.History = RecentHistory
	r := &Repo{NameWithOwner: "me/x"}
	if err := c.fetch(context.Background(), r, cached); err != nil {
		t.Fatal(err)
	}
	if !r.Complete() || len(r.Commits) != 2 {
		t.Errorf("history %q with %d commits: a complete cache stays complete", r.History, len(r.Commits))
	}
	for _, call := range calls() {
		if strings.Contains(call.Query, "history(") && call.Variables["since"] != newest.Add(time.Second).Format(time.RFC3339) {
			t.Errorf("commits since %v; want only those after the newest cached one", call.Variables["since"])
		}
	}
}

func TestFullFetchesWhatARecentCacheLacks(t *testing.T) {
	cached := &Repo{NameWithOwner: "me/x", History: "recent", FetchedAt: time.Now(), Commits: []Commit{{At: time.Now().Add(-day)}}}
	c, calls := recordingGitHub(t, func(call gqlCall) string {
		if strings.Contains(call.Query, "totalCount") {
			return totalsAnswer
		}
		if strings.Contains(call.Query, "history(") {
			return history(`{"committedDate":"` + ts(day) + `","messageHeadline":"a","authors":{"nodes":[]}},
				{"committedDate":"` + ts(900*day) + `","messageHeadline":"b","authors":{"nodes":[]}}`)
		}
		return emptyConn
	})
	s := &Store{path: filepath.Join(t.TempDir(), "repos.json"), Repos: map[string]*Repo{"me/x": cached}}
	repos, err := Sync(context.Background(), c, s, []*Repo{{NameWithOwner: "me/x"}}, time.Hour, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fetched := len(calls()) > 0; !fetched || !repos[0].Complete() || len(repos[0].Commits) != 2 {
		t.Errorf("fetched %v, history %q, %d commits: full history must fetch past a fresh recent cache",
			len(calls()) > 0, repos[0].History, len(repos[0].Commits))
	}
	for _, call := range calls() {
		if strings.Contains(call.Query, "history(") && call.Variables["since"] != nil {
			t.Errorf("commits since %v; want all of them", call.Variables["since"])
		}
	}
}

func TestOldCachesHaveNoTotals(t *testing.T) {
	old := `{"repos":{"me/x":{"nameWithOwner":"me/x","commits":[{"at":"2026-01-01T00:00:00Z","msg":"a"}],"fetchedAt":"2026-09-01T00:00:00Z"}}}`
	var s Store
	if err := json.Unmarshal([]byte(old), &s); err != nil {
		t.Fatal(err)
	}
	r := s.Repos["me/x"]
	if r.Totals != nil || r.GardenTotals() != (garden.Totals{}) || !r.Complete() {
		t.Errorf("a v0.7 cache: totals %+v, complete %v; want none, and complete", r.Totals, r.Complete())
	}
}

func TestRecentEmptyRepo(t *testing.T) {
	c, _ := recordingGitHub(t, func(call gqlCall) string {
		switch q := call.Query; {
		case strings.Contains(q, "totalCount"):
			return `{"data":{"repository":{"defaultBranchRef":null,"merged":{"totalCount":0},"releases":{"totalCount":0},"openIssues":{"totalCount":0}}}}`
		case strings.Contains(q, "history("):
			return `{"data":{"repository":{"defaultBranchRef":null}}}`
		}
		return emptyConn
	})
	c.History = RecentHistory
	r := &Repo{NameWithOwner: "me/empty"}
	if err := c.fetch(context.Background(), r, nil); err != nil {
		t.Fatalf("an empty repo should fetch cleanly: %v", err)
	}
	if r.Totals == nil || r.Totals.Commits != 0 || len(r.Commits) != 0 || r.History != "recent" {
		t.Errorf("totals %+v, %d commits, history %q", r.Totals, len(r.Commits), r.History)
	}
}
