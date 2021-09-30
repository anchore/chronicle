package github

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_issuesAfter(t *testing.T) {

	tests := []struct {
		name     string
		issue    ghIssue
		since    time.Time
		expected bool
	}{
		{
			name:  "ghIssue is before compare date date",
			since: time.Date(2021, time.September, 18, 19, 34, 0, 0, time.UTC),
			issue: ghIssue{
				ClosedAt: time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			expected: false,
		},
		{
			name:  "ghIssue is after compare date date",
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
