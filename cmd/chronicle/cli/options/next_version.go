package options

import (
	"github.com/anchore/fangs"
)

type EnforceV0 bool

func (c *EnforceV0) AddFlags(flags fangs.FlagSet) {
	flags.BoolVarP(
		(*bool)(c),
		"enforce-v0", "e",
		"major changes bump the minor version field for versions < 1.0",
	)
}

var _ fangs.FlagAdder = (*EnforceV0)(nil)
