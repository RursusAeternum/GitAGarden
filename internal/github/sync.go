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
// when since is zero), newest first.
func (c *Client) commitsSince(ctx context.Context, owner, name string, since time.Time) ([]Commit, error) {
	q := `query($owner:String!,$name:String!,$after:String,$since:GitTimestamp){repository(owner:$owner,name:$name){defaultBranchRef{target{... on Commit{history(first:100,after:$after,since:$since){pageInfo{hasNextPage endCursor} nodes{committedDate messageHeadline}}}}}}}`
	vars := map[string]any{"owner": owner, "name": name, "after": nil, "since": nil}
	if !since.IsZero() {
		vars["since"] = since.Add(time.Second).UTC().Format(time.RFC3339)
	}
	var all []Commit
	for page := 0; page < maxCommitPages; page++ {
		var out struct {
			Repository struct {
				DefaultBranchRef *struct {
					Target struct {
						History *struct {
							PageInfo pageInfo
							Nodes    []struct {
								CommittedDate   time.Time
								MessageHeadline string
							}
						}
					}
				}
			}
		}
		if err := c.query(ctx, q, vars, &out); err != nil {
			return nil, err
		}
		ref := out.Repository.DefaultBranchRef
		if ref == nil || ref.Target.History == nil { // empty repo
			break
		}
		h := ref.Target.History
		for _, n := range h.Nodes {
			all = append(all, Commit{At: n.CommittedDate, Message: n.MessageHeadline})
		}
		if !h.PageInfo.HasNextPage {
			break
		}
		vars["after"] = h.PageInfo.EndCursor
	}
	return all, nil
}

// connection pages through a repository connection that supports
// orderBy CREATED_AT, oldest first. args are extra connection arguments.
func connection[T any](ctx context.Context, c *Client, owner, name, field, args, nodeFields string) ([]T, error) {
	q := fmt.Sprintf(`query($owner:String!,$name:String!,$after:String){repository(owner:$owner,name:$name){conn: %s(first:100,after:$after,orderBy:{field:CREATED_AT,direction:ASC}%s){pageInfo{hasNextPage endCursor} nodes{%s}}}}`,
		field, args, nodeFields)
	vars := map[string]any{"owner": owner, "name": name, "after": nil}
	var all []T
	for page := 0; page < maxOtherPages; page++ {
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
		all = append(all, out.Repository.Conn.Nodes...)
		if !out.Repository.Conn.PageInfo.HasNextPage {
			break
		}
		vars["after"] = out.Repository.Conn.PageInfo.EndCursor
	}
	return all, nil
}

// fetch fills in r's history. Commits are fetched incrementally on top of
// cached ones; PRs, issues and releases are small enough to refetch whole,
// which also picks up issues closed since last time.
func (c *Client) fetch(ctx context.Context, r *Repo, cached *Repo) error {
	owner, name, _ := strings.Cut(r.NameWithOwner, "/")

	var since time.Time
	if cached != nil {
		r.Commits = cached.Commits
		for _, cm := range cached.Commits {
			if cm.At.After(since) {
				since = cm.At
			}
		}
	}
	newer, err := c.commitsSince(ctx, owner, name, since)
	if err != nil {
		return fmt.Errorf("commits: %w", err)
	}
	r.Commits = append(r.Commits, newer...)

	prs, err := connection[PR](ctx, c, owner, name, "pullRequests", ",states:MERGED", "number title mergedAt")
	if err != nil {
		return fmt.Errorf("pull requests: %w", err)
	}
	issues, err := connection[Issue](ctx, c, owner, name, "issues", "", "number title createdAt closedAt")
	if err != nil {
		return fmt.Errorf("issues: %w", err)
	}
	type releaseNode struct {
		TagName   string
		CreatedAt time.Time
		IsDraft   bool
	}
	rels, err := connection[releaseNode](ctx, c, owner, name, "releases", "", "tagName createdAt isDraft")
	if err != nil {
		return fmt.Errorf("releases: %w", err)
	}
	r.PRs, r.Issues, r.Releases = prs, issues, nil
	for _, rel := range rels {
		if !rel.IsDraft {
			r.Releases = append(r.Releases, Release{Tag: rel.TagName, CreatedAt: rel.CreatedAt})
		}
	}
	r.FetchedAt = time.Now()
	return nil
}

// Store is the on-disk cache of repo histories.
type Store struct {
	path  string
	Repos map[string]*Repo `json:"repos"`
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
// the returned error reports the first failure.
func Sync(ctx context.Context, c *Client, s *Store, metas []*Repo, ttl time.Duration, log func(string)) ([]*Repo, error) {
	var out []*Repo
	var firstErr error
	for _, m := range metas {
		cached := s.Repos[m.NameWithOwner]
		if cached != nil && time.Since(cached.FetchedAt) < ttl {
			m.Commits, m.PRs, m.Issues, m.Releases, m.FetchedAt = cached.Commits, cached.PRs, cached.Issues, cached.Releases, cached.FetchedAt
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
	return out, firstErr
}
