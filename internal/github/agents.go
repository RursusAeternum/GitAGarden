package github

import "strings"

// gitActor is one of a commit's authors as GitHub reports them, including
// co-authors from Co-Authored-By trailers.
type gitActor struct {
	Name  string
	Email string
	User  *struct{ Login string }
}

// agents are the AI coding agents GAG recognises, by an author's email or
// GitHub login and never by name alone: a person called Claude stays a
// person. An email starting with "@" matches its whole domain. One line per
// agent, so adding one is easy.
var agents = []struct {
	name           string
	emails, logins []string
}{
	{"Claude", []string{"noreply@anthropic.com"}, []string{"claude", "claude[bot]"}},
	{"Codex", []string{"@openai.com"}, []string{"chatgpt-codex-connector[bot]"}},
	{"Copilot", nil, []string{"copilot", "copilot-swe-agent[bot]"}},
	{"Devin", nil, []string{"devin-ai-integration[bot]"}},
	{"Cursor", []string{"cursoragent@cursor.com"}, nil},
	{"Gemini", nil, []string{"gemini-code-assist[bot]"}},
	{"Jules", nil, []string{"google-labs-jules[bot]"}},
}

// agentOf names the AI coding agent among a commit's authors, or "" when
// people wrote it. aider marks its commits by adding "(aider)" to the
// author's name: the one match by name.
func agentOf(authors []gitActor) string {
	for _, a := range authors {
		email := strings.ToLower(strings.TrimSpace(a.Email))
		login := ""
		if a.User != nil {
			login = strings.ToLower(a.User.Login)
		}
		if login == "" {
			login = noreplyLogin(email)
		}
		for _, ag := range agents {
			for _, e := range ag.emails {
				if email == e || (strings.HasPrefix(e, "@") && strings.HasSuffix(email, e)) {
					return ag.name
				}
			}
			for _, l := range ag.logins {
				if login == l {
					return ag.name
				}
			}
		}
		if strings.HasSuffix(strings.TrimSpace(a.Name), "(aider)") {
			return "aider"
		}
	}
	return ""
}

// noreplyLogin is the login in a GitHub no-reply address,
// <id>+<login>@users.noreply.github.com, or "" for any other address.
func noreplyLogin(email string) string {
	local, ok := strings.CutSuffix(email, "@users.noreply.github.com")
	if !ok {
		return ""
	}
	if _, login, ok := strings.Cut(local, "+"); ok {
		return login
	}
	return local
}
