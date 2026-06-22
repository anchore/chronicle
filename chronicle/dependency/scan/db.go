package scan

import (
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

// DBStatus reports the on-disk grype DB state without any network access.
// present is false when no DB is installed or its status is unreadable; age is
// how long ago it was built (meaningful only when present). The caller uses this
// to decide whether to update (when enabled) and whether to warn about staleness.
func DBStatus() (present bool, age time.Duration) {
	desc, err := v6.ReadDescription(installation.DefaultConfig(grypeID).DBFilePath())
	if err != nil || desc == nil {
		return false, 0
	}
	return true, time.Since(desc.Built.Time)
}

// LoadDB loads the grype vulnerability DB into a provider the scanner matches
// against. When update is set it first downloads the latest DB (the slow, network
// step); otherwise it opens whatever DB is on disk directly. The on-disk DB is
// loaded regardless of its age (see loadProvider) — staleness is the caller's
// concern to warn about, not an error here.
func LoadDB(update bool) (*DB, error) {
	provider, err := loadProvider(update)
	if err != nil {
		return nil, fmt.Errorf("unable to load vulnerability DB: %w", err)
	}
	return &DB{provider: provider}, nil
}

func loadProvider(update bool) (vulnerability.Provider, error) {
	installCfg := installation.DefaultConfig(grypeID)
	// don't let an over-age on-disk DB turn into a load error: chronicle decides
	// whether to update (when enabled) and warns on staleness itself. Checksum
	// validation stays on, so a genuinely corrupt DB still errors (→ degrade).
	installCfg.ValidateAge = false
	provider, _, err := grype.LoadVulnerabilityDB(distribution.DefaultConfig(), installCfg, update)
	return provider, err
}
