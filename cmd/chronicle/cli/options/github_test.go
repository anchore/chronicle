package options

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/anchore/chronicle/chronicle/release/change"
)

func TestGithubSummarizer_ToGithubConfig_prefixes(t *testing.T) {
	feature := change.NewType("added-feature", change.SemVerMinor)
	breaking := change.NewType("breaking-feature", change.SemVerMajor)

	summarizer := GithubSummarizer{
		InferChangeTypeFromTitle: true,
		Changes: []GithubChange{
			{
				Type:       "added-feature",
				SemVerKind: change.SemVerMinor.String(),
				Labels:     []string{"enhancement"},
				// mixed case to exercise normalization
				Prefixes: []string{"feat", "Feature"},
			},
			{
				Type:       "breaking-feature",
				SemVerKind: change.SemVerMajor.String(),
				Prefixes:   []string{change.BreakingChangePrefix},
			},
		},
	}

	cfg := summarizer.ToGithubConfig()

	assert.True(t, cfg.InferChangeTypeFromTitle)

	// prefix keys are lowercased so they match the (lowercase-normalized) parsed
	// PR title type, while the breaking marker is preserved verbatim.
	want := change.TypeSet{
		"feat":                      feature,
		"feature":                   feature,
		change.BreakingChangePrefix: breaking,
	}
	assert.Equal(t, want, cfg.ChangeTypesByConventionalCommitType)
}

func TestGithubSummarizer_PostLoad_backfillsDefaults(t *testing.T) {
	summarizer := GithubSummarizer{
		Changes: []GithubChange{
			// an older-style config entry with labels but no prefixes
			{Type: "bug-fix", Title: "Fixes", Labels: []string{"bug"}},
			// explicit empty prefixes opt out of inference for this type
			{Type: "added-feature", Prefixes: []string{}},
			// unknown to the defaults, left untouched
			{Type: "custom", Labels: []string{"custom"}},
		},
	}

	assert.NoError(t, summarizer.PostLoad())

	want := []GithubChange{
		{Type: "bug-fix", Title: "Fixes", SemVerKind: "patch", Labels: []string{"bug"}, Prefixes: []string{"fix"}},
		{Type: "added-feature", Title: "Added Features", SemVerKind: "minor", Labels: []string{"enhancement", "feature", "minor"}, Prefixes: []string{}},
		{Type: "custom", Labels: []string{"custom"}},
	}
	assert.Equal(t, want, summarizer.Changes)
}
