package git

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagsFromLocal(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		expects []string
	}{
		{
			name: "go case",
			path: "test-fixtures/repos/tag-range-repo",
			expects: []string{
				"v0.1.0",
				"v0.1.1",
				"v0.2.0",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := TagsFromLocal(test.path)
			var names []string
			for _, a := range actual {
				names = append(names, a.Name)
			}
			require.NoError(t, err)
			assert.Equal(t, test.expects, names)
		})
	}
}

func TestSearchForTag(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		tag      string
		hasMatch bool
	}{
		{
			name:     "first tag exists",
			path:     "test-fixtures/repos/tag-range-repo",
			tag:      "v0.1.0",
			hasMatch: true,
		},
		{
			name:     "last tag exists",
			path:     "test-fixtures/repos/tag-range-repo",
			tag:      "v0.2.0",
			hasMatch: true,
		},
		{
			name:     "fake tag",
			path:     "test-fixtures/repos/tag-range-repo",
			tag:      "v1.84793.23849",
			hasMatch: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := SearchForTag(test.path, test.tag)

			if test.hasMatch {
				require.NoError(t, err)
				expectedCommit := gitTagCommit(t, test.path, test.tag)
				require.Equal(t, expectedCommit, actual.Commit)
				require.Equal(t, test.tag, actual.Name)
			} else {
				require.Nil(t, actual)
				require.Error(t, err)
			}
		})
	}
}

func TestCommitsBetween(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		config Range
		count  int
	}{
		{
			name: "all inclusive",
			path: "test-fixtures/repos/tag-range-repo",
			config: Range{
				SinceRef:     "v0.1.0",
				UntilRef:     "v0.2.0",
				IncludeStart: true,
				IncludeEnd:   true,
			},
			count: 7,
		},
		{
			name: "exclude start",
			path: "test-fixtures/repos/tag-range-repo",
			config: Range{
				SinceRef:     "v0.1.0",
				UntilRef:     "v0.2.0",
				IncludeStart: false,
				IncludeEnd:   true,
			},
			count: 6,
		},
		{
			name: "exclude end",
			path: "test-fixtures/repos/tag-range-repo",
			config: Range{
				SinceRef:     "v0.1.0",
				UntilRef:     "v0.2.0",
				IncludeStart: true,
				IncludeEnd:   false,
			},
			count: 6,
		},
		{
			name: "exclude start and end",
			path: "test-fixtures/repos/tag-range-repo",
			config: Range{
				SinceRef:     "v0.1.0",
				UntilRef:     "v0.2.0",
				IncludeStart: false,
				IncludeEnd:   false,
			},
			count: 5,
		},
		{
			name: "include start and end; filter by time",
			path: "test-fixtures/repos/tag-range-repo",
			config: Range{
				SinceRef:     "v0.1.1",
				UntilRef:     "v0.2.0",
				IncludeStart: true,
				IncludeEnd:   true,
			},
			count: 4,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := CommitsBetween(test.path, test.config)
			require.NoError(t, err)

			// the answer is based off the current (dynamically created) git log test fixture
			expected := gitLogRange(t, test.path, test.config.SinceRef, test.config.UntilRef)
			require.NotEmpty(t, expected)

			if !test.config.IncludeStart {
				// remember: git log is in reverse chronological order
				expected = popBack(expected)
			}

			if !test.config.IncludeEnd {
				// remember: git log is in reverse chronological order
				expected = popFront(expected)
			}

			require.Len(t, expected, test.count, "BAD job building expected commits: expected %d, got %d", test.count, len(expected))

			assert.Equal(t, expected, actual)

			// make certain that the commit values match the extracted tag commit values
			if test.config.IncludeEnd {
				// remember: git log is in reverse chronological order
				assert.Equal(t, gitTagCommit(t, test.path, test.config.UntilRef), actual[0])
			}

			// make certain that the commit values match the extracted tag commit values
			if test.config.IncludeStart {
				// remember: git log is in reverse chronological order
				assert.Equal(t, gitTagCommit(t, test.path, test.config.SinceRef), actual[len(actual)-1])
			}
		})
	}
}

func gitLogRange(t *testing.T, path, since, until string) []string {
	t.Helper()

	since = strings.TrimSpace(since)
	if since == "" {
		t.Fatal("require 'since'")
	}

	// why the ~1? we want git log to return inclusive results
	cmd := exec.Command("git", "--no-pager", "log", `--pretty=format:%H`, fmt.Sprintf("%s~1..%s", since, until))
	cmd.Dir = path
	output, err := cmd.Output()
	require.NoError(t, err)

	rows := strings.Split(strings.TrimSpace(string(output)), "\n")
	return rows
}

func gitTagCommit(t *testing.T, path, tag string) string {
	t.Helper()

	tag = strings.TrimSpace(tag)
	if tag == "" {
		t.Fatal("require 'tag'")
	}

	// why the ~1? we want git log to return inclusive results
	cmd := exec.Command("git", "--no-pager", "log", `--pretty=format:%H`, fmt.Sprintf("%s~1..%s", tag, tag))
	cmd.Dir = path
	output, err := cmd.Output()
	require.NoError(t, err)

	rows := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(rows) != 1 {
		t.Fatalf("unable to get commit for tag=%s: %q", tag, output)
	}
	return rows[0]
}

func popFront(items []string) []string {
	if len(items) == 0 {
		return items
	}
	return items[1:]
}

func popBack(items []string) []string {
	if len(items) == 0 {
		return items
	}
	return items[:len(items)-1]
}
