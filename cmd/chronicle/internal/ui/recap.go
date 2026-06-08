package ui

import (
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/chronicle/chronicle/event"
	cliui "github.com/anchore/chronicle/cmd/chronicle/cli/ui"
)

// recap gathers the raw inputs for the post-teardown summary block as they
// arrive on the bus: the range/evidence groups and the summary figures. It
// holds data only — cli/ui owns the rendering — so the block can be painted
// once the live TUI area is gone.
type recap struct {
	groups  []*event.Group
	trees   []*event.Tree
	summary *event.Summary
}

// observe records any group/tree/summary event; all other events are ignored.
func (r *recap) observe(e partybus.Event) {
	switch e.Type {
	case event.GroupTaskType:
		if g, err := event.ParseGroupTaskType(e); err == nil {
			r.groups = append(r.groups, g)
		}
	case event.TreeTaskType:
		if t, err := event.ParseTreeTaskType(e); err == nil {
			r.trees = append(r.trees, t)
		}
	case event.CLISummaryType:
		if _, s, err := event.ParseCLISummaryType(e); err == nil {
			r.summary = &s
		}
	}
}

// render builds the recap block, or "" when no summary was published.
func (r *recap) render() string {
	if r.summary == nil {
		return ""
	}
	return cliui.RenderSummary(r.groups, r.trees, *r.summary)
}
