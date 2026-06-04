package render

import (
	"testing"

	"github.com/anchore/chronicle/chronicle/dependency"
)

func TestSummaryLine(t *testing.T) {
	// vuln builds a change carrying the given remediated/introduced IDs so the
	// diff's derived counts drive the vulnerability clause.
	vuln := func(kind dependency.ChangeKind, remediated, introduced []string) dependency.PackageChange {
		toVulns := func(ids []string) []dependency.Vulnerability {
			var out []dependency.Vulnerability
			for _, id := range ids {
				out = append(out, dependency.Vulnerability{ID: id})
			}
			return out
		}
		return dependency.PackageChange{
			Kind: kind,
			Vuln: &dependency.VulnDelta{Remediated: toVulns(remediated), Introduced: toVulns(introduced)},
		}
	}

	tests := []struct {
		name    string
		changes []dependency.PackageChange
		want    string
	}{
		{
			name: "breakdown derived from changes",
			changes: []dependency.PackageChange{
				{Kind: dependency.Updated}, {Kind: dependency.Updated}, {Kind: dependency.Added}, {Kind: dependency.Removed},
			},
			want: "4 dependency changes (2 updated, 1 added, 1 removed).",
		},
		{
			name:    "single change is singular",
			changes: []dependency.PackageChange{{Kind: dependency.Updated}},
			want:    "1 dependency change (1 updated).",
		},
		{
			name: "with remediated and introduced",
			changes: []dependency.PackageChange{
				vuln(dependency.Updated, []string{"CVE-1", "CVE-2", "CVE-3"}, nil),
				vuln(dependency.Downgraded, nil, []string{"CVE-9"}),
			},
			want: "2 dependency changes (1 updated, 1 downgraded). 3 vulnerabilities remediated and 1 vulnerability introduced.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dependency.NewDiff(tt.changes)
			if got := SummaryLine(d); got != tt.want {
				t.Errorf("SummaryLine() =\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}
