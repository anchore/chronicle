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

func Test_prsWithLabel(t *testing.T) {

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
			keep: true,
		},
		{
			name: "does not match on label",
			labels: []string{
				"positive",
			},
			pr: ghPullRequest{
				Labels: []string{"something-else", "negative"},
			},
			keep: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.keep, prsWithLabel(test.labels...)(test.pr))
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
