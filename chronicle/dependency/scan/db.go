package scan

import (
	"errors"
	"fmt"
	"time"

	"github.com/anchore/clio"
	"github.com/anchore/grype/grype"
	v6 "github.com/anchore/grype/grype/db/v6"
	"github.com/anchore/grype/grype/db/v6/distribution"
	"github.com/anchore/grype/grype/db/v6/installation"
	"github.com/anchore/grype/grype/vulnerability"
)

// grypeID identifies chronicle to grype as "grype" so the DB resolves to grype's
// own cache location (~/.cache/grype/db/...) — chronicle shares the grype CLI's
// database rather than maintaining a separate one.
var grypeID = clio.Identification{Name: "grype"}

// DB is a loaded grype vulnerability database, produced by LoadDB and handed to
// NewScanner to match against. It is opaque to callers outside this package (the
// dependency core and the command layer never touch grype types).
type DB struct {
	provider vulnerability.Provider
}

// DBStale reports whether the locally-installed grype vulnerability DB is missing
// or was built more than maxAge ago. It reads the on-disk DB description only — no
// network — so the caller can decide whether to spend time updating before the
// (potentially slow, networked) LoadDB. A missing DB reports stale=true; an
// unreadable DB reports stale=true with the error so the caller can still choose
// to attempt a refresh.
func DBStale(maxAge time.Duration) (stale bool, err error) {
	desc, err := v6.ReadDescription(installation.DefaultConfig(grypeID).DBFilePath())
	if err != nil {
		if errors.Is(err, v6.ErrDBDoesNotExist) {
			return true, nil
		}
		return true, fmt.Errorf("unable to read vulnerability DB status: %w", err)
	}
	return time.Since(desc.Built.Time) > maxAge, nil
}

// LoadDB loads the grype vulnerability DB into a provider the scanner matches
// against. When update is set it first downloads the latest DB (the slow, network
// step); otherwise it opens the on-disk DB directly. A local-only load is retried
// with an update if grype rejects the on-disk DB (e.g. it aged past grype's own
// max-allowed age between the staleness check and here), so a stale DB never
// fails the load outright.
func LoadDB(update bool) (*DB, error) {
	provider, err := loadProvider(update)
	if err != nil && !update {
		provider, err = loadProvider(true)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to load vulnerability DB: %w", err)
	}
	return &DB{provider: provider}, nil
}

func loadProvider(update bool) (vulnerability.Provider, error) {
	provider, _, err := grype.LoadVulnerabilityDB(distribution.DefaultConfig(), installation.DefaultConfig(grypeID), update)
	return provider, err
}
