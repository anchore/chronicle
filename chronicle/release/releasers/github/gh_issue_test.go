package github

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/anchore/chronicle/chronicle/release/change"
)

func Test_issuesAtOrAfter(t *testing.T) {

	tests := []struct {
		name     string
		issue    ghIssue
		since    time.Time
		expected bool
	}{
		{
			name:  "issue is before compare date",
			since: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			issue: ghIssue{
				ClosedAt: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			expected: false,
		},
		{
			name:  "issue is equal to compare date",
			since: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			issue: ghIssue{
				ClosedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			expected: true,
		},
		{
			name:  "issue is after compare date",
			since: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			issue: ghIssue{
				ClosedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, issuesAtOrAfter(test.since)(test.issue))
		})
	}
}

func Test_issuesAfter(t *testing.T) {

	tests := []struct {
		name     string
		issue    ghIssue
		since    time.Time
		expected bool
	}{
		{
			name:  "issue is before compare date",
			since: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			issue: ghIssue{
				ClosedAt: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			expected: false,
		},
		{
			name:  "issue is equal to compare date",
			since: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			issue: ghIssue{
				ClosedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			expected: false,
		},
		{
			name:  "issue is after compare date",
			since: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			issue: ghIssue{
				ClosedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, issuesAfter(test.since)(test.issue))
		})
	}
}

func Test_issuesAtOrBefore(t *testing.T) {

	tests := []struct {
		name     string
		issue    ghIssue
		until    time.Time
		expected bool
	}{
		{
			name:  "issue is after compare date",
			until: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			issue: ghIssue{
				ClosedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			expected: false,
		},
		{
			name:  "issue is equal to compare date",
			until: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			issue: ghIssue{
				ClosedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			expected: true,
		},
		{
			name:  "issue is before compare date",
			until: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			issue: ghIssue{
				ClosedAt: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, issuesAtOrBefore(test.until)(test.issue))
		})
	}
}

func Test_issuesBefore(t *testing.T) {

	tests := []struct {
		name     string
		issue    ghIssue
		until    time.Time
		expected bool
	}{
		{
			name:  "issue is after compare date",
			until: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			issue: ghIssue{
				ClosedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			expected: false,
		},
		{
			name:  "issue is equal to compare date",
			until: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			issue: ghIssue{
				ClosedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			expected: false,
		},
		{
			name:  "issue is before compare date",
			until: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			issue: ghIssue{
				ClosedAt: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, issuesBefore(test.until)(test.issue))
		})
	}
}

func Test_issuesWithLabel(t *testing.T) {

	tests := []struct {
		name     string
		issue    ghIssue
		labels   []string
		expected bool
	}{
		{
			name: "matches on label",
			labels: []string{
				"positive",
			},
			issue: ghIssue{
				Labels: []string{"something-else", "positive"},
			},
			expected: true,
		},
		{
			name: "does not match on label",
			labels: []string{
				"positive",
			},
			issue: ghIssue{
				Labels: []string{"something-else", "negative"},
			},
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, issuesWithLabel(test.labels...)(test.issue))
		})
	}
}

func Test_issuesWithoutLabel(t *testing.T) {

	tests := []struct {
		name     string
		issue    ghIssue
		labels   []string
		expected bool
	}{
		{
			name: "matches on label",
			labels: []string{
				"positive",
			},
			issue: ghIssue{
				Labels: []string{"something-else", "positive"},
			},
			expected: false,
		},
		{
			name: "does not match on label",
			labels: []string{
				"positive",
			},
			issue: ghIssue{
				Labels: []string{"something-else", "negative"},
			},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, issuesWithoutLabel(test.labels...)(test.issue))
		})
	}
}

func Test_issuesWithChangeTypes(t *testing.T) {
	tests := []struct {
		name     string
		issue    ghIssue
		label    string
		expected bool
	}{
		{
			name:  "matches on label",
			label: "positive",
			issue: ghIssue{
				Labels: []string{"something-else", "positive"},
			},
			expected: true,
		},
		{
			name:  "does not match on label",
			label: "positive",
			issue: ghIssue{
				Labels: []string{"something-else", "negative"},
			},
			expected: false,
		},
		{
			name:  "does not have change types",
			label: "positive",
			issue: ghIssue{
				Labels: []string{},
			},
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, issuesWithChangeTypes(Config{
				ChangeTypesByLabel: change.TypeSet{
					test.label: change.NewType(test.label, change.SemVerMinor),
				},
			})(test.issue))
		})
	}
}
