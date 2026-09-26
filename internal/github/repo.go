package github

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
)

type Commit struct {
	At      time.Time `json:"at"`
	Message string    `json:"msg"`
}

type PR struct {
	Number   int       `json:"number"`
	Title    string    `json:"title"`
	MergedAt time.Time `json:"mergedAt"`
}

type Issue struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	CreatedAt time.Time  `json:"createdAt"`
	ClosedAt  *time.Time `json:"closedAt,omitempty"`
}

type Release struct {
	Tag       string    `json:"tag"`
	CreatedAt time.Time `json:"createdAt"`
}

// Repo is everything the garden needs to know about one repository.
type Repo struct {
	NameWithOwner string    `json:"nameWithOwner"`
	Language      string    `json:"language"`
	Archived      bool      `json:"archived"`
	Topics        []string  `json:"topics"`
	PushedAt      time.Time `json:"pushedAt"`
	Commits       []Commit  `json:"commits"`
	PRs           []PR      `json:"prs"`
	Issues        []Issue   `json:"issues"`
	Releases      []Release `json:"releases"`
	FetchedAt     time.Time `json:"fetchedAt"`
}

func (r *Repo) Name() string {
	_, name, _ := strings.Cut(r.NameWithOwner, "/")
	return name
}

// Finished projects are archived, or tagged with a "finished" topic.
func (r *Repo) Finished() bool {
	if r.Archived {
		return true
	}
	for _, t := range r.Topics {
		if t == "finished" || t == "gag-finished" {
			return true
		}
	}
	return false
}

func (r *Repo) Species() garden.Species { return garden.SpeciesFor(r.Language) }

// Events flattens the history into the time-ordered events plants grow from.
// Commits on the default branch stand in for pushes; GitHub only keeps real
// push events for 90 days.
func (r *Repo) Events() []garden.Event {
	var ev []garden.Event
	for _, c := range r.Commits {
		ev = append(ev, garden.Event{Kind: garden.Push, At: c.At, Note: c.Message})
	}
	for _, p := range r.PRs {
		ev = append(ev, garden.Event{Kind: garden.Merge, At: p.MergedAt, Note: fmt.Sprintf("merged #%d %s", p.Number, p.Title)})
	}
	for _, rel := range r.Releases {
		ev = append(ev, garden.Event{Kind: garden.Release, At: rel.CreatedAt, Note: "released " + rel.Tag})
	}
	for _, i := range r.Issues {
		ev = append(ev, garden.Event{Kind: garden.IssueOpened, At: i.CreatedAt, Note: fmt.Sprintf("opened #%d %s", i.Number, i.Title)})
		if i.ClosedAt != nil {
			ev = append(ev, garden.Event{Kind: garden.IssueClosed, At: *i.ClosedAt, Note: fmt.Sprintf("closed #%d %s", i.Number, i.Title)})
		}
	}
	sort.SliceStable(ev, func(a, b int) bool { return ev[a].At.Before(ev[b].At) })
	return ev
}

const metaFields = `nameWithOwner isArchived pushedAt primaryLanguage{name} repositoryTopics(first:20){nodes{topic{name}}}`

type metaNode struct {
	NameWithOwner    string
	IsArchived       bool
	PushedAt         time.Time
	PrimaryLanguage  *struct{ Name string }
	RepositoryTopics struct {
		Nodes []struct{ Topic struct{ Name string } }
	}
}

func (m metaNode) repo() *Repo {
	r := &Repo{NameWithOwner: m.NameWithOwner, Archived: m.IsArchived, PushedAt: m.PushedAt}
	if m.PrimaryLanguage != nil {
		r.Language = m.PrimaryLanguage.Name
	}
	for _, n := range m.RepositoryTopics.Nodes {
		r.Topics = append(r.Topics, n.Topic.Name)
	}
	return r
}

// ListRepos returns the viewer's own (non-fork) repos, most recently pushed
// first. Only metadata is filled in; Sync fetches the histories.
func (c *Client) ListRepos(ctx context.Context, limit int) ([]*Repo, error) {
	q := `query($n:Int!){viewer{repositories(first:$n,ownerAffiliations:[OWNER],isFork:false,orderBy:{field:PUSHED_AT,direction:DESC}){nodes{` + metaFields + `}}}}`
	var out struct {
		Viewer struct {
			Repositories struct{ Nodes []metaNode }
		}
	}
	if err := c.query(ctx, q, map[string]any{"n": min(limit, 100)}, &out); err != nil {
		return nil, err
	}
	var repos []*Repo
	for _, n := range out.Viewer.Repositories.Nodes {
		repos = append(repos, n.repo())
	}
	return repos, nil
}

// ListOwnerRepos returns another user's or organization's public, non-fork
// repos, most recently pushed first.
func (c *Client) ListOwnerRepos(ctx context.Context, login string, limit int) ([]*Repo, error) {
	q := `query($login:String!,$n:Int!){repositoryOwner(login:$login){repositories(first:$n,isFork:false,privacy:PUBLIC,orderBy:{field:PUSHED_AT,direction:DESC}){nodes{` + metaFields + `}}}}`
	var out struct {
		RepositoryOwner *struct {
			Repositories struct{ Nodes []metaNode }
		}
	}
	if err := c.query(ctx, q, map[string]any{"login": login, "n": min(limit, 100)}, &out); err != nil {
		return nil, err
	}
	if out.RepositoryOwner == nil {
		return nil, fmt.Errorf("no GitHub user or organization named %q", login)
	}
	var repos []*Repo
	for _, n := range out.RepositoryOwner.Repositories.Nodes {
		repos = append(repos, n.repo())
	}
	return repos, nil
}

// LookupRepo fetches metadata for one "owner/name" repo.
func (c *Client) LookupRepo(ctx context.Context, nameWithOwner string) (*Repo, error) {
	owner, name, ok := strings.Cut(nameWithOwner, "/")
	if !ok {
		return nil, fmt.Errorf("repo %q: want owner/name", nameWithOwner)
	}
	q := `query($owner:String!,$name:String!){repository(owner:$owner,name:$name){` + metaFields + `}}`
	var out struct{ Repository *metaNode }
	if err := c.query(ctx, q, map[string]any{"owner": owner, "name": name}, &out); err != nil {
		return nil, err
	}
	if out.Repository == nil {
		return nil, fmt.Errorf("repo %q not found", nameWithOwner)
	}
	return out.Repository.repo(), nil
}
