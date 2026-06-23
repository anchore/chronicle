package options

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultDependencies_RootOnlyByDefault(t *testing.T) {
	// dependency scanning must default to the repository root only; recursing into
	// subdirs (e.g. a tooling .make/go.mod) is opt-in via recursive: true.
	assert.False(t, DefaultDependencies().Recursive)
}
