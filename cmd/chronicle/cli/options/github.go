package options

import (
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/chronicle/release/releasers/github"
)

type GithubSummarizer struct {
	Host                            string         `yaml:"host" json:"host" mapstructure:"host"`
	ExcludeLabels                   []string       `yaml:"exclude-labels" json:"exclude-labels" mapstructure:"exclude-labels"`
	IncludeIssuePRAuthors           bool           `yaml:"include-issue-pr-authors" json:"include-issue-pr-authors" mapstructure:"include-issue-pr-authors"`
	IncludeIssuePRs                 bool           `yaml:"include-issue-prs" json:"include-issue-prs" mapstructure:"include-issue-prs"`
	IncludeIssuesClosedAsNotPlanned bool           `yaml:"include-issues-not-planned" json:"include-issues-not-planned" mapstructure:"include-issues-not-planned"`
	IncludePRs                      bool           `yaml:"include-prs" json:"include-prs" mapstructure:"include-prs"`
	IncludeIssues                   bool           `yaml:"include-issues" json:"include-issues" mapstructure:"include-issues"`
	IncludeUnlabeledIssues          bool           `yaml:"include-unlabeled-issues" json:"include-unlabeled-issues" mapstructure:"include-unlabeled-issues"`
	IncludeUnlabeledPRs             bool           `yaml:"include-unlabeled-prs" json:"include-unlabeled-prs" mapstructure:"include-unlabeled-prs"`
	IssuesRequireLinkedPR           bool           `yaml:"issues-require-linked-prs" json:"issues-require-linked-prs" mapstructure:"issues-require-linked-prs"`
	ConsiderPRMergeCommits          bool           `yaml:"consider-pr-merge-commits" json:"consider-pr-merge-commits" mapstructure:"consider-pr-merge-commits"`
	Changes                         []GithubChange `yaml:"changes" json:"changes" mapstructure:"changes"`
}

type GithubChange struct {
	Type       string   `yaml:"name" json:"name" mapstructure:"name"`
	Title      string   `yaml:"title" json:"title" mapstructure:"title"`
	SemVerKind string   `yaml:"semver-field" json:"semver-field" mapstructure:"semver-field"`
	Labels     []string `yaml:"labels" json:"labels" mapstructure:"labels"`
}

func (c GithubSummarizer) ToGithubConfig() github.Config {
	typeSet := make(change.TypeSet)
	for _, c := range c.Changes {
		k := change.ParseSemVerKind(c.SemVerKind)
		t := change.NewType(c.Type, k)
		for _, l := range c.Labels {
			typeSet[l] = t
		}
	}
	return github.Config{
		Host:                            c.Host,
		IncludeIssuePRAuthors:           c.IncludeIssuePRAuthors,
		IncludeIssuePRs:                 c.IncludeIssuePRs,
		IncludeIssues:                   c.IncludeIssues,
		IncludeIssuesClosedAsNotPlanned: c.IncludeIssuesClosedAsNotPlanned,
		IncludePRs:                      c.IncludePRs,
		IncludeUnlabeledIssues:          c.IncludeUnlabeledIssues,
		IncludeUnlabeledPRs:             c.IncludeUnlabeledPRs,
		ExcludeLabels:                   c.ExcludeLabels,
		IssuesRequireLinkedPR:           c.IssuesRequireLinkedPR,
		ConsiderPRMergeCommits:          c.ConsiderPRMergeCommits,
		ChangeTypesByLabel:              typeSet,
	}
}

func NewGithubSummarizer() GithubSummarizer {
	return GithubSummarizer{
		Host:                            "github.com",
		IssuesRequireLinkedPR:           false,
		ConsiderPRMergeCommits:          true,
		IncludePRs:                      true,
		IncludeIssuePRAuthors:           true,
		IncludeIssuePRs:                 true,
		IncludeIssues:                   true,
		IncludeIssuesClosedAsNotPlanned: false,
		IncludeUnlabeledIssues:          true,
		IncludeUnlabeledPRs:             true,
		ExcludeLabels:                   []string{"duplicate", "question", "invalid", "wontfix", "wont-fix", "release-ignore", "changelog-ignore", "ignore"},
		Changes: []GithubChange{
			{
				Type:       "security-fixes",
				Title:      "Security Fixes",
				Labels:     []string{"security", "vulnerability"},
				SemVerKind: change.SemVerPatch.String(),
			},
			{
				Type:       "added-feature",
				Title:      "Added Features",
				Labels:     []string{"enhancement", "feature", "minor"},
				SemVerKind: change.SemVerMinor.String(),
			},
			{
				Type:       "bug-fix",
				Title:      "Bug Fixes",
				Labels:     []string{"bug", "fix", "bug-fix", "patch"},
				SemVerKind: change.SemVerPatch.String(),
			},
			{
				Type:       "breaking-feature",
				Title:      "Breaking Changes",
				Labels:     []string{"breaking", "backwards-incompatible", "breaking-change", "breaking-feature", "major"},
				SemVerKind: change.SemVerMajor.String(),
			},
			{
				Type:       "removed-feature",
				Title:      "Removed Features",
				Labels:     []string{"removed"},
				SemVerKind: change.SemVerMajor.String(),
			},
			{
				Type:       "deprecated-feature",
				Title:      "Deprecated Features",
				Labels:     []string{"deprecated"},
				SemVerKind: change.SemVerMinor.String(),
			},
			{
				Type:       change.UnknownType.Name,
				Title:      "Additional Changes",
				Labels:     []string{},
				SemVerKind: change.UnknownType.Kind.String(),
			},
		},
	}
}
