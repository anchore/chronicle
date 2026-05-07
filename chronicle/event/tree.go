package event

import (
	"sync"

	"github.com/wagoodman/go-progress"
)

// Leaf is a single line within a Tree group (e.g. "commits", "issues",
// "pull requests"). Concurrency-safe; satisfies progress.StagedProgressable.
type Leaf struct {
	progress.Stage
	progress.Manual

	mu    sync.Mutex
	name  string
	state SlotState
	count string
	note  string
	err   error
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

// NewTree constructs a Tree with its declared leaves in order.
func NewTree(header string, names []string) *Tree {
	t := &Tree{
		Header: header,
		leaves: make(map[string]*Leaf, len(names)),
		order:  make([]string, 0, len(names)),
	}
	for _, name := range names {
		l := &Leaf{
			Manual: *progress.NewManual(-1),
			name:   name,
			state:  SlotPending,
		}
		t.leaves[name] = l
		t.order = append(t.order, name)
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

// Close marks any still-running leaves as resolved and completes their progress.
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
		l.mu.Lock()
		if l.state == SlotPending || l.state == SlotRunning {
			l.state = SlotResolved
		}
		l.mu.Unlock()
		l.SetCompleted()
	}
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

// Count returns the resolved count string (e.g. "47").
func (l *Leaf) Count() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.count
}

// Note returns the resolved trailer note (e.g. "32 associated").
func (l *Leaf) Note() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.note
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

// Resolve sets the resolved count string and an optional trailer note. The leaf
// model is responsible for formatting (e.g. dim parens around the note).
func (l *Leaf) Resolve(count, note string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.count = count
	l.note = note
	l.state = SlotResolved
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
