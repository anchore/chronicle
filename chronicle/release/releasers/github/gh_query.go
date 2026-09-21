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

// repoQueryVariables returns the variable bindings shared by every repository-scoped
// query. Callers add their own pagination cursor to the returned map.
func repoQueryVariables(user, repo string) map[string]interface{} {
	return map[string]interface{}{
		varRepositoryOwner: githubv4.String(user),
		varRepositoryName:  githubv4.String(repo),
	}
}
