package options

import (
	"github.com/anchore/fangs"
)

type NextVersion struct {
	EnforceV0            bool `yaml:"enforce-v0" json:"enforce-v0" mapstructure:"enforce-v0"`
	SpeculateNextVersion bool `yaml:"speculate-next-version" json:"speculate-next-version" mapstructure:"speculate-next-version"` // -n, guess the next version based on issues and PRs
}

func (c *NextVersion) AddFlags(flags fangs.FlagSet) {
	flags.BoolVarP(
		&c.SpeculateNextVersion,
		"speculate-next-version", "n",
		"guess the next release version based off of issues and PRs in cases where there is no semver tag after --since-tag (cannot use with --until-tag)",
	)

	flags.BoolVarP(
		&c.EnforceV0,
		"enforce-v0", "e",
		"major changes bump the minor version field for versions < 1.0",
	)
}
