package bus

// resetIdentity clears the package-level repo identity so tests run
// independently. Test-only.
func resetIdentity() {
	identityMu.Lock()
	idRepo = ""
	identityMu.Unlock()
}
