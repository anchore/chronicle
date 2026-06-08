package event

import (
	"sync"

	"github.com/wagoodman/go-progress"
)

// Leaf is a single line within a Tree group (e.g. "commits", "issues",
// "pull requests"). A leaf may itself carry child leaves one level deep (e.g.
// "source sbom" with "since"/"until" branches). Concurrency-safe; satisfies
// progress.StagedProgressable.
type Leaf struct {
	progress.Stage
	progress.Manual

	mu       sync.Mutex
	name     string
	state    SlotState
	metrics  []Metric // raw resolved figures; the UI formats them
	dropped  int      // optional "(N dropped)" secondary; 0 renders nothing
	err      error
	children []*Leaf

	// live is an optional progress source whose stage string drives the
	// running detail (e.g. syft's live "142 packages" count as it catalogs).
	// Once the leaf resolves, the resolved count takes over and live is ignored.
	live progress.StagedProgressable
}

// LeafSpec declares one leaf in a Tree, optionally with named children one
// level deep.
type LeafSpec struct {
	Name     string
	Children []string
}

// Tree is a tree-style group of leaves (e.g. "evidence" with commits/issues/PRs).
// Methods are nil-safe so worker code can be written without scaffolding when
// no publisher is attached.
type Tree struct {
	Header string

	mu     sync.Mutex
	order  []string
	leaves map[string]*Leaf
}

func newLeafNode(name string) *Leaf {
	return &Leaf{
		Manual: *progress.NewManual(-1),
		name:   name,
		state:  SlotPending,
	}
}

// NewTree constructs a flat Tree with its declared leaves in order.
func NewTree(header string, names []string) *Tree {
	specs := make([]LeafSpec, len(names))
	for i, n := range names {
		specs[i] = LeafSpec{Name: n}
	}
	return NewTreeWithChildren(header, specs)
}

// NewTreeWithChildren constructs a Tree whose leaves may each carry child
// leaves one level deep.
func NewTreeWithChildren(header string, specs []LeafSpec) *Tree {
	t := &Tree{
		Header: header,
		leaves: make(map[string]*Leaf, len(specs)),
		order:  make([]string, 0, len(specs)),
	}
	for _, spec := range specs {
		l := newLeafNode(spec.Name)
		for _, cn := range spec.Children {
			l.children = append(l.children, newLeafNode(cn))
		}
		t.leaves[spec.Name] = l
		t.order = append(t.order, spec.Name)
	}
	return t
}

// Leaf returns the named leaf. Returns nil if the tree is nil or the name is
// missing — callers may still chain method calls because Leaf methods are
// nil-safe.
func (t *Tree) Leaf(name string) *Leaf {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.leaves[name]
}

// Names returns the leaf names in declared order.
func (t *Tree) Names() []string {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]string, len(t.order))
	copy(out, t.order)
	return out
}

// Close marks any still-running leaves (and their children) as resolved and
// completes their progress.
func (t *Tree) Close() {
	if t == nil {
		return
	}
	t.mu.Lock()
	leaves := make([]*Leaf, 0, len(t.leaves))
	for _, l := range t.leaves {
		leaves = append(leaves, l)
	}
	t.mu.Unlock()
	for _, l := range leaves {
		l.closeRecursive()
	}
}

func (l *Leaf) closeRecursive() {
	l.mu.Lock()
	if l.state == SlotPending || l.state == SlotRunning {
		l.state = SlotResolved
	}
	children := l.children
	l.mu.Unlock()
	l.SetCompleted()
	for _, c := range children {
		c.closeRecursive()
	}
}

// Child returns the named child of this leaf, or nil. Nil-safe so callers may
// chain method calls.
func (l *Leaf) Child(name string) *Leaf {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, c := range l.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

// Children returns this leaf's child leaves in declared order.
func (l *Leaf) Children() []*Leaf {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]*Leaf, len(l.children))
	copy(out, l.children)
	return out
}

// BindLive attaches a live progress source whose current stage string is shown
// as the running detail (e.g. syft's "142 packages" as it catalogs). Flips a
// pending leaf to running.
func (l *Leaf) BindLive(p progress.StagedProgressable) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.live = p
	if l.state == SlotPending {
		l.state = SlotRunning
	}
	l.mu.Unlock()
}

// RunningDetail is the text shown while the leaf is running: the bound live
// source's current stage if one is set (the live package count), otherwise the
// manual stage detail set via SetStage.
func (l *Leaf) RunningDetail() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	live := l.live
	stage := l.Stage.Current
	l.mu.Unlock()
	if live != nil {
		if s := live.Stage(); s != "" {
			return s
		}
	}
	return stage
}

// Name returns the leaf's name.
func (l *Leaf) Name() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.name
}

// State returns the current leaf state.
func (l *Leaf) State() SlotState {
	if l == nil {
		return SlotPending
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state
}

// Metrics returns the resolved figures (e.g. [{"", 47}] or [{"added", 10}, …]).
// The UI decides how they appear.
func (l *Leaf) Metrics() []Metric {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Metric, len(l.metrics))
	copy(out, l.metrics)
	return out
}

// Dropped returns the raw count of fetched items not associated with the
// release, which the UI renders as a dim "(N dropped)" trailer. 0 means none.
func (l *Leaf) Dropped() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.dropped
}

// Err returns the failure error, if any.
func (l *Leaf) Err() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err
}

// Start flips the leaf to the running state. Optional.
func (l *Leaf) Start() {
	if l == nil {
		return
	}
	l.mu.Lock()
	if l.state == SlotPending {
		l.state = SlotRunning
	}
	l.mu.Unlock()
}

// Resolve sets the leaf's resolved figures. The UI formats them: a single
// unnamed metric renders as a bare number, a single named metric as a
// pluralized count ("142 packages"), and several as a "name=count" breakdown.
func (l *Leaf) Resolve(metrics ...Metric) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.metrics = append(l.metrics[:0], metrics...)
	l.state = SlotResolved
	l.mu.Unlock()
	l.SetCompleted()
}

// SetDropped records how many fetched items were dropped (not associated with
// the release). The UI renders it as a dim "(N dropped)" trailer; 0 renders
// nothing. Independent of Resolve, so call order does not matter.
func (l *Leaf) SetDropped(n int) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.dropped = n
	l.mu.Unlock()
}

// Skip marks the leaf as intentionally not run — e.g. the analysis
// short-circuited before this fetch was needed. The leaf carries no count; the
// UI renders a distinct "skipped" state rather than a completed value.
func (l *Leaf) Skip() {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.state = SlotSkipped
	l.mu.Unlock()
	l.SetCompleted()
}

// Fail flips the leaf to the failed state.
func (l *Leaf) Fail(err error) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.err = err
	l.state = SlotFailed
	l.mu.Unlock()
	l.SetError(err)
}

// SetStage updates the mid-resolution detail string (e.g. "page 4 — 164 received").
func (l *Leaf) SetStage(detail string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.Stage.Current = detail
	if l.state == SlotPending {
		l.state = SlotRunning
	}
	l.mu.Unlock()
}
