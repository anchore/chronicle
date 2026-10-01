package github

import (
	"github.com/shurcooL/githubv4"
)

// every GraphQL query in this package scopes to the same
// `repository(owner:$repositoryOwner, name:$repositoryName)` selector, so the two
// variable names are declared once here and bound by repoQueryVariables.
const (
	varRepositoryOwner = "repositoryOwner"
	varRepositoryName  = "repositoryName"
)

// labelConnection is the `labels(...)` connection shape shared by every PR and issue query.
type labelConnection struct {
	Edges []struct {
		Node struct {
			Name githubv4.String
		}
	}
}

func (l labelConnection) names() []string {
	var names []string
	for _, e := range l.Edges {
		names = append(names, string(e.Node.Name))
	}
	return names
}

// repoQueryVariables returns the variable bindings shared by every repository-scoped
// query. Callers add their own pagination cursor to the returned map.
func repoQueryVariables(user, repo string) map[string]interface{} {
	return map[string]interface{}{
		varRepositoryOwner: githubv4.String(user),
		varRepositoryName:  githubv4.String(repo),
	}
}

// assigneeConnection is the `assignees(...)` connection shape shared by every issue query. Github caps
// issues at 10 assignees, so `assignees(first:10)` always holds all of them.
type assigneeConnection struct {
	Nodes []struct {
		Login githubv4.String
	}
}

func (a assigneeConnection) logins() []string {
	var logins []string
	for _, n := range a.Nodes {
		if n.Login != "" {
			logins = append(logins, string(n.Login))
		}
	}
	return logins
}
