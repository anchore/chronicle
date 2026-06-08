package bus

import (
	"github.com/wagoodman/go-partybus"
	"github.com/wagoodman/go-progress"

	"github.com/anchore/chronicle/chronicle/event"
)

// PublishTask emits a TaskType event with a StagedProgressable payload tracking a single in-flight task.
// total of -1 yields an indeterminate progress (stage strings drive the UI in that case).
func PublishTask(titles event.Title, context string, total int) *event.ManualStagedProgress {
	prog := event.ManualStagedProgress{
		Manual: *progress.NewManual(int64(total)),
	}

	publish(partybus.Event{
		Type: event.TaskType,
		Source: event.Task{
			Title:   titles,
			Context: context,
		},
		Value: progress.StagedProgressable(&struct {
			progress.Stager
			progress.Progressable
		}{
			Stager:       &prog.Stage,
			Progressable: &prog.Manual,
		}),
	})

	return &prog
}

// PublishGroup emits a GroupTaskType event for a bracket group of slots. The
// returned *event.Group is always non-nil — even when no publisher is set —
// so worker code can call Slot()/Close() unconditionally as no-ops.
func PublishGroup(header string, slots []event.GroupSlotInit) *event.Group {
	g := event.NewGroup(header, slots)
	publish(partybus.Event{
		Type:   event.GroupTaskType,
		Source: header,
		Value:  g,
	})
	return g
}

// PublishTree emits a TreeTaskType event for a flat tree group of leaves. The
// returned *event.Tree is always non-nil — even when no publisher is set —
// so worker code can call Leaf()/Close() unconditionally as no-ops.
func PublishTree(header string, names []string) *event.Tree {
	specs := make([]event.LeafSpec, len(names))
	for i, n := range names {
		specs[i] = event.LeafSpec{Name: n}
	}
	return PublishTreeSpec(header, specs)
}

// PublishTreeSpec emits a TreeTaskType event for a tree group whose leaves may
// each carry child leaves one level deep (e.g. "source sbom" → since/until).
func PublishTreeSpec(header string, specs []event.LeafSpec) *event.Tree {
	t := event.NewTreeWithChildren(header, specs)
	publish(partybus.Event{
		Type:   event.TreeTaskType,
		Source: header,
		Value:  t,
	})
	return t
}

// Notify emits a transient CLI notification (renders post-teardown to stderr).
func Notify(message string) {
	publish(partybus.Event{
		Type:  event.CLINotificationType,
		Value: message,
	})
}

// Report emits product output destined for stdout (e.g. the rendered
// changelog). Buffered by the UI and flushed post-teardown so it doesn't
// interleave with the live TUI on stderr.
func Report(report string) {
	publish(partybus.Event{
		Type:  event.CLIReportType,
		Value: report,
	})
}

// PublishSummary emits the raw figures for the post-run recap block. The UI
// renders them (alongside the range/evidence groups it already received) into
// the recap shown on stderr post-teardown. Carrying data rather than a
// pre-rendered string keeps all presentation in the UI layer.
func PublishSummary(s event.Summary) {
	publish(partybus.Event{
		Type:  event.CLISummaryType,
		Value: s,
	})
}

// Exit signals the main process loop to exit.
func Exit() {
	publish(partybus.Event{
		Type: event.CLIExitType,
	})
}
