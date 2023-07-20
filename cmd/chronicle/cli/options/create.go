package options

import (
	"fmt"

	"github.com/anchore/chronicle/chronicle/release/format"
	"github.com/anchore/fangs"
)

type Create struct {
	Output      string           `yaml:"output" json:"output" mapstructure:"output"`                   // -o, the Presenter hint string to use for report formatting
	VersionFile string           `yaml:"version-file" json:"version-file" mapstructure:"version-file"` // --version-file, the path to a file containing the version to use for the changelog
	SinceTag    string           `yaml:"since-tag" json:"since-tag" mapstructure:"since-tag"`          // -s, the tag to start the changelog from
	UntilTag    string           `yaml:"until-tag" json:"until-tag" mapstructure:"until-tag"`          // -u, the tag to end the changelog at
	Title       string           `yaml:"title" json:"title" mapstructure:"title"`
	Github      GithubSummarizer `yaml:"github" json:"github" mapstructure:"github"`
	NextVersion `yaml:",inline" mapstructure:",squash"`
	Repo        `yaml:",inline" mapstructure:",squash"`
}

var _ fangs.FlagAdder = (*Create)(nil)

func (c *Create) AddFlags(flags fangs.FlagSet) {
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
}

func NewCreate() *Create {
	return &Create{
		Output:      string(format.MarkdownFormat),
		VersionFile: "",
		SinceTag:    "",
		UntilTag:    "",
		Title:       "",
		Repo: Repo{
			RepoPath: "",
		},
		NextVersion: NextVersion{
			SpeculateNextVersion: false,
			EnforceV0:            false,
		},
		Github: NewGithubSummarizer(),
	}
}
