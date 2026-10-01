package github

import (
	"testing"
	"time"

	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"

	"github.com/anchore/chronicle/chronicle/release/change"
)

func Test_prsAtOrAfter(t *testing.T) {
	tests := []struct {
		name  string
		pr    ghPullRequest
		since time.Time
		keep  bool
	}{
		{
			name:  "pr is before compare date",
			since: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			keep: false,
		},
		{
			name:  "pr is equal to compare date",
			since: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			keep: true,
		},
		{
			name:  "pr is after compare date",
			since: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			keep: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.keep, prsAtOrAfter(test.since)(test.pr))
		})
	}
}

func Test_prsAfter(t *testing.T) {
	tests := []struct {
		name  string
		pr    ghPullRequest
		since time.Time
		keep  bool
	}{
		{
			name:  "pr is before compare date",
			since: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			keep: false,
		},
		{
			name:  "pr is equal to compare date",
			since: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			keep: false,
		},
		{
			name:  "pr is after compare date",
			since: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			keep: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.keep, prsAfter(test.since)(test.pr))
		})
	}
}

func Test_prsAtOrBefore(t *testing.T) {
	tests := []struct {
		name  string
		pr    ghPullRequest
		until time.Time
		keep  bool
	}{
		{
			name:  "pr is after compare date",
			until: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			keep: false,
		},
		{
			name:  "pr is equal to compare date",
			until: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			keep: true,
		},
		{
			name:  "pr is before compare date",
			until: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			keep: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.keep, prsAtOrBefore(test.until)(test.pr))
		})
	}
}

func Test_prsBefore(t *testing.T) {
	tests := []struct {
		name  string
		pr    ghPullRequest
		until time.Time
		keep  bool
	}{
		{
			name:  "pr is after compare date",
			until: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			keep: false,
		},
		{
			name:  "pr is equal to compare date",
			until: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			keep: false,
		},
		{
			name:  "pr is before compare date",
			until: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			keep: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.keep, prsBefore(test.until)(test.pr))
		})
	}
}

func Test_prsWithoutLabel(t *testing.T) {
	tests := []struct {
		name   string
		pr     ghPullRequest
		labels []string
		keep   bool
	}{
		{
			name: "matches on label",
			labels: []string{
				"positive",
			},
			pr: ghPullRequest{
				Labels: []string{"something-else", "positive"},
			},
			keep: false,
		},
		{
			name: "does not match on label",
			labels: []string{
				"positive",
			},
			pr: ghPullRequest{
				Labels: []string{"something-else", "negative"},
			},
			keep: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.keep, prsWithoutLabel(test.labels...)(test.pr))
		})
	}
}

func Test_prsWithoutAuthor(t *testing.T) {
	tests := []struct {
		name    string
		pr      ghPullRequest
		authors []string
		keep    bool
	}{
		{
			name:    "matches bot login with [bot] suffix",
			authors: []string{"dependabot"},
			pr:      ghPullRequest{Author: "dependabot[bot]"},
			keep:    false,
		},
		{
			name:    "matches bare login",
			authors: []string{"dependabot"},
			pr:      ghPullRequest{Author: "dependabot"},
			keep:    false,
		},
		{
			name:    "match is case-insensitive",
			authors: []string{"Dependabot"},
			pr:      ghPullRequest{Author: "dependabot[bot]"},
			keep:    false,
		},
		{
			name:    "matches any of several authors",
			authors: []string{"dependabot", "renovate"},
			pr:      ghPullRequest{Author: "renovate[bot]"},
			keep:    false,
		},
		{
			name:    "does not match a normal author",
			authors: []string{"dependabot", "renovate"},
			pr:      ghPullRequest{Author: "octocat"},
			keep:    true,
		},
		{
			name:    "empty author list keeps everything",
			authors: nil,
			pr:      ghPullRequest{Author: "dependabot[bot]"},
			keep:    true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.keep, prsWithoutAuthor(test.authors...)(test.pr))
		})
	}
}

func Test_prsWithoutClosedLinkedIssue(t *testing.T) {
	tests := []struct {
		name string
		pr   ghPullRequest
		keep bool
	}{
		{
			name: "has closed linked issue",
			pr: ghPullRequest{
				LinkedIssues: []ghIssue{
					{
						Closed: true,
					},
				},
			},
			keep: false,
		},
		{
			name: "open linked issue",
			pr: ghPullRequest{
				LinkedIssues: []ghIssue{
					{
						Closed: false,
					},
				},
			},
			keep: true,
		},
		{
			name: "no linked issue",
			pr: ghPullRequest{
				LinkedIssues: []ghIssue{},
			},
			keep: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.keep, prsWithoutClosedLinkedIssue()(test.pr))
		})
	}
}

func Test_prsWithoutOpenLinkedIssue(t *testing.T) {
	tests := []struct {
		name     string
		pr       ghPullRequest
		labels   []string
		expected bool
	}{
		{
			name: "has closed linked issue",
			pr: ghPullRequest{
				LinkedIssues: []ghIssue{
					{
						Closed: true,
					},
				},
			},
			expected: true,
		},
		{
			name: "open linked issue",
			pr: ghPullRequest{
				LinkedIssues: []ghIssue{
					{
						Closed: false,
					},
				},
			},
			expected: false,
		},
		{
			name: "no linked issue",
			pr: ghPullRequest{
				LinkedIssues: []ghIssue{},
			},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, prsWithoutOpenLinkedIssue()(test.pr))
		})
	}
}

func Test_prsWithoutMergeCommit(t *testing.T) {
	tests := []struct {
		name     string
		pr       ghPullRequest
		commits  []string
		expected bool
	}{
		{
			name: "has merge commit within range",
			pr: ghPullRequest{
				MergeCommit: "commit-1",
			},
			commits: []string{
				"commit-1",
				"commit-2",
				"commit-3",
			},
			expected: true,
		},
		{
			name: "has merge commit within range",
			pr: ghPullRequest{
				MergeCommit: "commit-bogosity",
			},
			commits: []string{
				"commit-1",
				"commit-2",
				"commit-3",
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, prsWithoutMergeCommit(tt.commits...)(tt.pr))
		})
	}
}

func Test_prsWithChangeTypes(t *testing.T) {
	tests := []struct {
		name     string
		pr       ghPullRequest
		label    string
		expected bool
	}{
		{
			name:  "matches on label",
			label: "positive",
			pr: ghPullRequest{
				Labels: []string{"something-else", "positive"},
			},
			expected: true,
		},
		{
			name:  "does not match on label",
			label: "positive",
			pr: ghPullRequest{
				Labels: []string{"something-else", "negative"},
			},
			expected: false,
		},
		{
			name:  "does not have change types",
			label: "positive",
			pr: ghPullRequest{
				Labels: []string{},
			},
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, prsWithChangeTypes(Config{
				ChangeTypesByLabel: change.TypeSet{
					test.label: change.NewType(test.label, change.SemVerMinor),
				},
			})(test.pr))
		})
	}
}

func Test_prsWithoutLabels(t *testing.T) {
	tests := []struct {
		name     string
		pr       ghPullRequest
		expected bool
	}{
		{
			name: "omitted when labels",
			pr: ghPullRequest{
				Labels: []string{"something-else", "positive"},
			},
			expected: false,
		},
		{
			name: "included with no labels",
			pr: ghPullRequest{
				Labels: []string{},
			},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, prsWithoutLabels()(test.pr))
		})
	}
}

func Test_prsWithoutLinkedIssues(t *testing.T) {
	tests := []struct {
		name     string
		pr       ghPullRequest
		expected bool
	}{
		{
			name:     "matches when unlinked",
			pr:       ghPullRequest{},
			expected: true,
		},
		{
			name: "does not match when linked",
			pr: ghPullRequest{
				LinkedIssues: []ghIssue{
					{
						Number: 1,
						Title:  "an issue",
					},
				},
			},
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, prsWithoutLinkedIssues()(test.pr))
		})
	}
}

func Test_checkSearchTermination(t *testing.T) {
	since := githubv4.DateTime{Time: time.Date(1987, time.September, 16, 19, 34, 0, 0, time.UTC)}
	hourAfter := &githubv4.DateTime{Time: since.Add(time.Hour)}
	minuteAfter := &githubv4.DateTime{Time: since.Add(time.Minute)}
	minuteBefore := &githubv4.DateTime{Time: since.Add(-time.Minute)}

	type args struct {
		since     *time.Time
		updatedAt *githubv4.DateTime
		mergedAt  *githubv4.DateTime
	}
	tests := []struct {
		name          string
		args          args
		wantProcess   bool
		wantTerminate bool
	}{
		{
			name: "go case candidate",
			args: args{
				since:     &since.Time,
				updatedAt: hourAfter,
				mergedAt:  hourAfter,
			},
			wantProcess:   true,
			wantTerminate: false,
		},
		{
			name: "candidate updated after the merge, and merged after the compare date",
			args: args{
				since:     &since.Time,
				updatedAt: hourAfter,
				mergedAt:  minuteAfter,
			},
			wantProcess:   true,
			wantTerminate: false,
		},
		{
			name: "candidate updated after the merge, but merged before the compare date",
			args: args{
				since:     &since.Time,
				updatedAt: hourAfter,
				mergedAt:  minuteBefore,
			},
			wantProcess:   false,
			wantTerminate: false,
		},
		{
			name: "candidate updated before the merge, and merged before the compare date",
			args: args{
				since:     &since.Time,
				updatedAt: minuteBefore,
				mergedAt:  minuteBefore,
			},
			wantProcess:   false,
			wantTerminate: true,
		},
		{
			name: "impossible: candidate updated before the merge, but merged after the compare date",
			args: args{
				since:     &since.Time,
				updatedAt: minuteBefore,
				mergedAt:  minuteAfter,
			},
			wantProcess:   true,
			wantTerminate: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProcess, gotTerminate := checkSearchTermination(tt.args.since, tt.args.updatedAt, tt.args.mergedAt)
			assert.Equalf(t, tt.wantProcess, gotProcess, "wantProcess: checkSearchTermination(%v, %v, %v)", tt.args.since, tt.args.updatedAt, tt.args.mergedAt)
			assert.Equalf(t, tt.wantTerminate, gotTerminate, "wantTerminate: checkSearchTermination(%v, %v, %v)", tt.args.since, tt.args.updatedAt, tt.args.mergedAt)
		})
	}
}

func Test_prFromNode(t *testing.T) {
	labels := func(names ...string) labelConnection {
		var l labelConnection
		for _, n := range names {
			l.Edges = append(l.Edges, struct {
				Node struct{ Name githubv4.String }
			}{Node: struct{ Name githubv4.String }{Name: githubv4.String(n)}})
		}
		return l
	}

	merged := time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC)
	closed := merged.Add(time.Minute)

	var n prNode
	n.Title = "fix the thing"
	n.Number = 10
	n.URL = "https://github.com/owner/repo/pull/10"
	n.Author.Login = "someone"
	n.MergeCommit.OID = "abc123"
	n.MergedAt = githubv4.DateTime{Time: merged}
	n.Labels = labels("dependencies")
	n.ClosingIssuesReferences.TotalCount = 3

	const repoID = githubv4.Int(42)

	var bug, unlabeled, otherRepo closingIssueNode
	bug.Title = "it is broken"
	bug.Number = 7
	bug.URL = "https://github.com/owner/repo/issues/7"
	bug.Author.Login = "reporter"
	bug.Closed = true
	bug.ClosedAt = githubv4.DateTime{Time: closed}
	bug.Labels = labels("bug", "priority")

	bug.Repository.DatabaseID = repoID

	unlabeled.Number = 8
	unlabeled.Closed = true
	unlabeled.Repository.DatabaseID = repoID

	// issues in other repos never show up in this repo's changelog, so they are not linked
	otherRepo.Number = 9
	otherRepo.Closed = true
	otherRepo.Labels = labels("bug")
	otherRepo.Repository.DatabaseID = 7

	n.ClosingIssuesReferences.Nodes = []closingIssueNode{bug, unlabeled, otherRepo}

	// each linked issue must carry its own labels, never the PR's
	assert.Equal(t, ghPullRequest{
		Title:       "fix the thing",
		Number:      10,
		Author:      "someone",
		MergedAt:    merged,
		Labels:      []string{"dependencies"},
		URL:         "https://github.com/owner/repo/pull/10",
		MergeCommit: "abc123",
		LinkedIssues: []ghIssue{
			{
				Title:    "it is broken",
				Number:   7,
				Author:   "reporter",
				ClosedAt: closed,
				Closed:   true,
				Labels:   []string{"bug", "priority"},
				URL:      "https://github.com/owner/repo/issues/7",
			},
			{
				Number: 8,
				Closed: true,
			},
		},
	}, prFromNode(n, repoID))
}
