package options

import (
	"github.com/anchore/chronicle/chronicle/release/toolchain"
	"github.com/anchore/clio"
)

// Toolchain configures detection of declared toolchain-requirement changes (e.g. a bump to the
// minimum Go version in go.mod) between the changelog's since and until points. It is opt-in.
type Toolchain struct {
	Enabled    bool               `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
	Ecosystems []string           `yaml:"ecosystems" json:"ecosystems" mapstructure:"ecosystems"`
	Ignore     []string           `yaml:"ignore" json:"ignore" mapstructure:"ignore"`
	Go         ToolchainEcosystem `yaml:"go" json:"go" mapstructure:"go"`
	Python     ToolchainEcosystem `yaml:"python" json:"python" mapstructure:"python"`
	Node       ToolchainEcosystem `yaml:"node" json:"node" mapstructure:"node"`
}

// ToolchainEcosystem holds per-ecosystem detection settings.
type ToolchainEcosystem struct {
	Paths []string `yaml:"paths" json:"paths" mapstructure:"paths"`
}

var _ clio.FieldDescriber = (*Toolchain)(nil)
var _ clio.FlagAdder = (*Toolchain)(nil)

func (c *Toolchain) DescribeFields(descriptions clio.FieldDescriptionSet) {
	descriptions.Add(&c.Enabled, "detect toolchain requirement changes (e.g. minimum Go version) between the since and until points")
	descriptions.Add(&c.Ecosystems, "ecosystems to inspect (e.g. go, python, node); empty means all known")
	descriptions.Add(&c.Ignore, "path globs excluded from toolchain source discovery")
	descriptions.Add(&c.Go, "Go toolchain detection options")
	descriptions.Add(&c.Python, "Python toolchain detection options")
	descriptions.Add(&c.Node, "Node toolchain detection options")
}

func (c *Toolchain) AddFlags(flags clio.FlagSet) {
	flags.BoolVarP(
		&c.Enabled,
		"detect-toolchain", "",
		"detect toolchain requirement changes (e.g. minimum Go version) between the since and until points",
	)
	flags.StringArrayVarP(
		&c.Ecosystems,
		"toolchain-ecosystems", "",
		"ecosystems to inspect for toolchain changes (e.g. go, python, node); empty means all known",
	)
}

func (c *ToolchainEcosystem) DescribeFields(descriptions clio.FieldDescriptionSet) {
	descriptions.Add(&c.Paths, "path globs to inspect for this ecosystem (e.g. **/go.mod); overrides the default")
}

var _ clio.FieldDescriber = (*ToolchainEcosystem)(nil)

// ToToolchainConfig translates the user-facing options into the detection config. Per-ecosystem
// path overrides are only passed through when set, so the detector defaults apply otherwise.
func (c Toolchain) ToToolchainConfig() toolchain.Config {
	paths := map[string][]string{}
	if len(c.Go.Paths) > 0 {
		paths["go"] = c.Go.Paths
	}
	if len(c.Python.Paths) > 0 {
		paths["python"] = c.Python.Paths
	}
	if len(c.Node.Paths) > 0 {
		paths["node"] = c.Node.Paths
	}
	return toolchain.Config{
		Enabled:    c.Enabled,
		Ecosystems: c.Ecosystems,
		Ignore:     c.Ignore,
		Paths:      paths,
	}
}

func DefaultToolchain() Toolchain {
	return Toolchain{
		Enabled:    false,
		Ecosystems: nil, // all known
		Ignore: []string{
			"**/vendor/**",
			"**/node_modules/**",
			"**/testdata/**",
			"**/examples/**",
		},
		// defaults derive from the detectors so the discovery globs have a single source of truth.
		Go:     ToolchainEcosystem{Paths: toolchain.DefaultPaths("go")},
		Python: ToolchainEcosystem{Paths: toolchain.DefaultPaths("python")},
		Node:   ToolchainEcosystem{Paths: toolchain.DefaultPaths("node")},
	}
}
