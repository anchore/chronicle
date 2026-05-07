package event

import (
	"sync"

	"github.com/wagoodman/go-progress"
)

// SlotState describes the visual state of a Slot in a bracket Group.
type SlotState int

const (
	SlotPending SlotState = iota
	SlotRunning
	SlotResolved
	SlotFailed
)

// GroupSlotInit is the configuration for one slot at group-construction time.
type GroupSlotInit struct {
	Name   string // internal id ("since")
	Label  string // user-visible left label ("since")
	Intent string // dim auxiliary ("latest release")
}

// Slot is a single line within a bracket Group. It is concurrency-safe and
// satisfies progress.StagedProgressable so bubbly's taskprogress can consume it.
type Slot struct {
	progress.Stage
	progress.Manual

	mu     sync.Mutex
	name   string
	label  string
	intent string
	state  SlotState
	values []string // raw value segments (e.g. "v0.14.0", "a3b4c5d", "Jan 15 2026")
	err    error
}

// Group is a bracket-style group of slots (e.g. "range" with since/until).
// Methods are nil-safe so worker code can be written without scaffolding when
// no publisher is attached.
type Group struct {
	Header string

	mu    sync.Mutex
	order []string
	slots map[string]*Slot
}

// NewGroup constructs a Group with its declared slots in order.
func NewGroup(header string, inits []GroupSlotInit) *Group {
	g := &Group{
		Header: header,
		slots:  make(map[string]*Slot, len(inits)),
		order:  make([]string, 0, len(inits)),
	}
	for _, init := range inits {
		s := &Slot{
			Manual: *progress.NewManual(-1),
			name:   init.Name,
			label:  init.Label,
			intent: init.Intent,
			state:  SlotPending,
		}
		g.slots[init.Name] = s
		g.order = append(g.order, init.Name)
	}
	return g
}

// Slot returns the named slot. Returns nil if the group is nil or the name is
// missing — callers may still chain method calls because Slot methods are
// nil-safe.
func (g *Group) Slot(name string) *Slot {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.slots[name]
}

// Names returns the slot names in declared order.
func (g *Group) Names() []string {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]string, len(g.order))
	copy(out, g.order)
	return out
}

// Close marks any still-running slots as resolved and completes the group's progress.
func (g *Group) Close() {
	if g == nil {
		return
	}
	g.mu.Lock()
	slots := make([]*Slot, 0, len(g.slots))
	for _, s := range g.slots {
		slots = append(slots, s)
	}
	g.mu.Unlock()
	for _, s := range slots {
		s.mu.Lock()
		if s.state == SlotPending || s.state == SlotRunning {
			s.state = SlotResolved
		}
		s.mu.Unlock()
		s.SetCompleted()
	}
}

// Name returns the slot's internal name.
func (s *Slot) Name() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

// Label returns the slot's user-visible left label.
func (s *Slot) Label() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.label
}

// Intent returns the slot's dim auxiliary text (e.g. "latest release").
func (s *Slot) Intent() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.intent
}

// State returns the current slot state.
func (s *Slot) State() SlotState {
	if s == nil {
		return SlotPending
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Values returns the raw resolved value segments. The UI model owns formatting.
func (s *Slot) Values() []string {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.values))
	copy(out, s.values)
	return out
}

// Err returns the failure error, if any.
func (s *Slot) Err() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Start flips the slot to the running state. Optional — Resolve/Fail will also
// transition out of pending implicitly.
func (s *Slot) Start() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.state == SlotPending {
		s.state = SlotRunning
	}
	s.mu.Unlock()
}

// Resolve sets the bright resolved value as raw segments (e.g. tag, sha, date).
// The slot model is responsible for join/format/styling.
func (s *Slot) Resolve(values ...string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.values = append(s.values[:0], values...)
	s.state = SlotResolved
	s.mu.Unlock()
	s.SetCompleted()
}

// Fail flips the slot to the failed state with the given error.
func (s *Slot) Fail(err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.err = err
	s.state = SlotFailed
	s.mu.Unlock()
	s.SetError(err)
}

// SetStage updates the mid-resolution detail string (proxy to Stage.Current).
func (s *Slot) SetStage(detail string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.Stage.Current = detail
	if s.state == SlotPending {
		s.state = SlotRunning
	}
	s.mu.Unlock()
}
