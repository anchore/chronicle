package json

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
)

func TestEncoder_RoundTrip(t *testing.T) {
	in := release.Description{
		Release: release.Release{
			Version: "v1.2.3",
			Date:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		VCSReferenceURL: "https://example.com/tree/v1.2.3",
		VCSChangesURL:   "https://example.com/compare/v1.2.2...v1.2.3",
		Changes: []change.Change{
			{
				ChangeTypes: []change.Type{change.NewType("bug", change.SemVerPatch)},
				Text:        "fix something <important> & sharp",
				References:  []change.Reference{{Text: "#1", URL: "https://example.com/pull/1"}},
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, (&Encoder{}).Encode(&buf, "ignored", in))

	// HTML escaping is explicitly disabled — `<`, `>`, `&` must survive verbatim
	// so JSON consumers see the same text the user wrote.
	require.Contains(t, buf.String(), "<important>")
	require.Contains(t, buf.String(), "& sharp")

	// indentation is two-space.
	require.True(t, strings.Contains(buf.String(), "\n  \""), "expected pretty-printed output")

	var out release.Description
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))

	require.Equal(t, in.Version, out.Version)
	require.True(t, in.Date.Equal(out.Date))
	require.Equal(t, in.VCSReferenceURL, out.VCSReferenceURL)
	require.Equal(t, in.VCSChangesURL, out.VCSChangesURL)
	require.Len(t, out.Changes, 1)
	require.Equal(t, in.Changes[0].Text, out.Changes[0].Text)
	require.Equal(t, in.Changes[0].References, out.Changes[0].References)
}
