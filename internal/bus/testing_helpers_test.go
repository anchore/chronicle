package bus

import "github.com/anchore/chronicle/chronicle/event"

// resetSummaryCache clears the package-level state used by ReportSummary so
// tests can run independently. Test-only.
func resetSummaryCache() {
	cacheMu.Lock()
	lastGroups = nil
	lastTrees = nil
	groupByName = make(map[string]*event.Group)
	treeByName = make(map[string]*event.Tree)
	cacheMu.Unlock()

	identityMu.Lock()
	idRepo = ""
	identityMu.Unlock()
}
