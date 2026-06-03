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
