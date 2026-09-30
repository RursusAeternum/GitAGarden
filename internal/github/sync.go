package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Page caps keep a first sync of a huge repo bounded. Commits come newest
// first, so a capped repo loses its oldest history, not its recent growth.
const (
	maxCommitPages = 30
	maxOtherPages  = 20
)

type pageInfo struct {
	HasNextPage bool
	EndCursor   string
}

// commitsSince fetches default-branch commits newer than since (all of them
// when since is zero), newest first, in at most pages pages of 100. The
// repo's totals come in the same request, so they count the same commits.
func (c *Client) commitsSince(ctx context.Context, owner, name string, since time.Time, pages int) ([]Commit, *Totals, error) {
	q := `query($owner:String!,$name:String!,$after:String,$since:GitTimestamp){repository(owner:$owner,name:$name){` +
		`defaultBranchRef{target{... on Commit{history(first:100,after:$after,since:$since){pageInfo{hasNextPage endCursor} nodes{committedDate messageHeadline authors(first:3){nodes{name email user{login}}}}} all: history{totalCount}}}} ` +
		`merged: pullRequests(states:MERGED){totalCount} releases{totalCount} openIssues: issues(states:OPEN){totalCount}}}`
	vars := map[string]any{"owner": owner, "name": name, "after": nil, "since": nil}
	if !since.IsZero() {
		vars["since"] = since.Add(time.Second).UTC().Format(time.RFC3339)
	}
	type count struct{ TotalCount int }
	var all []Commit
	var totals *Totals
	for page := 0; page < pages; page++ {
		var out struct {
			Repository struct {
				DefaultBranchRef *struct {
					Target struct {
						History *struct {
							PageInfo pageInfo
							Nodes    []struct {
								CommittedDate   time.Time
								MessageHeadline string
								Authors         struct{ Nodes []gitActor }
							}
						}
						All count
					}
				}
				Merged, Releases, OpenIssues count
			}
		}
		if err := c.query(ctx, q, vars, &out); err != nil {
			return nil, nil, err
		}
		rep, ref := out.Repository, out.Repository.DefaultBranchRef
		if totals == nil {
			totals = &Totals{Merged: rep.Merged.TotalCount, Releases: rep.Releases.TotalCount, OpenIssues: rep.OpenIssues.TotalCount}
			if ref != nil {
				totals.Commits = ref.Target.All.TotalCount
			}
		}
		if ref == nil || ref.Target.History == nil { // empty repo
			break
		}
		h := ref.Target.History
		for _, n := range h.Nodes {
			all = append(all, Commit{At: n.CommittedDate, Message: n.MessageHeadline, Agent: agentOf(n.Authors.Nodes)})
		}
		if !h.PageInfo.HasNextPage {
			break
		}
		vars["after"] = h.PageInfo.EndCursor
	}
	return all, totals, nil
}

// connection pages through a repository connection that supports
// orderBy CREATED_AT, oldest first. args are extra connection arguments.
func connection[T any](ctx context.Context, c *Client, owner, name, field, args, nodeFields string) ([]T, error) {
	return pages[T](ctx, c, owner, name, field, args+",orderBy:{field:CREATED_AT,direction:ASC}", nodeFields, 100, maxOtherPages, "", nil)
}

// pages pages through a repository connection from cursor after ("" for
// the start), first nodes a page and at most max pages. args are its
// arguments, orderBy included. A non-nil keep ends it at the first node it
// rejects: the connection is ordered so that every node after that one is
// unwanted too.
func pages[T any](ctx context.Context, c *Client, owner, name, field, args, nodeFields string, first, max int, after string, keep func(T) bool) ([]T, error) {
	q := fmt.Sprintf(`query($owner:String!,$name:String!,$after:String){repository(owner:$owner,name:$name){conn: %s(first:%d,after:$after%s){pageInfo{hasNextPage endCursor} nodes{%s}}}}`,
		field, first, args, nodeFields)
	vars := map[string]any{"owner": owner, "name": name, "after": nil}
	if after != "" {
		vars["after"] = after
	}
	var all []T
	for page := 0; page < max; page++ {
		var out struct {
			Repository struct {
				Conn struct {
					PageInfo pageInfo
					Nodes    []T
				}
			}
		}
		if err := c.query(ctx, q, vars, &out); err != nil {
			return nil, err
		}
		for _, n := range out.Repository.Conn.Nodes {
			if keep != nil && !keep(n) {
				return all, nil
			}
			all = append(all, n)
		}
		if !out.Repository.Conn.PageInfo.HasNextPage {
			break
		}
		vars["after"] = out.Repository.Conn.PageInfo.EndCursor
	}
	return all, nil
}

type releaseNode struct {
	TagName   string
	CreatedAt time.Time
	IsDraft   bool
}

type openPRNode struct {
	Number    int
	Title     string
	CreatedAt time.Time
	IsDraft   bool
}

// recentLists fetches what RecentHistory keeps besides commits, newest
// first: PRs merged since cutoff, the first page of open issues, issues
// closed since cutoff, the newest releases, and the open PRs. The first page
// of each comes in one request; only a list with more to come is paged on.
func (c *Client) recentLists(ctx context.Context, owner, name string, cutoff time.Time) (prs []PR, issues []Issue, rels []releaseNode, open []openPRNode, err error) {
	since := cutoff.UTC().Format(time.RFC3339)
	q := fmt.Sprintf(`query($owner:String!,$name:String!){repository(owner:$owner,name:$name){`+
		`merged: pullRequests(first:100,states:MERGED,orderBy:{field:UPDATED_AT,direction:DESC}){pageInfo{hasNextPage endCursor} nodes{number title mergedAt updatedAt}} `+
		`open: issues(first:100,states:OPEN,orderBy:{field:CREATED_AT,direction:DESC}){pageInfo{hasNextPage endCursor} nodes{number title createdAt closedAt}} `+
		`closed: issues(first:100,states:CLOSED,filterBy:{since:%q},orderBy:{field:UPDATED_AT,direction:DESC}){pageInfo{hasNextPage endCursor} nodes{number title createdAt closedAt}} `+
		`releases(first:%d,orderBy:{field:CREATED_AT,direction:DESC}){pageInfo{hasNextPage endCursor} nodes{tagName createdAt isDraft}} `+
		`openPRs: pullRequests(first:100,states:OPEN,orderBy:{field:CREATED_AT,direction:ASC}){pageInfo{hasNextPage endCursor} nodes{number title createdAt isDraft}}}}`,
		since, recentReleases)
	type prNode struct {
		PR
		UpdatedAt time.Time
	}
	type page[T any] struct {
		PageInfo pageInfo
		Nodes    []T
	}
	var out struct {
		Repository struct {
			Merged   page[prNode]
			Open     page[Issue]
			Closed   page[Issue]
			Releases page[releaseNode]
			OpenPRs  page[openPRNode]
		}
	}
	if err := c.query(ctx, q, map[string]any{"owner": owner, "name": name}, &out); err != nil {
		return nil, nil, nil, nil, err
	}
	rep := out.Repository
	inWindow := func(n prNode) bool { return !n.UpdatedAt.Before(cutoff) }
	merged := rep.Merged.Nodes
	if last := len(merged) - 1; rep.Merged.PageInfo.HasNextPage && last >= 0 && inWindow(merged[last]) {
		more, err := pages(ctx, c, owner, name, "pullRequests", ",states:MERGED,orderBy:{field:UPDATED_AT,direction:DESC}",
			"number title mergedAt updatedAt", 100, maxOtherPages, rep.Merged.PageInfo.EndCursor, inWindow)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("pull requests: %w", err)
		}
		merged = append(merged, more...)
	}
	for _, n := range merged {
		if !n.MergedAt.Before(cutoff) {
			prs = append(prs, n.PR)
		}
	}
	closed := rep.Closed.Nodes
	if rep.Closed.PageInfo.HasNextPage {
		more, err := pages[Issue](ctx, c, owner, name, "issues",
			fmt.Sprintf(",states:CLOSED,filterBy:{since:%q},orderBy:{field:UPDATED_AT,direction:DESC}", since),
			"number title createdAt closedAt", 100, maxOtherPages, rep.Closed.PageInfo.EndCursor, nil)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("issues: %w", err)
		}
		closed = append(closed, more...)
	}
	issues = rep.Open.Nodes
	for _, is := range closed {
		if is.ClosedAt != nil && !is.ClosedAt.Before(cutoff) {
			issues = append(issues, is)
		}
	}
	open = rep.OpenPRs.Nodes
	if rep.OpenPRs.PageInfo.HasNextPage {
		more, err := pages[openPRNode](ctx, c, owner, name, "pullRequests", ",states:OPEN,orderBy:{field:CREATED_AT,direction:ASC}",
			"number title createdAt isDraft", 100, maxOtherPages, rep.OpenPRs.PageInfo.EndCursor, nil)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("open pull requests: %w", err)
		}
		open = append(open, more...)
	}
	return prs, issues, rep.Releases.Nodes, open, nil
}

// fetch fills in r's totals and history. Commits are fetched incrementally
// on top of cached ones, and the totals come with them. With FullHistory,
// PRs, issues and releases are refetched whole, which also picks up issues
// closed since last time; with RecentHistory only the recent ones are, in
// one request for a small repo.
func (c *Client) fetch(ctx context.Context, r *Repo, cached *Repo) error {
	owner, name, _ := strings.Cut(r.NameWithOwner, "/")
	recent := c.History == RecentHistory
	cutoff := time.Now().Add(-RecentWindow)

	commitsWhole := !recent
	var since time.Time
	if cached != nil && (recent || cached.CommitsComplete()) { // full history can't build on recent commits
		r.Commits, commitsWhole = cached.Commits, cached.CommitsComplete()
		for _, cm := range cached.Commits {
			if cm.At.After(since) {
				since = cm.At
			}
		}
	}
	if since.IsZero() && recent {
		since = cutoff
	}
	newer, totals, err := c.commitsSince(ctx, owner, name, since, maxCommitPages)
	if err != nil {
		return fmt.Errorf("commits: %w", err)
	}
	r.Commits, r.Totals = append(r.Commits, newer...), totals
	if recent && len(r.Commits) == 0 { // a quiet repo: its newest commits still tell when it was tended
		if r.Commits, _, err = c.commitsSince(ctx, owner, name, time.Time{}, 1); err != nil {
			return fmt.Errorf("commits: %w", err)
		}
	}
	switch {
	case !recent:
		r.History = "full"
	case commitsWhole: // recent lists on whole commits: replay must still fetch the lists
		r.History = "commits"
	default:
		r.History = "recent"
	}

	var prs []PR
	var issues []Issue
	var rels []releaseNode
	var open []openPRNode
	if recent {
		if prs, issues, rels, open, err = c.recentLists(ctx, owner, name, cutoff); err != nil {
			return fmt.Errorf("recent history: %w", err)
		}
	} else {
		if prs, err = connection[PR](ctx, c, owner, name, "pullRequests", ",states:MERGED", "number title mergedAt"); err != nil {
			return fmt.Errorf("pull requests: %w", err)
		}
		if issues, err = connection[Issue](ctx, c, owner, name, "issues", "", "number title createdAt closedAt"); err != nil {
			return fmt.Errorf("issues: %w", err)
		}
		if rels, err = connection[releaseNode](ctx, c, owner, name, "releases", "", "tagName createdAt isDraft"); err != nil {
			return fmt.Errorf("releases: %w", err)
		}
		if open, err = connection[openPRNode](ctx, c, owner, name, "pullRequests", ",states:OPEN", "number title createdAt isDraft"); err != nil {
			return fmt.Errorf("open pull requests: %w", err)
		}
	}
	r.PRs, r.Issues, r.Releases = prs, issues, nil
	for _, rel := range rels {
		if !rel.IsDraft {
			r.Releases = append(r.Releases, Release{Tag: rel.TagName, CreatedAt: rel.CreatedAt})
		}
	}
	// Tokens without checks access get FORBIDDEN here: CI stays unknown
	// (clear skies) rather than losing the rest of the repo. So it stays a
	// request of its own.
	branch, ci, err := c.branchStatus(ctx, owner, name)
	if err != nil {
		branch, ci = "", ""
	}
	r.OpenPRs, r.Branch, r.CI = nil, branch, ci
	for _, n := range open {
		r.OpenPRs = append(r.OpenPRs, OpenPR{Number: n.Number, Title: n.Title, CreatedAt: n.CreatedAt, Draft: n.IsDraft})
	}
	r.FetchedAt = time.Now()
	return nil
}

// Store is the on-disk cache of repo histories.
type Store struct {
	path  string
	Repos map[string]*Repo `json:"repos"`
	// Viewer is the login whose repos ListRepos last returned, so the offline
	// fallback shows your own garden and not repos cached by -user or -repos.
	Viewer string `json:"viewer,omitempty"`
}

// OpenStore loads the cache from the user cache dir (~/.cache/gag on Linux,
// ~/Library/Caches/gag on macOS). A missing or unreadable cache starts empty.
func OpenStore() (*Store, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "gag", "repos.json"), Repos: map[string]*Repo{}}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	} else if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil || s.Repos == nil {
		s.Repos = map[string]*Repo{} // corrupt cache: refetch rather than fail
	}
	return s, nil
}

func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Sync brings each repo's history up to date, skipping any fetched within
// ttl. It saves after every repo so progress survives a slow or dropped
// connection. Repos that fail to refresh fall back to their cached copy;
// the returned error reports the first failure. progress, if not nil, hears
// (i, n, name) before each repo and (n, n, "") at the end.
func Sync(ctx context.Context, c *Client, s *Store, metas []*Repo, ttl time.Duration, log func(string), progress func(done, total int, current string)) ([]*Repo, error) {
	var out []*Repo
	var firstErr error
	report := func(done int, current string) {
		if progress != nil {
			progress(done, len(metas), current)
		}
	}
	for i, m := range metas {
		report(i, m.NameWithOwner)
		cached := s.Repos[m.NameWithOwner]
		if cached != nil && time.Since(cached.FetchedAt) < ttl && (cached.Complete() || c == nil || c.History == RecentHistory) {
			m.Commits, m.PRs, m.Issues, m.Releases, m.FetchedAt = cached.Commits, cached.PRs, cached.Issues, cached.Releases, cached.FetchedAt
			m.OpenPRs, m.Branch, m.CI = cached.OpenPRs, cached.Branch, cached.CI
			m.Totals, m.History = cached.Totals, cached.History
			s.Repos[m.NameWithOwner] = m
			out = append(out, m)
			continue
		}
		if log != nil {
			log("fetching " + m.NameWithOwner)
		}
		if err := c.fetch(ctx, m, cached); err != nil {
			err = fmt.Errorf("%s: %w", m.NameWithOwner, err)
			if log != nil {
				log(err.Error())
			}
			if firstErr == nil {
				firstErr = err
			}
			if cached != nil {
				out = append(out, cached)
			}
			continue
		}
		s.Repos[m.NameWithOwner] = m
		if err := s.Save(); err != nil && firstErr == nil {
			firstErr = err
		}
		out = append(out, m)
	}
	report(len(metas), "")
	return out, firstErr
}

// branchStatus returns the default branch's name and the combined CI state
// of its latest commit: SUCCESS, FAILURE, ERROR, PENDING, EXPECTED, or ""
// when the commit has no checks or the repo is empty.
func (c *Client) branchStatus(ctx context.Context, owner, name string) (branch, state string, err error) {
	q := `query($owner:String!,$name:String!){repository(owner:$owner,name:$name){defaultBranchRef{name target{... on Commit{statusCheckRollup{state}}}}}}`
	var out struct {
		Repository struct {
			DefaultBranchRef *struct {
				Name   string
				Target struct {
					StatusCheckRollup *struct{ State string }
				}
			}
		}
	}
	if err := c.query(ctx, q, map[string]any{"owner": owner, "name": name}, &out); err != nil {
		return "", "", err
	}
	ref := out.Repository.DefaultBranchRef
	if ref == nil {
		return "", "", nil
	}
	if ref.Target.StatusCheckRollup != nil {
		state = ref.Target.StatusCheckRollup.State
	}
	return ref.Name, state, nil
}
