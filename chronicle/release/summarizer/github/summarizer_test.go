package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_extractGithubUserAndRepo(t *testing.T) {

	tests := []struct {
		url  string
		user string
		repo string
	}{
		{
			url:  "git@github.com:someone/project.git",
			user: "someone",
			repo: "project",
		},
		{
			url:  "https://github.com/someone/project.git",
			user: "someone",
			repo: "project",
		},
		{
			url:  "http://github.com/someone/project.git",
			user: "someone",
			repo: "project",
		},
	}
	for _, test := range tests {
		t.Run(test.url, func(t *testing.T) {
			user, repo := extractGithubUserAndRepo(test.url)
			assert.Equal(t, test.user, user, "bad user")
			assert.Equal(t, test.repo, repo, "bad repo")
		})
	}
}
