package options

import (
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/anchore/chronicle/chronicle/release/output"
	jsonenc "github.com/anchore/chronicle/chronicle/release/output/encoders/json"
	mdenc "github.com/anchore/chronicle/chronicle/release/output/encoders/markdown"
	mdpretty "github.com/anchore/chronicle/chronicle/release/output/encoders/markdownpretty"
	trunkenc "github.com/anchore/chronicle/chronicle/release/output/encoders/trunk"
	versionenc "github.com/anchore/chronicle/chronicle/release/output/encoders/version"
	"github.com/anchore/chronicle/internal/log"
	"github.com/anchore/clio"
	"github.com/anchore/fangs"
)

// TrunkOptions holds user-configurable settings for the trunk output format.
type TrunkOptions struct {
	Condensed    bool `yaml:"condensed" json:"condensed" mapstructure:"condensed"`
	ShowFiltered bool `yaml:"show-filtered" json:"show-filtered" mapstructure:"show-filtered"`
}

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

	// Trunk holds format-specific options for the trunk encoder.
	Trunk TrunkOptions `yaml:"trunk" json:"trunk" mapstructure:"trunk"`
}

var _ clio.FlagAdder = (*Output)(nil)

// DefaultOutput returns an Output with the standard chronicle encoder set
// (md, json, version, md-pretty, trunk) wired up and a default of markdown-on-stdout.
// TTY detection for md-pretty and trunk happens once at construction time; if stdout
// later turns out to be piped, those encoders fall back gracefully.
func DefaultOutput() Output {
	return Output{
		Available: output.NewEncoders(
			&mdenc.Encoder{},
			&jsonenc.Encoder{},
			&versionenc.Encoder{},
			&mdpretty.Encoder{IsTTY: isStdoutTTY()},
			&trunkenc.Encoder{
				IsTTY:        isStdoutTTY(),
				Condensed:    true,
				ShowFiltered: true,
			},
		),
		Outputs: []string{mdenc.ID},
		Trunk:   TrunkOptions{Condensed: true, ShowFiltered: true},
	}
}

func isStdoutTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
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

	flags.BoolVarP(
		&o.Trunk.Condensed,
		"trunk-condensed", "",
		"trunk format: one row per commit (set to false for expanded, multi-row output)",
	)

	flags.BoolVarP(
		&o.Trunk.ShowFiltered,
		"trunk-show-filtered", "",
		"trunk format: show non-contributing commits and filtered PRs/issues dimmed",
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

// Check runs validation against the available encoder set without opening
// any file sinks. Use it to fail fast on misconfigured -o values before
// kicking off expensive upstream work.
func (o *Output) Check() error {
	specs, err := o.Specs()
	if err != nil {
		return err
	}
	return output.Check(specs, o.Available)
}

// Writer constructs the output writer for the configured specs, validated
// against this Output's available encoder set. The trunk encoder is refreshed
// from the current TrunkOptions so that flag-parsed values take effect even
// though the encoder was constructed before flag parsing ran.
func (o *Output) Writer() (output.Writer, error) {
	// refresh trunk encoder fields from the (now flag-parsed) TrunkOptions
	if enc, ok := o.Available[trunkenc.ID]; ok {
		if te, ok := enc.(*trunkenc.Encoder); ok {
			te.Condensed = o.Trunk.Condensed
			te.ShowFiltered = o.Trunk.ShowFiltered
		}
	}

	specs, err := o.Specs()
	if err != nil {
		return nil, err
	}
	return output.New(specs, o.Available)
}
