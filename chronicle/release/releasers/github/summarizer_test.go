package github

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal/git"
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

func Test_issueFilters(t *testing.T) {
	patch := change.NewType("patch", change.SemVerPatch)
	feature := change.NewType("added-feature", change.SemVerMinor)
	breaking := change.NewType("breaking-change", change.SemVerMajor)

	changeTypeSet := change.TypeSet{
		"bug":             patch,
		"fix":             patch,
		"feature":         feature,
		"breaking":        breaking,
		"removed":         breaking,
		"breaking-change": breaking,
	}

	timeStart := time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC)
	timeAfter := timeStart.Add(2 * time.Hour)
	timeBefore := timeStart.Add(-2 * time.Hour)
	timeEnd := timeStart.Add(5 * time.Hour)
	timeAfterEnd := timeEnd.Add(3 * time.Hour)

	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: timeStart,
	}

	untilTag := &git.Tag{
		Name:      "v0.2.0",
		Timestamp: timeEnd,
	}

	bugAfterLastRelease := ghIssue{
		Title:    "bug after last release",
		Number:   1,
		ClosedAt: timeAfter,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	bugAtLastRelease := ghIssue{
		Title:    "bug at (included within) last release",
		Number:   6,
		ClosedAt: timeStart,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	bugAtEndTag := ghIssue{
		Title:    "bug at (included within) end tag",
		Number:   7,
		ClosedAt: timeEnd,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	issueWithoutLabelAfterLastRelease := ghIssue{
		Title:    "issue without label after last release",
		Number:   2,
		ClosedAt: timeAfter,
		Closed:   true,
		Labels:   []string{},
	}

	bugBeforeLastRelease := ghIssue{
		Title:    "bug before last release",
		Number:   3,
		ClosedAt: timeBefore,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	bugAfterEndTag := ghIssue{
		Title:    "bug after end tag",
		Number:   4,
		ClosedAt: timeAfterEnd,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	featureAfterLastRelease := ghIssue{
		Title:    "feature after last release",
		Number:   5,
		ClosedAt: timeAfter,
		Closed:   true,
		Labels:   []string{"feature"},
	}

	input := []ghIssue{
		// keep
		bugAfterLastRelease,
		issueWithoutLabelAfterLastRelease,
		featureAfterLastRelease,
		bugAtEndTag, // edge case
		// not keep
		bugBeforeLastRelease,
		bugAfterEndTag,
		bugAtLastRelease, // edge case
	}
	tests := []struct {
		name           string
		since          *git.Tag
		until          *git.Tag
		config         Config
		inputIssues    []ghIssue
		expectedIssues []ghIssue
	}{
		{
			name:  "keep changes between the tags",
			since: sinceTag,
			until: untilTag,
			config: Config{
				ExcludeLabels:      nil,
				ChangeTypesByLabel: changeTypeSet,
			},
			inputIssues: input,
			expectedIssues: []ghIssue{
				bugAfterLastRelease,
				featureAfterLastRelease,
				bugAtEndTag,
			},
		},
		{
			name:  "keep changes after start tag",
			since: sinceTag,
			config: Config{
				ExcludeLabels:      nil,
				ChangeTypesByLabel: changeTypeSet,
			},
			inputIssues: input,
			expectedIssues: []ghIssue{
				bugAfterLastRelease,
				bugAfterEndTag,
				featureAfterLastRelease,
				bugAtEndTag,
			},
		},
		{
			name:  "keep changes after start tag (that are not bugs)",
			since: sinceTag,
			config: Config{
				ExcludeLabels:      []string{"bug"},
				ChangeTypesByLabel: changeTypeSet,
			},
			inputIssues: input,
			expectedIssues: []ghIssue{
				featureAfterLastRelease,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.expectedIssues, filterIssues(tt.inputIssues, standardIssueFilters(tt.config, tt.since, tt.until)...))
		})
	}
}

func Test_prFilters(t *testing.T) {
	patch := change.NewType("patch", change.SemVerPatch)
	feature := change.NewType("added-feature", change.SemVerMinor)
	breaking := change.NewType("breaking-change", change.SemVerMajor)

	changeTypeSet := change.TypeSet{
		"bug":             patch,
		"fix":             patch,
		"feature":         feature,
		"breaking":        breaking,
		"removed":         breaking,
		"breaking-change": breaking,
	}

	timeStart := time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC)
	timeAfter := timeStart.Add(2 * time.Hour)
	timeBefore := timeStart.Add(-2 * time.Hour)
	timeEnd := timeStart.Add(5 * time.Hour)
	timeAfterEnd := timeEnd.Add(3 * time.Hour)

	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: timeStart,
	}

	untilTag := &git.Tag{
		Name:      "v0.2.0",
		Timestamp: timeEnd,
	}

	prBugAfterLastRelease := ghPullRequest{
		Title:    "pr bug after starting tag",
		Number:   1,
		MergedAt: timeAfter,
		Labels:   []string{"bug"},
	}

	prBugAtLastRelease := ghPullRequest{
		Title:    "pr bug at (within) last release",
		Number:   10,
		MergedAt: timeStart,
		Labels:   []string{"bug"},
	}

	prBugAtEndTag := ghPullRequest{
		Title:    "pr bug at (within) end tag",
		Number:   11,
		MergedAt: timeEnd,
		Labels:   []string{"bug"},
	}

	prAfterLastRelease := ghPullRequest{
		Title:    "pr after starting tag",
		Number:   9,
		MergedAt: timeAfter,
	}

	prBugBeforeLastRelease := ghPullRequest{
		Title:    "pr bug before starting tag",
		Number:   2,
		MergedAt: timeBefore,
		Labels:   []string{"bug"},
	}

	issueClosedAfterLastRelease := ghIssue{
		Title:    "issue bug (closed after last release)",
		Number:   4,
		ClosedAt: timeAfter,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	issueOpen := ghIssue{
		Title:  "issue bug (open)",
		Number: 5,
		Closed: false,
		Labels: []string{"bug"},
	}

	prBugAfterLastReleaseWithOpenLinkedIssue := ghPullRequest{
		Title:        "pr bug after starting tag (w/ open linked issue)",
		Number:       3,
		MergedAt:     timeAfter,
		Labels:       []string{"bug"},
		LinkedIssues: []ghIssue{issueOpen},
	}

	prAfterLastReleaseWithOpenLinkedIssue := ghPullRequest{
		Title:        "pr after starting tag (w/ open linked issue)",
		Number:       6,
		MergedAt:     timeAfter,
		LinkedIssues: []ghIssue{issueOpen},
	}

	prBugAfterLastReleaseWithClosedLinkedIssue := ghPullRequest{
		Title:        "pr bug after starting tag (w/ closed linked issue)",
		Number:       7,
		MergedAt:     timeAfter,
		Labels:       []string{"bug"},
		LinkedIssues: []ghIssue{issueClosedAfterLastRelease},
	}

	prAfterLastReleaseWithClosedLinkedIssue := ghPullRequest{
		Title:        "pr after starting tag (w/ closed linked issue)",
		Number:       8,
		MergedAt:     timeAfter,
		LinkedIssues: []ghIssue{issueClosedAfterLastRelease},
	}

	prFeatureAfterEndTag := ghPullRequest{
		Title:    "pr feature after end tag",
		Number:   1,
		MergedAt: timeAfterEnd,
		Labels:   []string{"feature"},
	}

	input := []ghPullRequest{
		// keep
		prBugAfterLastRelease,
		prBugAtEndTag, // edge case

		// filter out
		prAfterLastRelease,
		prBugAtLastRelease, // edge case
		prBugBeforeLastRelease,
		prBugAfterLastReleaseWithOpenLinkedIssue,
		prAfterLastReleaseWithOpenLinkedIssue,
		prBugAfterLastReleaseWithClosedLinkedIssue,
		prAfterLastReleaseWithClosedLinkedIssue,
		prFeatureAfterEndTag,
	}

	tests := []struct {
		name        string
		since       *git.Tag
		until       *git.Tag
		config      Config
		inputPrs    []ghPullRequest
		expectedPrs []ghPullRequest
	}{
		{
			name:  "keep changes between the tags",
			since: sinceTag,
			until: untilTag,
			config: Config{
				ExcludeLabels:      nil,
				ChangeTypesByLabel: changeTypeSet,
			},
			inputPrs: input,
			expectedPrs: []ghPullRequest{
				prBugAfterLastRelease,
				prBugAtEndTag,
			},
		},
		{
			name:  "keep changes after start tag",
			since: sinceTag,
			config: Config{
				ExcludeLabels:      nil,
				ChangeTypesByLabel: changeTypeSet,
			},
			inputPrs: input,
			expectedPrs: []ghPullRequest{
				prBugAfterLastRelease,
				prBugAtEndTag,
				prFeatureAfterEndTag,
			},
		},
		{
			name:  "keep only added features after start tag",
			since: sinceTag,
			config: Config{
				ExcludeLabels:      []string{"bug"},
				ChangeTypesByLabel: changeTypeSet,
			},
			inputPrs: input,
			expectedPrs: []ghPullRequest{
				prFeatureAfterEndTag,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.expectedPrs, filterPRs(tt.inputPrs, standardPrFilters(tt.config, tt.since, tt.until)...))
		})
	}
}

func Test_changesFromIssuesExtractedFromPRs(t *testing.T) {
	//log.Log = logger.NewLogrusLogger(logger.LogrusConfig{
	//	EnableConsole: true,
	//	EnableFile:    false,
	//	Structured:    false,
	//	Level:         logrus.TraceLevel,
	//})
	patch := change.NewType("patch", change.SemVerPatch)
	feature := change.NewType("added-feature", change.SemVerMinor)
	breaking := change.NewType("breaking-change", change.SemVerMajor)

	changeTypeSet := change.TypeSet{
		"bug":             patch,
		"fix":             patch,
		"feature":         feature,
		"breaking":        breaking,
		"removed":         breaking,
		"breaking-change": breaking,
	}

	timeStart := time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC)
	timeAfter := timeStart.Add(2 * time.Hour)
	timeBefore := timeStart.Add(-2 * time.Hour)
	timeEnd := timeStart.Add(5 * time.Hour)
	timeAfterEnd := timeEnd.Add(3 * time.Hour)

	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: timeStart,
	}

	untilTag := &git.Tag{
		Name:      "v0.2.0",
		Timestamp: timeEnd,
	}

	prBugAfterLastRelease := ghPullRequest{
		Title:    "pr bug after starting tag",
		Number:   1,
		MergedAt: timeAfter,
		Labels:   []string{"bug"},
	}

	prBugAtLastRelease := ghPullRequest{
		Title:    "pr bug at (within) last release",
		Number:   10,
		MergedAt: timeStart,
		Labels:   []string{"bug"},
	}

	prBugAtEndTag := ghPullRequest{
		Title:    "pr bug at (within) end tag",
		Number:   11,
		MergedAt: timeEnd,
		Labels:   []string{"bug"},
	}

	prAfterLastRelease := ghPullRequest{
		Title:    "pr after starting tag",
		Number:   9,
		MergedAt: timeAfter,
	}

	prBugBeforeLastRelease := ghPullRequest{
		Title:    "pr bug before starting tag",
		Number:   2,
		MergedAt: timeBefore,
		Labels:   []string{"bug"},
	}

	issueClosedAfterLastRelease := ghIssue{
		Title:    "issue bug (closed after last release)",
		Number:   4,
		ClosedAt: timeAfter,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	issueClosedAfterLastRelease2 := ghIssue{
		Title:    "issue bug (closed after last release) -- 2",
		Number:   13,
		ClosedAt: timeAfter,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	issueClosedAfterLastRelease3 := ghIssue{
		Title:    "issue feature (closed after last release) -- 3",
		Number:   15,
		ClosedAt: timeAfterEnd,
		Closed:   true,
		Labels:   []string{"feature"},
	}

	issueOpen := ghIssue{
		Title:  "issue bug (open)",
		Number: 5,
		Closed: false,
		Labels: []string{"bug"},
	}

	prBugAfterLastReleaseWithOpenLinkedIssue := ghPullRequest{
		Title:        "pr bug after starting tag (w/ open linked issue)",
		Number:       3,
		MergedAt:     timeAfter,
		Labels:       []string{"bug"},
		LinkedIssues: []ghIssue{issueOpen},
	}

	prAfterLastReleaseWithOpenLinkedIssue := ghPullRequest{
		Title:        "pr after starting tag (w/ open linked issue)",
		Number:       6,
		MergedAt:     timeAfter,
		LinkedIssues: []ghIssue{issueOpen},
	}

	prBugAfterLastReleaseWithClosedLinkedIssue := ghPullRequest{
		Title:        "pr bug after starting tag (w/ closed linked issue)",
		Number:       7,
		MergedAt:     timeAfter,
		Labels:       []string{"bug"},
		LinkedIssues: []ghIssue{issueClosedAfterLastRelease},
	}

	prAfterLastReleaseWithClosedLinkedIssue := ghPullRequest{
		Title:        "pr after starting tag (w/ closed linked issue)",
		Number:       8,
		MergedAt:     timeAfter,
		LinkedIssues: []ghIssue{issueClosedAfterLastRelease2},
	}

	prFeatureAfterEndTag := ghPullRequest{
		Title:    "pr feature after end tag",
		Number:   12,
		MergedAt: timeAfterEnd,
		Labels:   []string{"feature"},
	}

	prAfterEndTagWithClosedLinkedIssue := ghPullRequest{
		Title:        "pr after end tag (w/ closed linked issue)",
		Number:       14,
		MergedAt:     timeAfterEnd,
		Labels:       nil,
		LinkedIssues: []ghIssue{issueClosedAfterLastRelease3},
	}

	input := []ghPullRequest{
		// keep
		prAfterLastReleaseWithClosedLinkedIssue, // = issue "issueClosedAfterLastRelease2"
		prAfterEndTagWithClosedLinkedIssue,      // = issue "issueClosedAfterLastRelease3"
		// filter out
		prAfterLastRelease,
		prBugAtLastRelease, // edge case
		prBugBeforeLastRelease,
		prBugAfterLastReleaseWithOpenLinkedIssue,
		prAfterLastReleaseWithOpenLinkedIssue,
		prBugAfterLastRelease,
		prBugAtEndTag, // edge case
		prAfterLastReleaseWithClosedLinkedIssue,
		prFeatureAfterEndTag,
		// why not this one? PRs with these labels should explicitly be used in the changelog directly (not the corresponding linked issue)
		prBugAfterLastReleaseWithClosedLinkedIssue, // = issue "issueClosedAfterLastRelease",
	}

	tests := []struct {
		name           string
		since          *git.Tag
		until          *git.Tag
		config         Config
		inputPrs       []ghPullRequest
		expectedIssues []ghIssue
	}{
		{
			name:  "keep changes between the tags for only closed PRs with linked closed issues",
			since: sinceTag,
			until: untilTag,
			config: Config{
				ExcludeLabels:      nil,
				ChangeTypesByLabel: changeTypeSet,
			},
			inputPrs: input,
			expectedIssues: []ghIssue{
				issueClosedAfterLastRelease2,
			},
		},
		{
			name:  "keep changes after start tag",
			since: sinceTag,
			config: Config{
				ExcludeLabels:      nil,
				ChangeTypesByLabel: changeTypeSet,
			},
			inputPrs: input,
			expectedIssues: []ghIssue{
				issueClosedAfterLastRelease2,
				issueClosedAfterLastRelease3,
			},
		},
		{
			name:  "keep only added features after start tag",
			since: sinceTag,
			config: Config{
				ExcludeLabels:      []string{"bug"},
				ChangeTypesByLabel: changeTypeSet,
			},
			inputPrs: input,
			expectedIssues: []ghIssue{
				issueClosedAfterLastRelease3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.expectedIssues, issuesExtractedFromPRs(tt.config, tt.inputPrs, tt.since, tt.until))
		})
	}
}
