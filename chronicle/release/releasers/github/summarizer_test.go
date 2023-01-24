package github

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	mergeCommit1 := "commit-pr-1-hash"
	mergeCommit2 := "commit-pr-2-hash"
	mergeCommit3 := "commit-pr-3-hash"
	// note: 4-5 reserved for issues... (not PRs)
	mergeCommit6 := "commit-pr-6-hash"
	mergeCommit7 := "commit-pr-7-hash"
	mergeCommit8 := "commit-pr-8-hash"
	mergeCommit9 := "commit-pr-9-hash"
	mergeCommit10 := "commit-pr-10-hash"
	mergeCommit11 := "commit-pr-11-hash"
	mergeCommit12 := "commit-pr-12-hash"
	// 13 is made up
	mergeCommit14 := "commit-pr-14-hash"

	tagRangeMergeCommits := []string{
		mergeCommit1,
		mergeCommit2,
		mergeCommit3,
		mergeCommit9,
		mergeCommit10,
		mergeCommit11,
		// note: merge commit 13 represents a PR that was merged after the end tag, thus us excluded via commit filter
	}

	outsideRangeMergeCommits := []string{
		mergeCommit6,
		mergeCommit7,
		mergeCommit8,
		mergeCommit12,
		mergeCommit14,
	}

	var allMergeCommits []string
	allMergeCommits = append(allMergeCommits, tagRangeMergeCommits...)
	allMergeCommits = append(allMergeCommits, outsideRangeMergeCommits...)

	prBugAfterLastRelease := ghPullRequest{
		Title:       "pr bug after starting tag",
		Number:      1,
		MergedAt:    timeAfter,
		Labels:      []string{"bug"},
		MergeCommit: mergeCommit1,
	}

	prBugAtLastRelease := ghPullRequest{
		Title:       "pr bug at (within) last release",
		Number:      10,
		MergedAt:    timeStart,
		Labels:      []string{"bug"},
		MergeCommit: mergeCommit10,
	}

	prBugAtEndTag := ghPullRequest{
		Title:       "pr bug at (within) end tag",
		Number:      11,
		MergedAt:    timeEnd,
		Labels:      []string{"bug"},
		MergeCommit: mergeCommit3,
	}

	prAfterLastRelease := ghPullRequest{
		Title:       "pr after starting tag",
		Number:      9,
		MergedAt:    timeAfter,
		MergeCommit: mergeCommit9,
	}

	prBugBeforeLastRelease := ghPullRequest{
		Title:       "pr bug before starting tag",
		Number:      2,
		MergedAt:    timeBefore,
		Labels:      []string{"bug"},
		MergeCommit: mergeCommit2,
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
		Number:       14,
		MergedAt:     timeAfter,
		Labels:       []string{"bug"},
		LinkedIssues: []ghIssue{issueOpen},
		MergeCommit:  mergeCommit14,
	}

	prAfterLastReleaseWithOpenLinkedIssue := ghPullRequest{
		Title:        "pr after starting tag (w/ open linked issue)",
		Number:       6,
		MergedAt:     timeAfter,
		LinkedIssues: []ghIssue{issueOpen},
		MergeCommit:  mergeCommit6,
	}

	prBugAfterLastReleaseWithClosedLinkedIssue := ghPullRequest{
		Title:        "pr bug after starting tag (w/ closed linked issue)",
		Number:       7,
		MergedAt:     timeAfter,
		Labels:       []string{"bug"},
		LinkedIssues: []ghIssue{issueClosedAfterLastRelease},
		MergeCommit:  mergeCommit7,
	}

	prAfterLastReleaseWithClosedLinkedIssue := ghPullRequest{
		Title:        "pr after starting tag (w/ closed linked issue)",
		Number:       8,
		MergedAt:     timeAfter,
		LinkedIssues: []ghIssue{issueClosedAfterLastRelease},
		MergeCommit:  mergeCommit8,
	}

	prFeatureAfterEndTag := ghPullRequest{
		Title:       "pr feature after end tag",
		Number:      12,
		MergedAt:    timeAfterEnd,
		Labels:      []string{"feature"},
		MergeCommit: mergeCommit12,
	}

	prFeatureAfterEndTagAndMergeRange := ghPullRequest{
		Title:       "pr feature after end tag (and merge range)",
		Number:      13,
		MergedAt:    timeAfterEnd,
		Labels:      []string{"feature"},
		MergeCommit: "made-up-commit-hash",
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
		prFeatureAfterEndTagAndMergeRange,
	}

	tests := []struct {
		name        string
		since       *git.Tag
		until       *git.Tag
		commits     []string
		config      Config
		inputPrs    []ghPullRequest
		expectedPrs []ghPullRequest
	}{
		{
			name:  "keep changes between the tags (dont consider PR commits)",
			since: sinceTag,
			until: untilTag,
			config: Config{
				ExcludeLabels:          nil,
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: false,
			},
			inputPrs: input,
			commits:  tagRangeMergeCommits,
			expectedPrs: []ghPullRequest{
				prBugAfterLastRelease,
				prBugAtEndTag,
			},
		},
		{
			name:  "keep changes after start tag (dont consider PR commits)",
			since: sinceTag,
			config: Config{
				ExcludeLabels:          nil,
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: false,
			},
			inputPrs: input,
			commits:  allMergeCommits,
			expectedPrs: []ghPullRequest{
				prBugAfterLastRelease,
				prBugAtEndTag,
				prFeatureAfterEndTag,
				prFeatureAfterEndTagAndMergeRange,
			},
		},
		{
			name:  "keep only added features after start tag (dont consider PR commits)",
			since: sinceTag,
			config: Config{
				ExcludeLabels:          []string{"bug"},
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: false,
			},
			inputPrs: input,
			commits:  allMergeCommits,
			expectedPrs: []ghPullRequest{
				prFeatureAfterEndTag,
				prFeatureAfterEndTagAndMergeRange,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keep := applyStandardPRFilters(tt.inputPrs, tt.config, tt.since, tt.until, tt.commits)
			assert.ElementsMatch(t, tt.expectedPrs, keep)
		})
	}
}

func Test_prFilters_byCommits(t *testing.T) {
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

	mergeCommit12 := "commit-pr-12-hash"

	validMergeCommits := []string{
		mergeCommit12,
	}

	prFeatureAfterEndTag := ghPullRequest{
		Title:       "pr feature after end tag",
		Number:      12,
		MergedAt:    timeAfterEnd, // note: this is after the end tag...
		Labels:      []string{"feature"},
		MergeCommit: mergeCommit12, // continue note: ... but this commit is within the merge range
	}

	prFeatureBeforeEndTagButNotWithinMergeRange := ghPullRequest{
		Title:       "pr feature not within merge range",
		Number:      13,
		MergedAt:    timeAfter, // note: this is during the tag range...
		Labels:      []string{"feature"},
		MergeCommit: "made-up-commit-hash", // continue note: ... but this commit is not within the merge range
	}

	input := []ghPullRequest{
		// keep
		prFeatureAfterEndTag, // re-included via commit range

		// filter out
		prFeatureBeforeEndTagButNotWithinMergeRange,
	}

	tests := []struct {
		name        string
		since       *git.Tag
		until       *git.Tag
		commits     []string
		config      Config
		inputPrs    []ghPullRequest
		expectedPrs []ghPullRequest
	}{
		{
			name:  "add PRs merged after end tag if they are in the commit range",
			since: sinceTag,
			until: untilTag,
			config: Config{
				ExcludeLabels:          nil,
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: true,
			},
			inputPrs: input,
			commits:  validMergeCommits,
			expectedPrs: []ghPullRequest{
				prFeatureAfterEndTag,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keep := applyStandardPRFilters(tt.inputPrs, tt.config, tt.since, tt.until, tt.commits)
			assert.ElementsMatch(t, tt.expectedPrs, keep)
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
		commits        []string
		expectedIssues []ghIssue
	}{
		{
			name:  "keep changes between the tags for only closed PRs with linked closed issues",
			since: sinceTag,
			until: untilTag,
			config: Config{
				ExcludeLabels:          nil,
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: false,
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
				ExcludeLabels:          nil,
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: false,
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
				ExcludeLabels:          []string{"bug"},
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: false,
			},
			inputPrs: input,
			expectedIssues: []ghIssue{
				issueClosedAfterLastRelease3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.expectedIssues, issuesExtractedFromPRs(tt.config, tt.inputPrs, tt.since, tt.until, tt.commits))
		})
	}
}

func Test_createChangesFromIssues(t *testing.T) {
	timeStart := time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC)

	patch := change.NewType("patch", change.SemVerPatch)

	changeTypeSet := change.TypeSet{
		"bug": patch,
	}

	issue1 := ghIssue{
		Title:    "Issue 1",
		Number:   1,
		URL:      "issue-1-url",
		ClosedAt: timeStart,
		Labels:   []string{"bug"},
	}

	issue2 := ghIssue{
		Title:    "Issue 2",
		Number:   2,
		URL:      "issue-2-url",
		ClosedAt: timeStart,
		Labels:   []string{"bug"},
	}

	issue3 := ghIssue{
		Title:    "Issue 3 no PRs",
		Number:   3,
		URL:      "issue-3-url",
		ClosedAt: timeStart,
		Labels:   []string{"bug"},
	}

	prWithLinkedIssues1 := ghPullRequest{
		Title:    "pr 1 with linked issues",
		MergedAt: timeStart,
		Number:   1,
		Labels:   []string{"bug"},
		Author:   "some-author-1",
		URL:      "pr-1-url",
		LinkedIssues: []ghIssue{
			issue1,
		},
	}

	prWithLinkedIssues2 := ghPullRequest{
		Title:    "pr 2 with linked issues",
		MergedAt: timeStart,
		Number:   2,
		Labels:   []string{"another-label"},
		Author:   "some-author-2",
		URL:      "pr-2-url",
		LinkedIssues: []ghIssue{
			issue1,
			issue2,
		},
	}

	prWithoutLinkedIssues1 := ghPullRequest{
		MergedAt: timeStart,
		Title:    "pr 3 without linked issues",
		Number:   3,
		Author:   "some-author",
		URL:      "pr-3-url",
	}

	tests := []struct {
		name            string
		config          Config
		inputPrs        []ghPullRequest
		issues          []ghIssue
		expectedChanges []change.Change
	}{
		{
			name: "includes author for issues",
			config: Config{
				IncludeIssuePRAuthors: true,
				IncludeIssuePRs:       true,
				ChangeTypesByLabel:    changeTypeSet,
				Host:                  "some-host",
			},
			inputPrs: []ghPullRequest{
				prWithLinkedIssues1,
				prWithLinkedIssues2,
				prWithoutLinkedIssues1,
			},
			issues: []ghIssue{
				issue1,
				issue2,
				issue3,
			},
			expectedChanges: []change.Change{
				{
					Text:        "Issue 1",
					ChangeTypes: []change.Type{patch},
					Timestamp:   timeStart,
					References: []change.Reference{
						{
							Text: "Issue #1",
							URL:  "issue-1-url",
						},
						{
							Text: "PR #1",
							URL:  "pr-1-url",
						},
						{
							Text: "some-author-1",
							URL:  "https://some-host/some-author-1",
						},
						{
							Text: "PR #2",
							URL:  "pr-2-url",
						},
						{
							Text: "some-author-2",
							URL:  "https://some-host/some-author-2",
						},
					},
					EntryType: "githubIssue",
					Entry:     issue1,
				},
				{
					Text:        "Issue 2",
					ChangeTypes: []change.Type{patch},
					Timestamp:   timeStart,
					References: []change.Reference{
						{
							Text: "Issue #2",
							URL:  "issue-2-url",
						},
						{
							Text: "PR #2",
							URL:  "pr-2-url",
						},
						{
							Text: "some-author-2",
							URL:  "https://some-host/some-author-2",
						},
					},
					EntryType: "githubIssue",
					Entry:     issue2,
				},
				{
					Text:        "Issue 3 no PRs",
					ChangeTypes: []change.Type{patch},
					Timestamp:   timeStart,
					References: []change.Reference{
						{
							Text: "Issue #3",
							URL:  "issue-3-url",
						},
					},
					EntryType: "githubIssue",
					Entry:     issue3,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := createChangesFromIssues(tt.config, tt.inputPrs, tt.issues)
			if !reflect.DeepEqual(tt.expectedChanges, changes) {
				// print out a JSON diff
				toJson := func(changes []change.Change) string {
					out, err := json.Marshal(changes)
					require.NoError(t, err)
					return string(out)
				}
				assert.JSONEq(t, toJson(tt.expectedChanges), toJson(changes))
			}
		})
	}
}
