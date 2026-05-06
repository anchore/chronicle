package version

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/release"
)

func TestEncoder(t *testing.T) {
	tests := []struct {
		name    string
		desc    release.Description
		want    string
		wantErr require.ErrorAssertionFunc
	}{
		{
			name: "happy path",
			desc: release.Description{Release: release.Release{Version: "v1.2.3"}},
			want: "v1.2.3\n",
		},
		{
			name:    "empty version errors",
			desc:    release.Description{},
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			var buf bytes.Buffer
			err := (&Encoder{}).Encode(&buf, "", tt.desc)
			tt.wantErr(t, err)
			if err != nil {
				return
			}
			require.Equal(t, tt.want, buf.String())
		})
	}
}
