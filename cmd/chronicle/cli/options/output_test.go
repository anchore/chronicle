package options

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/release/output"
)

func TestOutput_Specs(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Output
		want    []output.Spec
		wantErr require.ErrorAssertionFunc
	}{
		{
			name: "default constructor → md to stdout",
			cfg:  DefaultOutput(),
			want: []output.Spec{{Name: "md"}},
		},
		{
			name: "single output passthrough",
			cfg:  Output{Outputs: []string{"json"}},
			want: []output.Spec{{Name: "json"}},
		},
		{
			name: "deprecated version-file appended",
			cfg:  Output{Outputs: []string{"md"}, VersionFile: "VERSION"},
			want: []output.Spec{{Name: "md"}, {Name: "version", Path: "VERSION"}},
		},
		{
			name: "version-file alone produces just the version spec",
			cfg:  Output{VersionFile: "VERSION"},
			want: []output.Spec{{Name: "version", Path: "VERSION"}},
		},
		{
			name: "multiple outputs with files",
			cfg: Output{Outputs: []string{
				"md=CHANGELOG.md",
				"version=VERSION",
				"json",
			}},
			want: []output.Spec{
				{Name: "md", Path: "CHANGELOG.md"},
				{Name: "version", Path: "VERSION"},
				{Name: "json"},
			},
		},
		{
			name:    "malformed spec",
			cfg:     Output{Outputs: []string{"md="}},
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			got, err := tt.cfg.Specs()
			tt.wantErr(t, err)
			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("specs mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDefaultOutput_Encoders(t *testing.T) {
	o := DefaultOutput()
	require.ElementsMatch(t, []string{"md", "json", "version"}, o.Available.Names())
}
