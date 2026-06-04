package options

import (
	"strings"

	"github.com/anchore/clio"
)

// Dependencies holds configuration for scanning and diffing source dependencies
// between refs. The feature is enabled when Ecosystems is non-empty; each
// ecosystem value is a syft cataloger selection expression (e.g. "language",
// "go", "python").
type Dependencies struct {
	Ecosystems                   []string          `yaml:"ecosystems" json:"ecosystems" mapstructure:"ecosystems"`
	Exclude                      []string          `yaml:"exclude" json:"exclude" mapstructure:"exclude"`
	AnnotateVulnerabilities      bool              `yaml:"annotate-vulnerabilities" json:"annotate-vulnerabilities" mapstructure:"annotate-vulnerabilities"`
	OnlyVulnerable               bool              `yaml:"only-vulnerable" json:"only-vulnerable" mapstructure:"only-vulnerable"`
	ShowRemainingVulnerabilities bool              `yaml:"show-remaining-vulnerabilities" json:"show-remaining-vulnerabilities" mapstructure:"show-remaining-vulnerabilities"`
	MinSeverity                  string            `yaml:"min-severity" json:"min-severity" mapstructure:"min-severity"`
	DetectToolchain              bool              `yaml:"detect-toolchain" json:"detect-toolchain" mapstructure:"detect-toolchain"`
	Actions                      DependencyActions `yaml:"actions" json:"actions" mapstructure:"actions"`
}

// DependencyActions sets how each kind of dependency change is displayed. Each
// value is a comma-separated fallback list of modes (hide, summary, list,
// collapsed); the encoder uses the first mode it supports — e.g. "collapsed,list"
// collapses in markdown but enumerates in slack (which can't collapse).
type DependencyActions struct {
	Updated    string `yaml:"updated" json:"updated" mapstructure:"updated"`
	Downgraded string `yaml:"downgraded" json:"downgraded" mapstructure:"downgraded"`
	Added      string `yaml:"added" json:"added" mapstructure:"added"`
	Removed    string `yaml:"removed" json:"removed" mapstructure:"removed"`
}

// CleanedEcosystems normalizes the configured ecosystem values: each entry may
// itself be comma-separated, so flatten, trim, and drop blanks. This is the
// single source for both feature enablement and the selectors handed to syft.
func (c Dependencies) CleanedEcosystems() []string {
	var out []string
	for _, entry := range c.Ecosystems {
		for _, part := range strings.Split(entry, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

// Enabled reports whether the dependency diff will run — i.e. at least one
// ecosystem was requested.
func (c Dependencies) Enabled() bool {
	return len(c.CleanedEcosystems()) > 0
}

func (c *Dependencies) DescribeFields(descriptions clio.FieldDescriptionSet) {
	descriptions.Add(&c.Ecosystems, "ecosystems to scan (syft cataloger selection, e.g. language, go, python); enables the feature when set")
	descriptions.Add(&c.Exclude, "paths to exclude from dependency scanning (syft exclude patterns; each must start with ./, */, or **/, e.g. ./vendor, **/testdata)")
	descriptions.Add(&c.AnnotateVulnerabilities, "annotate dependency changes with known vulnerability information")
	descriptions.Add(&c.OnlyVulnerable, "only show dependency changes that remediated or introduced a vulnerability (requires annotate-vulnerabilities)")
	descriptions.Add(&c.ShowRemainingVulnerabilities, "show the remaining (carried-over) vulnerabilities still present in the latest scan that this release did not remediate, as a rollup (requires annotate-vulnerabilities)")
	descriptions.Add(&c.MinSeverity, "minimum vulnerability severity to include in annotations (e.g. low, medium, high, critical)")
	descriptions.Add(&c.DetectToolchain, "detect declared toolchain minimum-version changes (e.g. the go directive in go.mod) for the activated ecosystems, shown as a Toolchains rollup under Dependencies")
	descriptions.Add(&c.Actions, "how each change kind is displayed: hide, summary (count only), list (bullet list), or collapsed (bullet list in a <details> block)")
}

var _ clio.FieldDescriber = (*Dependencies)(nil)

func (c *DependencyActions) DescribeFields(descriptions clio.FieldDescriptionSet) {
	const help = "comma-separated fallback modes (hide, summary, list, collapsed); first supported by the format wins"
	descriptions.Add(&c.Updated, "display for updated packages: "+help)
	descriptions.Add(&c.Downgraded, "display for downgraded packages: "+help)
	descriptions.Add(&c.Added, "display for added packages: "+help)
	descriptions.Add(&c.Removed, "display for removed packages: "+help)
}

var _ clio.FieldDescriber = (*DependencyActions)(nil)

// DefaultDependencies returns the default configuration for dependency scanning.
// Ecosystems is empty (feature off); when enabled, "language" is the
// recommended value. Every change kind defaults to collapsed — a count that
// expands to a full list — falling back to a full list in formats that can't
// collapse (e.g. slack, md-pretty) so no section is reduced to a bare count.
func DefaultDependencies() Dependencies {
	return Dependencies{
		Ecosystems:                   nil,
		Exclude:                      nil,
		AnnotateVulnerabilities:      false,
		OnlyVulnerable:               false,
		ShowRemainingVulnerabilities: false,
		MinSeverity:                  "",
		// toolchain detection rides on the dependencies feature for the activated
		// ecosystems; on by default so a go-directive bump surfaces without extra flags.
		DetectToolchain: true,
		Actions: DependencyActions{
			Updated:    "collapsed,list",
			Downgraded: "collapsed,list",
			Added:      "collapsed,list",
			Removed:    "collapsed,list",
		},
	}
}
