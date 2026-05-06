package commands

import (
	"github.com/anchore/chronicle/cmd/chronicle/cli/options"
	"github.com/anchore/clio"
)

type createConfig struct {
	options.Output       `yaml:",inline" json:",inline" mapstructure:",squash"`
	SinceTag             string                   `yaml:"since-tag" json:"since-tag" mapstructure:"since-tag"`                                        // -s, the tag to start the changelog from
	UntilTag             string                   `yaml:"until-tag" json:"until-tag" mapstructure:"until-tag"`                                        // -u, the tag to end the changelog at
	Title                string                   `yaml:"title" json:"title" mapstructure:"title"`                                                    // -t, the title template
	Github               options.GithubSummarizer `yaml:"github" json:"github" mapstructure:"github"`                                                 // GitHub-specific configuration
	SpeculateNextVersion bool                     `yaml:"speculate-next-version" json:"speculate-next-version" mapstructure:"speculate-next-version"` // -n, guess the next version based on issues and PRs
	RepoPath             string                   `yaml:"repo-path" json:"repo-path" mapstructure:"-"`
	EnforceV0            options.EnforceV0        `yaml:"enforce-v0" json:"enforce-v0" mapstructure:"enforce-v0"`
}

var _ clio.FlagAdder = (*createConfig)(nil)
var _ clio.FieldDescriber = (*createConfig)(nil)

func (c *createConfig) DescribeFields(descriptions clio.FieldDescriptionSet) {
	descriptions.Add(&c.SinceTag, "git tag to start changelog processing from (inclusive)")
	descriptions.Add(&c.UntilTag, "git tag to end changelog processing at (inclusive)")
	descriptions.Add(&c.Title, "title template for the changelog output")
	descriptions.Add(&c.Github, "GitHub-specific configuration options")
	descriptions.Add(&c.SpeculateNextVersion, "guess the next version based on issues and PRs")
	descriptions.Add(&c.EnforceV0, "major changes bump minor version for versions < 1.0")
}

func (c *createConfig) AddFlags(flags clio.FlagSet) {
	flags.StringVarP(
		&c.SinceTag,
		"since-tag", "s",
		"tag to start changelog processing from (inclusive)",
	)

	flags.StringVarP(
		&c.UntilTag,
		"until-tag", "u",
		"tag to end changelog processing at (inclusive)",
	)

	flags.StringVarP(
		&c.Title,
		"title", "t",
		"The title of the changelog output",
	)

	flags.BoolVarP(
		&c.SpeculateNextVersion,
		"speculate-next-version", "n",
		"guess the next release version based off of issues and PRs in cases where there is no semver tag after --since-tag (cannot use with --until-tag)",
	)
}

func defaultCreateConfig() *createConfig {
	return &createConfig{
		Output:               options.DefaultOutput(),
		SinceTag:             "",
		UntilTag:             "",
		Title:                `{{ .Version }}`,
		RepoPath:             "",
		SpeculateNextVersion: false,
		EnforceV0:            false,
		Github:               options.DefaultGithubSimmarizer(),
	}
}
