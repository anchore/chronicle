package options

import (
	"github.com/anchore/clio"
)

type EnforceV0 bool

func (c *EnforceV0) AddFlags(flags clio.FlagSet) {
	flags.BoolVarP(
		(*bool)(c),
		"enforce-v0", "e",
		"major changes bump the minor version field for versions < 1.0",
	)
}

var _ clio.FlagAdder = (*EnforceV0)(nil)
