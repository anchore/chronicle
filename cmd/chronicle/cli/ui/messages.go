package ui

// ExpireGroupsMsg tells bracketGroup/treeGroup models to flag themselves so
// the bubbly frame's TerminalElement check prunes them on the next
// frame.Update cycle. Emitted by the outer UI on receipt of the application
// exit signal.
type ExpireGroupsMsg struct{}

// PruneTickMsg is a no-op message whose only job is to force a frame.Update
// pass *after* an ExpireGroupsMsg has flipped groups to expired, so the
// IsAlive prune actually fires before the program quits.
type PruneTickMsg struct{}
