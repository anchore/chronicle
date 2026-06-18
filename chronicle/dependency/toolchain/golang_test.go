package toolchain

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoDetector_Compare(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantCmp int
		wantOK  bool
	}{
		{name: "upgrade", from: "1.21", to: "1.23", wantCmp: 1, wantOK: true},
		{name: "downgrade", from: "1.23", to: "1.21", wantCmp: -1, wantOK: true},
		{name: "equal", from: "1.21", to: "1.21", wantCmp: 0, wantOK: true},
		{name: "patch-level upgrade", from: "1.21.0", to: "1.21.4", wantCmp: 1, wantOK: true},
		{name: "not comparable", from: "1.21", to: "not-a-version", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmp, ok := goDetector{}.Compare(tt.from, tt.to)
			assert.Equal(t, tt.wantOK, ok)
			if !ok {
				return
			}
			assert.Equal(t, tt.wantCmp, cmp)
		})
	}
}

func TestGoDetector_Requirement(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []requirement
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:    "reads go directive",
			content: "module example.com/foo\n\ngo 1.21\n",
			want:    []requirement{{source: "go directive", version: "1.21"}},
		},
		{
			name:    "reads patch-level go directive",
			content: "module example.com/foo\n\ngo 1.21.4\n",
			want:    []requirement{{source: "go directive", version: "1.21.4"}},
		},
		{
			name:    "ignores toolchain directive, only reports go directive",
			content: "module example.com/foo\n\ngo 1.21\n\ntoolchain go1.21.4\n",
			want:    []requirement{{source: "go directive", version: "1.21"}},
		},
		{
			name:    "no go directive yields nothing",
			content: "module example.com/foo\n",
			want:    nil,
		},
		{
			// ParseLax tolerates unknown/newer directives rather than erroring.
			name:    "tolerates unknown directives",
			content: "module example.com/foo\n\ngo 1.23\n\ngodebug default=go1.21\n",
			want:    []requirement{{source: "go directive", version: "1.23"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			got, err := goDetector{}.Requirement("go.mod", []byte(tt.content))
			tt.wantErr(t, err)

			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(requirement{})); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
