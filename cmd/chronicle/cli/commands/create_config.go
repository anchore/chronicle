package commands

import (
	"fmt"

	"github.com/anchore/chronicle/chronicle/release/format"
	"github.com/anchore/chronicle/cmd/chronicle/cli/options"
	"github.com/anchore/clio"
)

type createConfig struct {
	Output               string                   `yaml:"output" json:"output" mapstructure:"output"`                   // -o, the Presenter hint string to use for report formatting
	VersionFile          string                   `yaml:"version-file" json:"version-file" mapstructure:"version-file"` // --version-file, the path to a file containing the version to use for the changelog
	SinceTag             string                   `yaml:"since-tag" json:"since-tag" mapstructure:"since-tag"`          // -s, the tag to start the changelog from
	UntilTag             string                   `yaml:"until-tag" json:"until-tag" mapstructure:"until-tag"`          // -u, the tag to end the changelog at
	Title                string                   `yaml:"title" json:"title" mapstructure:"title"`
	Github               options.GithubSummarizer `yaml:"github" json:"github" mapstructure:"github"`
	SpeculateNextVersion bool                     `yaml:"speculate-next-version" json:"speculate-next-version" mapstructure:"speculate-next-version"` // -n, guess the next version based on issues and PRs
	RepoPath             string                   `yaml:"repo-path" json:"repo-path" mapstructure:"-"`
	EnforceV0            options.EnforceV0        `yaml:"enforce-v0" json:"enforce-v0" mapstructure:"enforce-v0"`
}

var _ clio.FlagAdder = (*createConfig)(nil)

func (c *createConfig) AddFlags(flags clio.FlagSet) {
	flags.StringVarP(
		&c.Output,
		"output", "o",
		fmt.Sprintf("output format to use: %+v", format.All()),
	)

	flags.StringVarP(
		&c.VersionFile,
		"version-file", "",
		"output the current version of the generated changelog to the given file",
	)

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
		Output:               string(format.MarkdownFormat),
		VersionFile:          "",
		SinceTag:             "",
		UntilTag:             "",
		Title:                "",
		RepoPath:             "",
		SpeculateNextVersion: false,
		EnforceV0:            false,
		Github:               options.DefaultGithubSimmarizer(),
	}
}
