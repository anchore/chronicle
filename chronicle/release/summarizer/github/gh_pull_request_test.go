package github

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_prsAfter(t *testing.T) {

	tests := []struct {
		name     string
		pr       ghPullRequest
		since    time.Time
		expected bool
	}{
		{
			name:  "pr is before compare date",
			since: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			expected: false,
		},
		{
			name:  "pr is after compare date",
			since: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, prsAfter(test.since)(test.pr))
		})
	}
}

func Test_prsBefore(t *testing.T) {

	tests := []struct {
		name     string
		pr       ghPullRequest
		until    time.Time
		expected bool
	}{
		{
			name:  "pr is after compare date",
			until: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			},
			expected: false,
		},
		{
			name:  "pr is before compare date",
			until: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			pr: ghPullRequest{
				MergedAt: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, prsBefore(test.until)(test.pr))
		})
	}
}

func Test_prsWithLabel(t *testing.T) {

	tests := []struct {
		name     string
		pr       ghPullRequest
		labels   []string
		expected bool
	}{
		{
			name: "matches on label",
			labels: []string{
				"positive",
			},
			pr: ghPullRequest{
				Labels: []string{"something-else", "positive"},
			},
			expected: true,
		},
		{
			name: "does not match on label",
			labels: []string{
				"positive",
			},
			pr: ghPullRequest{
				Labels: []string{"something-else", "negative"},
			},
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, prsWithLabel(test.labels...)(test.pr))
		})
	}
}

func Test_prsWithoutLabel(t *testing.T) {

	tests := []struct {
		name     string
		pr       ghPullRequest
		labels   []string
		expected bool
	}{
		{
			name: "matches on label",
			labels: []string{
				"positive",
			},
			pr: ghPullRequest{
				Labels: []string{"something-else", "positive"},
			},
			expected: false,
		},
		{
			name: "does not match on label",
			labels: []string{
				"positive",
			},
			pr: ghPullRequest{
				Labels: []string{"something-else", "negative"},
			},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, prsWithoutLabel(test.labels...)(test.pr))
		})
	}
}

func Test_prsWithClosedLinkedIssue(t *testing.T) {

	tests := []struct {
		name     string
		pr       ghPullRequest
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
			expected: false,
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
			expected: true,
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
			assert.Equal(t, test.expected, prsWithClosedLinkedIssue()(test.pr))
		})
	}
}

func Test_prsWithOpenLinkedIssue(t *testing.T) {

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
			assert.Equal(t, test.expected, prsWithOpenLinkedIssue()(test.pr))
		})
	}
}
