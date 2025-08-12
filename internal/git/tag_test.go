package git

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
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
			path: "testdata/repos/tag-range-repo",
			expects: []string{
				"v0.1.0",
				"v0.1.1",
				"v0.2.0",
			},
		},
		{
			name: "annotated tags",
			path: "testdata/repos/annotated-tagged-repo",
			expects: []string{
				"v0.1.0",
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

func TestTagsFromLocal_processTag_timestamp(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		expects []Tag
	}{
		{
			name:    "lightweight tags case",
			path:    "testdata/repos/tag-range-repo",
			expects: expectedTags(t, "testdata/repos/tag-range-repo"),
		},
		{
			name:    "annotated tags",
			path:    "testdata/repos/annotated-tagged-repo",
			expects: expectedTags(t, "testdata/repos/annotated-tagged-repo"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := TagsFromLocal(test.path)
			require.NoError(t, err)
			if d := cmp.Diff(test.expects, actual); d != "" {
				t.Fatalf("unexpected tags (-want +got):\n%s", d)
			}
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
			path:     "testdata/repos/tag-range-repo",
			tag:      "v0.1.0",
			hasMatch: true,
		},
		{
			name:     "last tag exists",
			path:     "testdata/repos/tag-range-repo",
			tag:      "v0.2.0",
			hasMatch: true,
		},
		{
			name:     "fake tag",
			path:     "testdata/repos/tag-range-repo",
			tag:      "v1.84793.23849",
			hasMatch: false,
		},
		{
			name:     "annotated tag exists",
			path:     "testdata/repos/annotated-tagged-repo",
			tag:      "v0.1.0",
			hasMatch: true,
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
			path: "testdata/repos/tag-range-repo",
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
			path: "testdata/repos/tag-range-repo",
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
			path: "testdata/repos/tag-range-repo",
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
			path: "testdata/repos/tag-range-repo",
			config: Range{
				SinceRef:     "v0.1.0",
				UntilRef:     "v0.2.0",
				IncludeStart: false,
				IncludeEnd:   false,
			},
			count: 5,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := CommitsBetween(test.path, test.config)
			require.NoError(t, err)

			// the answer is based off the the current (dynamically created) git log test fixture
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

	// note: the -1 is to stop listing entries after the first entry
	cmd := exec.Command("git", "--no-pager", "log", `--pretty=format:%H`, "-1", tag)
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

func expectedTags(t *testing.T, path string) []Tag {
	t.Helper()

	cmd := exec.Command("git", "--no-pager", "for-each-ref", "refs/tags")
	cmd.Dir = path
	output, err := cmd.Output()
	require.NoError(t, err)

	rows := strings.Split(strings.TrimSpace(string(output)), "\n")

	var tags []Tag
	for _, row := range rows {
		// process rows like: "55b45584644cc820f0c0d64a64321d69b3def778 commit\trefs/tags/v0.1.0"
		fields := strings.Split(strings.ReplaceAll(row, "\t", " "), " ")
		if len(fields) != 3 {
			t.Fatalf("unexpected row: %q", row)
		}

		// type commit = lightweight tag... the tag commit is the ref to the blob
		// type tag = annotated tag... the tag commit has tag info
		tagCommit, ty, name := fields[0], fields[1], fields[2]
		nameFields := strings.Split(name, "/")
		date := dateForCommit(t, path, tagCommit)
		var annotated bool
		switch ty {
		case "tag":
			annotated = true
			date = dateForAnnotatedTag(t, path, name)
		case "commit":
			annotated = false
			date = dateForCommit(t, path, tagCommit)
		default:
			t.Fatalf("unexpected type: %q", ty)
		}

		tags = append(tags, Tag{
			Name:      nameFields[len(nameFields)-1],
			Timestamp: date,
			Commit:    tagHash(t, path, name),
			Annotated: annotated,
		})
	}

	return tags
}

func dateForCommit(t *testing.T, path string, commit string) time.Time {
	// note: %ci is the committer date in an ISO 8601-like format
	cmd := exec.Command("git", "--no-pager", "show", "-s", "--format=%ci", fmt.Sprintf("%s^{commit}", commit))
	cmd.Dir = path
	output, err := cmd.Output()
	require.NoError(t, err)

	rows := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(rows) != 1 {
		t.Fatalf("unable to get commit for commit=%s: %q", commit, output)
	}

	// output should be something like: "2023-09-18 15:15:40 -0400"
	tt, err := time.Parse("2006-01-02 15:04:05 -0700", rows[0])
	require.NoError(t, err)
	return tt
}

func dateForAnnotatedTag(t *testing.T, path string, tag string) time.Time {
	// for-each-ref is a nice way to get the raw information about a tag object ad not the information about the commit
	// the tag object points to (in this case we're interested in the tag object's timestamp).
	cmd := exec.Command("git", "--no-pager", "for-each-ref", `--format="%(creatordate)"`, tag)
	cmd.Dir = path
	output, err := cmd.Output()
	require.NoError(t, err)

	rows := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(rows) != 1 {
		t.Fatalf("unable to get commit for tag=%s: %q", tag, output)
	}

	// output should be something like: "Mon Sep 18 17:22:13 2023 -0400"
	tt, err := time.Parse(`"Mon Jan 2 15:04:05 2006 -0700"`, rows[0])
	require.NoError(t, err)
	return tt
}

func tagHash(t *testing.T, repo string, tag string) string {
	// note: this will work for both lightweight and annotated tags since we are dereferencing the tag to the closest
	// commit object with the ^{commit} syntax
	cmd := exec.Command("git", "--no-pager", "show", "-s", "--format=%H", fmt.Sprintf("%s^{commit}", tag))
	cmd.Dir = repo
	output, err := cmd.Output()
	require.NoError(t, err)

	rows := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(rows) != 1 {
		t.Fatalf("unable to get commit for tag=%s: %q", tag, output)
	}

	return rows[0]
}
