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

func ts(d time.Duration) string { return time.Now().Add(-d).UTC().Format(time.RFC3339) }

const day = 24 * time.Hour

func TestRecentFetchesTotalsAndRecentHistory(t *testing.T) {
	c, calls := recordingGitHub(t, func(call gqlCall) string {
		q := call.Query
		switch {
		case strings.Contains(q, "history("):
			return commitsAnswer(`{"committedDate":"` + ts(2*day) + `","messageHeadline":"recent","authors":{"nodes":[]}}`)
		case strings.Contains(q, "closed: issues("):
			return listsAnswer(
				`{"number":41,"title":"in the window","mergedAt":"`+ts(3*day)+`","updatedAt":"`+ts(3*day)+`"},
				{"number":12,"title":"merged long ago, commented on lately","mergedAt":"`+ts(400*day)+`","updatedAt":"`+ts(5*day)+`"},
				{"number":9,"title":"past the window","mergedAt":"`+ts(200*day)+`","updatedAt":"`+ts(100*day)+`"}`,
				`{"number":31,"title":"open","createdAt":"`+ts(1*day)+`","closedAt":null}`,
				`{"number":30,"title":"closed lately","createdAt":"`+ts(300*day)+`","closedAt":"`+ts(4*day)+`"},
				{"number":20,"title":"closed long ago, edited lately","createdAt":"`+ts(300*day)+`","closedAt":"`+ts(200*day)+`"}`,
				`{"tagName":"v2.0.0","createdAt":"`+ts(700*day)+`","isDraft":false}`, ``)
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
		case strings.Contains(q, "closed: issues(") && !strings.Contains(q, "releases(first:10,"):
			t.Errorf("releases: want the ten newest: %s", q)
		case strings.Contains(q, "after:$after") && strings.Contains(q, "direction:ASC"):
			t.Errorf("a recent fetch should not page through whole histories: %s", q)
		}
	}
}

func TestRecentQuietRepoKeepsItsNewestCommit(t *testing.T) {
	c, _ := recordingGitHub(t, func(call gqlCall) string {
		if strings.Contains(call.Query, "history(") {
			if call.Variables["since"] != nil {
				return commitsAnswer(``) // nothing in the last 90 days
			}
			return commitsAnswer(`{"committedDate":"` + ts(300*day) + `","messageHeadline":"last one","authors":{"nodes":[]}}`)
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
		if strings.Contains(call.Query, "history(") {
			return commitsAnswer(``)
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
		if strings.Contains(call.Query, "history(") {
			return commitsAnswer(`{"committedDate":"` + ts(day) + `","messageHeadline":"a","authors":{"nodes":[]}},
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
		if strings.Contains(call.Query, "history(") {
			return `{"data":{"repository":{"defaultBranchRef":null,"merged":{"totalCount":0},"releases":{"totalCount":0},"openIssues":{"totalCount":0}}}}`
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

// commitsAnswer answers the commits request: the commits, and the repo's
// totals, which come in the same request.
func commitsAnswer(commits string) string {
	return `{"data":{"repository":{"defaultBranchRef":{"target":{
		"history":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[` + commits + `]},"all":{"totalCount":4200}}},
		"merged":{"totalCount":900},"releases":{"totalCount":35},"openIssues":{"totalCount":120}}}}`
}

// listsAnswer answers the recent-lists request, one page of each list.
func listsAnswer(merged, open, closed, releases, openPRs string) string {
	page := func(nodes string) string {
		return `{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[` + nodes + `]}`
	}
	return `{"data":{"repository":{"merged":` + page(merged) + `,"open":` + page(open) + `,"closed":` + page(closed) +
		`,"releases":` + page(releases) + `,"openPRs":` + page(openPRs) + `}}}`
}

func TestRecentFetchOfASmallRepoIsThreeRequests(t *testing.T) {
	c, calls := recordingGitHub(t, func(call gqlCall) string {
		switch q := call.Query; {
		case strings.Contains(q, "statusCheckRollup"):
			return `{"data":{"repository":{"defaultBranchRef":{"name":"main","target":{"statusCheckRollup":{"state":"SUCCESS"}}}}}}`
		case strings.Contains(q, "history("):
			return commitsAnswer(`{"committedDate":"` + ts(2*day) + `","messageHeadline":"recent","authors":{"nodes":[]}}`)
		case strings.Contains(q, "closed: issues("):
			return listsAnswer(
				`{"number":41,"title":"in the window","mergedAt":"`+ts(3*day)+`","updatedAt":"`+ts(3*day)+`"}`,
				`{"number":31,"title":"open","createdAt":"`+ts(1*day)+`","closedAt":null}`,
				`{"number":30,"title":"closed lately","createdAt":"`+ts(300*day)+`","closedAt":"`+ts(4*day)+`"}`,
				`{"tagName":"v2.0.0","createdAt":"`+ts(700*day)+`","isDraft":false}`,
				`{"number":7,"title":"Add bees","createdAt":"`+ts(5*day)+`","isDraft":false}`)
		}
		return emptyConn
	})
	c.History = RecentHistory
	r := &Repo{NameWithOwner: "me/small"}
	if err := c.fetch(context.Background(), r, nil); err != nil {
		t.Fatal(err)
	}
	if n := len(calls()); n != 3 {
		t.Errorf("%d requests; want 3: commits with totals, the recent lists, and CI", n)
	}
	if want := (Totals{Commits: 4200, Merged: 900, Releases: 35, OpenIssues: 120}); r.Totals == nil || *r.Totals != want {
		t.Errorf("totals = %+v, want %+v, from the commits request", r.Totals, want)
	}
	if len(r.Commits) != 1 || len(r.PRs) != 1 || len(r.Issues) != 2 || len(r.Releases) != 1 || len(r.OpenPRs) != 1 || r.CI != "SUCCESS" {
		t.Errorf("commits %d, PRs %d, issues %d, releases %d, open PRs %d, CI %q",
			len(r.Commits), len(r.PRs), len(r.Issues), len(r.Releases), len(r.OpenPRs), r.CI)
	}
}

func TestRecentListsPageOnOnlyInsideTheWindow(t *testing.T) {
	c, calls := recordingGitHub(t, func(call gqlCall) string {
		switch q := call.Query; {
		case strings.Contains(q, "history("):
			return commitsAnswer(`{"committedDate":"` + ts(day) + `","messageHeadline":"x","authors":{"nodes":[]}}`)
		case strings.Contains(q, "closed: issues("):
			return strings.Replace(listsAnswer(`{"number":50,"title":"new","mergedAt":"`+ts(1*day)+`","updatedAt":"`+ts(1*day)+`"}`, ``, ``, ``, ``),
				`"merged":{"pageInfo":{"hasNextPage":false,"endCursor":""}`, `"merged":{"pageInfo":{"hasNextPage":true,"endCursor":"c1"}`, 1)
		case strings.Contains(q, "states:MERGED"):
			if call.Variables["after"] != "c1" {
				t.Errorf("paging merged PRs from %v, want the first request's cursor", call.Variables["after"])
			}
			return `{"data":{"repository":{"conn":{"pageInfo":{"hasNextPage":true,"endCursor":"c2"},"nodes":[
				{"number":49,"title":"still in","mergedAt":"` + ts(20*day) + `","updatedAt":"` + ts(20*day) + `"},
				{"number":10,"title":"past it","mergedAt":"` + ts(300*day) + `","updatedAt":"` + ts(200*day) + `"}]}}}}`
		}
		return emptyConn
	})
	c.History = RecentHistory
	r := &Repo{NameWithOwner: "me/busy"}
	if err := c.fetch(context.Background(), r, nil); err != nil {
		t.Fatal(err)
	}
	if len(r.PRs) != 2 || r.PRs[1].Number != 49 {
		t.Errorf("PRs %+v; want #50 and #49, and nothing past the window", r.PRs)
	}
	if n := len(calls()); n != 4 {
		t.Errorf("%d requests; want 4: commits, the lists, one more page of merged PRs, CI", n)
	}
}
