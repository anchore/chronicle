package config

import (
	"fmt"

	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/chronicle/release/summarizer/github"
	"github.com/spf13/viper"
)

type githubSummarizer struct {
	Host          string         `yaml:"host" json:"host" mapstructure:"host"`
	Changes       []githubChange `yaml:"changes" json:"changes" mapstructure:"changes"`
	ExcludeLabels []string       `yaml:"exclude-labels" json:"exclude-labels" mapstructure:"exclude-labels"`
	IncludePRs    bool           `yaml:"include-prs" json:"include-prs" mapstructure:"include-prs"`
	IncludeIssues bool           `yaml:"include-issues" json:"include-issues" mapstructure:"include-issues"`
}

type githubChange struct {
	Type       string   `yaml:"name" json:"name" mapstructure:"name"`
	Title      string   `yaml:"title" json:"title" mapstructure:"title"`
	Labels     []string `yaml:"labels" json:"labels" mapstructure:"labels"`
	SemVerKind string   `yaml:"semver-field" json:"semver-field" mapstructure:"semver-field"`
}

func (cfg githubSummarizer) ToGithubConfig() (github.Config, error) {
	typeSet := make(change.TypeSet)
	for _, c := range cfg.Changes {
		k := change.ParseSemVerKind(c.SemVerKind)
		if k == change.SemVerUnknown {
			return github.Config{}, fmt.Errorf("unknown semver field: %q", k)
		}
		t := change.NewType(c.Type, k)
		for _, l := range c.Labels {
			typeSet[l] = t
		}
	}
	return github.Config{
		Host:               cfg.Host,
		IncludeIssues:      cfg.IncludeIssues,
		IncludePRs:         cfg.IncludePRs,
		ExcludeLabels:      cfg.ExcludeLabels,
		ChangeTypesByLabel: typeSet,
	}, nil
}

func (cfg githubSummarizer) loadDefaultValues(v *viper.Viper) {
	v.SetDefault("github.host", "github.com")
	v.SetDefault("github.include-prs", true)
	v.SetDefault("github.include-issues", true)
	v.SetDefault("github.exclude-labels", []string{"duplicate", "question", "invalid", "wontfix", "wont-fix", "release-ignore", "changelog-ignore", "ignore"})
	v.SetDefault("github.changes", []githubChange{
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
	})
}
