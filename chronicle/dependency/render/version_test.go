package render

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/dependency"
)

func TestShortenVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain semver unchanged", in: "v0.23.0", want: "v0.23.0"},
		{name: "incompatible suffix unchanged", in: "v29.4.0+incompatible", want: "v29.4.0+incompatible"},
		{
			name: "pseudo-version on v0.0.0 base",
			in:   "v0.0.0-20211214055906-6f57359322fd",
			want: "v0.0.0-6f57359",
		},
		{
			name: "pseudo-version on release base",
			in:   "v1.2.4-0.20211214055906-6f57359322fd",
			want: "v1.2.4-0.6f57359",
		},
		{
			name: "pseudo-version on pre-release base",
			in:   "v2.0.0-beta.0.20240101000000-abcdef012345",
			want: "v2.0.0-beta.0.abcdef0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ShortenVersion(tt.in))
		})
	}
}

func TestVersionTransition(t *testing.T) {
	require.Equal(t, "v0.0.0-6f57359 → v0.1.0",
		VersionTransition(dependency.PackageChange{Kind: dependency.Updated, FromVersion: "v0.0.0-20211214055906-6f57359322fd", ToVersion: "v0.1.0"}))
	require.Equal(t, "v0.4.0", VersionTransition(dependency.PackageChange{Kind: dependency.Added, ToVersion: "v0.4.0"}))
	require.Equal(t, "v0.9.0", VersionTransition(dependency.PackageChange{Kind: dependency.Removed, FromVersion: "v0.9.0"}))
}

func TestVulnNote(t *testing.T) {
	// unannotated (nil Vuln) and annotated-but-clean both produce no note.
	require.Equal(t, "", VulnNote(dependency.PackageChange{Kind: dependency.Updated}))
	require.Equal(t, "", VulnNote(dependency.PackageChange{Kind: dependency.Updated, Vuln: &dependency.VulnDelta{}}))
	require.Equal(t, "🟢 remediated CVE-1, CVE-2",
		VulnNote(dependency.PackageChange{Kind: dependency.Updated, Vuln: &dependency.VulnDelta{Remediated: []dependency.Vulnerability{{ID: "CVE-2"}, {ID: "CVE-1"}}}}))
	require.Equal(t, "🔴 reintroduces CVE-9",
		VulnNote(dependency.PackageChange{Kind: dependency.Downgraded, Vuln: &dependency.VulnDelta{Introduced: []dependency.Vulnerability{{ID: "CVE-9"}}}}))
	require.Equal(t, "🔴 introduces CVE-9",
		VulnNote(dependency.PackageChange{Kind: dependency.Added, Vuln: &dependency.VulnDelta{Introduced: []dependency.Vulnerability{{ID: "CVE-9"}}}}))
	require.Equal(t, "🟢 remediated CVE-1; 🔴 introduces CVE-9",
		VulnNote(dependency.PackageChange{Kind: dependency.Updated, Vuln: &dependency.VulnDelta{Remediated: []dependency.Vulnerability{{ID: "CVE-1"}}, Introduced: []dependency.Vulnerability{{ID: "CVE-9"}}}}))
}
