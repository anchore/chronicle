package render

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/dependency"
)

func TestRemediatedAndIntroducedVulns(t *testing.T) {
	// CVE-1 is remediated on two packages (must collapse to one listing naming
	// both, sorted); CVE-2 remediated on one; CVE-9 introduced. An unannotated
	// change (nil Vuln) contributes nothing.
	diff := dependency.NewDiff([]dependency.PackageChange{
		{
			Name: "pkg-b", Kind: dependency.Updated,
			Vuln: &dependency.VulnDelta{Remediated: []dependency.Vulnerability{
				{ID: "CVE-2", Severity: "high", DataSource: "https://example.com/CVE-2"},
				{ID: "CVE-1", Severity: "low", DataSource: "https://example.com/CVE-1"},
			}},
		},
		{
			Name: "pkg-a", Kind: dependency.Removed,
			Vuln: &dependency.VulnDelta{Remediated: []dependency.Vulnerability{
				{ID: "CVE-1", Severity: "low", DataSource: "https://example.com/CVE-1"},
			}},
		},
		{
			Name: "pkg-c", Kind: dependency.Added,
			Vuln: &dependency.VulnDelta{Introduced: []dependency.Vulnerability{
				{ID: "CVE-9", Severity: "critical", DataSource: "https://example.com/CVE-9"},
			}},
		},
		{Name: "pkg-d", Kind: dependency.Updated}, // unannotated: ignored
	})

	wantRemediated := []VulnListing{
		{ID: "CVE-1", Severity: "low", DataSource: "https://example.com/CVE-1", Packages: []string{"pkg-a", "pkg-b"}},
		{ID: "CVE-2", Severity: "high", DataSource: "https://example.com/CVE-2", Packages: []string{"pkg-b"}},
	}
	wantIntroduced := []VulnListing{
		{ID: "CVE-9", Severity: "critical", DataSource: "https://example.com/CVE-9", Packages: []string{"pkg-c"}},
	}

	if diff := cmp.Diff(wantRemediated, RemediatedVulns(diff)); diff != "" {
		t.Errorf("RemediatedVulns mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantIntroduced, IntroducedVulns(diff)); diff != "" {
		t.Errorf("IntroducedVulns mismatch (-want +got):\n%s", diff)
	}
}

func TestRemediatedAndIntroducedVulns_Empty(t *testing.T) {
	// an unannotated diff yields no listings (not a nil-deref).
	diff := dependency.NewDiff([]dependency.PackageChange{{Name: "x", Kind: dependency.Updated}})
	require.Empty(t, RemediatedVulns(diff))
	require.Empty(t, IntroducedVulns(diff))
	require.Empty(t, RemainingVulns(diff))
}

func TestRemainingVulns(t *testing.T) {
	// remaining reads Diff.Remaining (the carried-over listing the core attributes
	// across the whole latest scan). CVE-1 carries over on two packages and must
	// collapse to one listing naming both (sorted); CVE-2 on one.
	diff := dependency.Diff{
		Remaining: []dependency.PackageVulns{
			{
				Package: dependency.Package{Name: "pkg-a", Type: "go-module"},
				Vulns: []dependency.Vulnerability{
					{ID: "CVE-1", Severity: "low", DataSource: "https://example.com/CVE-1"},
				},
			},
			{
				Package: dependency.Package{Name: "pkg-b", Type: "go-module"},
				Vulns: []dependency.Vulnerability{
					{ID: "CVE-1", Severity: "low", DataSource: "https://example.com/CVE-1"},
					{ID: "CVE-2", Severity: "high", DataSource: "https://example.com/CVE-2"},
				},
			},
		},
	}

	want := []VulnListing{
		{ID: "CVE-1", Severity: "low", DataSource: "https://example.com/CVE-1", Packages: []string{"pkg-a", "pkg-b"}},
		{ID: "CVE-2", Severity: "high", DataSource: "https://example.com/CVE-2", Packages: []string{"pkg-b"}},
	}

	if d := cmp.Diff(want, RemainingVulns(diff)); d != "" {
		t.Errorf("RemainingVulns mismatch (-want +got):\n%s", d)
	}
}
