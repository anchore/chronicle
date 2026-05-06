package github

import (
	"fmt"
	"strings"
)

// explainGithubAPIError adds an actionable hint for common GitHub API failure modes.
// The shurcooL/githubv4 client returns errors like:
//
//	non-200 OK status code: 401 Unauthorized body: "..."
//	non-200 OK status code: 403 Forbidden body: "..."
//	non-200 OK status code: 404 Not Found body: "..."
//
// here we sniff the message text (the client doesn't expose a structured status code) and prepend a hint.
func explainGithubAPIError(operation string, user, repo string, err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "401"):
		return fmt.Errorf("%s: GitHub authentication failed (HTTP 401). Set GITHUB_TOKEN to a token with 'repo' scope (or 'public_repo' for public repositories): %w", operation, err)
	case strings.Contains(msg, "403"):
		return fmt.Errorf("%s: GitHub authorization failed (HTTP 403). The token may lack required scopes, or you've hit the API rate limit: %w", operation, err)
	case strings.Contains(msg, "404"):
		return fmt.Errorf("%s: GitHub repository %q not found (HTTP 404). Check spelling and that the token can access it: %w", operation, user+"/"+repo, err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
