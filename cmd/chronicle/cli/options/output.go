package options

import (
	"fmt"

	"github.com/anchore/chronicle/chronicle/release/output"
	jsonenc "github.com/anchore/chronicle/chronicle/release/output/encoders/json"
	mdenc "github.com/anchore/chronicle/chronicle/release/output/encoders/markdown"
	versionenc "github.com/anchore/chronicle/chronicle/release/output/encoders/version"
	"github.com/anchore/chronicle/internal/log"
	"github.com/anchore/clio"
	"github.com/anchore/fangs"
)

// Output configures one or more `-o NAME[=PATH]` outputs for a command.
// Embed this in a command's config (squashed) to expose the standard set
// of output flags and decoding behavior.
type Output struct {
	// Available is the encoder set this command will accept. Set by the
	// constructor; not configurable through yaml/flags.
	Available output.Encoders `yaml:"-" json:"-" mapstructure:"-"`

	// Outputs is the user-provided list of `-o NAME[=PATH]` specs.
	Outputs []string `yaml:"output" json:"output" mapstructure:"output"`

	// VersionFile is the deprecated --version-file path. It is folded into
	// the spec list by Specs() and emits a deprecation warning when set.
	VersionFile string `yaml:"version-file" json:"version-file" mapstructure:"version-file"`
}

var _ clio.FlagAdder = (*Output)(nil)

// DefaultOutput returns an Output with the standard chronicle encoder set
// (md, json, version) wired up and a default of markdown-on-stdout.
func DefaultOutput() Output {
	return Output{
		Available: output.NewEncoders(
			&mdenc.Encoder{},
			&jsonenc.Encoder{},
			&versionenc.Encoder{},
		),
		Outputs: []string{mdenc.ID},
	}
}

func (o *Output) AddFlags(flags clio.FlagSet) {
	flags.StringArrayVarP(
		&o.Outputs,
		"output", "o",
		fmt.Sprintf("output format(s); repeat -o for multiple destinations, e.g. -o md=CHANGELOG.md -o version=VERSION (formats: %v)", o.Available.Names()),
	)

	flags.StringVarP(
		&o.VersionFile,
		"version-file", "",
		"deprecated: use -o version=<path> instead",
	)

	// MarkDeprecated both hides the flag from help and prints a one-time
	// notice on stderr whenever the flag is used on the command line. The
	// runtime log.Warn in Specs() covers the yaml/env path that pflag never
	// observes.
	if pfp, ok := flags.(fangs.PFlagSetProvider); ok {
		_ = pfp.PFlagSet().MarkDeprecated("version-file", "use -o version=<path> instead")
	}
}

// Specs resolves the configured Outputs into parsed specs, folding in the
// deprecated --version-file value and emitting a warning if it was used.
func (o *Output) Specs() ([]output.Spec, error) {
	specs, err := output.ParseSpecs(o.Outputs)
	if err != nil {
		return nil, err
	}
	if o.VersionFile != "" {
		log.Warn("--version-file is deprecated; use -o version=<path> instead")
		specs = append(specs, output.Spec{Name: versionenc.ID, Path: o.VersionFile})
	}
	return specs, nil
}

// Writer constructs the output writer for the configured specs, validated
// against this Output's available encoder set.
func (o *Output) Writer() (output.Writer, error) {
	specs, err := o.Specs()
	if err != nil {
		return nil, err
	}
	return output.New(specs, o.Available)
}
