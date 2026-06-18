package dependency

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEcosystem(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   Ecosystem
		wantOK bool
	}{
		{
			name:   "canonical value",
			input:  "go",
			want:   EcosystemGo,
			wantOK: true,
		},
		{
			name:   "syft package type maps to ecosystem",
			input:  "go-module",
			want:   EcosystemGo,
			wantOK: true,
		},
		{
			name:   "npm package type maps to javascript",
			input:  "npm",
			want:   EcosystemJavaScript,
			wantOK: true,
		},
		{
			name:   "selector alias maps to javascript",
			input:  "javascript",
			want:   EcosystemJavaScript,
			wantOK: true,
		},
		{
			name:   "case-insensitive and trimmed",
			input:  "  Java-Archive ",
			want:   EcosystemJava,
			wantOK: true,
		},
		{
			name:   "jvm family collapses onto java",
			input:  "jenkins-plugin",
			want:   EcosystemJava,
			wantOK: true,
		},
		{
			name:   "unknown type is not recognized",
			input:  "deb",
			want:   EcosystemUnknown,
			wantOK: false,
		},
		{
			name:   "empty is not recognized",
			input:  "",
			want:   EcosystemUnknown,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseEcosystem(tt.input)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEcosystem_LabelAndSelector(t *testing.T) {
	tests := []struct {
		name         string
		eco          Ecosystem
		wantLabel    string
		wantSelector string
	}{
		{
			name:         "go",
			eco:          EcosystemGo,
			wantLabel:    "Go",
			wantSelector: "go",
		},
		{
			name:         "javascript has a distinct label and selector",
			eco:          EcosystemJavaScript,
			wantLabel:    "JavaScript",
			wantSelector: "javascript",
		},
		{
			name:         "dotnet renders the friendly label",
			eco:          EcosystemDotNet,
			wantLabel:    ".NET",
			wantSelector: "dotnet",
		},
		{
			name:         "unknown falls back to the raw value",
			eco:          Ecosystem("cobol"),
			wantLabel:    "cobol",
			wantSelector: "cobol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantLabel, tt.eco.Label())
			require.Equal(t, tt.wantSelector, tt.eco.Selector())
		})
	}
}

func TestEcosystems_AreParseableAndOrdered(t *testing.T) {
	// every canonical ecosystem must round-trip through its own value, and the
	// list is the single source of display order, so it must be non-empty.
	ecos := Ecosystems()
	require.NotEmpty(t, ecos)
	require.Equal(t, EcosystemGo, ecos[0], "Go leads the canonical order")

	for _, e := range ecos {
		got, ok := ParseEcosystem(string(e))
		assert.True(t, ok, "ecosystem %q should parse from its own value", e)
		assert.Equal(t, e, got)
		assert.NotEmpty(t, e.Label())
		assert.NotEmpty(t, e.Selector())
	}
}
