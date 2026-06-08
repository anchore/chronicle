package bus

import "sync"

// identity state — the OWNER/REPO of the repo being scanned. Set by the worker
// once the remote URL parses cleanly so the UI can render its project title
// (live) and the post-teardown recap header. It is raw data; the UI owns the
// "Project: OWNER/REPO" presentation.
var (
	identityMu sync.Mutex
	idRepo     string
)

// SetRepo records the OWNER/REPO of the scanned project.
func SetRepo(r string) {
	identityMu.Lock()
	defer identityMu.Unlock()
	idRepo = r
}

// Repo returns the OWNER/REPO recorded by SetRepo, or "" if not set.
func Repo() string {
	identityMu.Lock()
	defer identityMu.Unlock()
	return idRepo
}
