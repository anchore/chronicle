/*
Package event provides event types for all events that the library published onto the event bus. By convention, for
each event defined here there should be a corresponding event parser defined in this package.
*/
package event

import "github.com/wagoodman/go-partybus"

const (
	typePrefix    = "chronicle"
	cliTypePrefix = typePrefix + "-cli"

	// TaskType is a partybus event for a single in-flight task with a stage label and progress count.
	TaskType partybus.EventType = typePrefix + "-task"

	// GroupTaskType is a partybus event for a bracket group of slots (e.g. since/until range).
	GroupTaskType partybus.EventType = typePrefix + "-group-task"

	// TreeTaskType is a partybus event for a tree group of leaves (e.g. evidence: commits/issues/PRs).
	TreeTaskType partybus.EventType = typePrefix + "-tree-task"

	// CLIExitType is a partybus event indicating the main process is to exit.
	CLIExitType partybus.EventType = cliTypePrefix + "-exit-event"

	// CLIReportType carries product output destined for stdout (e.g. the rendered
	// changelog). The UI buffers these until after teardown so they don't
	// interleave with the live TUI on stderr.
	CLIReportType partybus.EventType = cliTypePrefix + "-report"

	// CLISummaryType carries the post-run recap block destined for stderr
	// (range / evidence / changes-by-type / version transition). Distinct from
	// CLIReportType so it can be routed to a different stream without running
	// through the magenta notification style.
	CLISummaryType partybus.EventType = cliTypePrefix + "-summary"

	// CLINotificationType carries short status messages for stderr, rendered
	// in magenta post-teardown.
	CLINotificationType partybus.EventType = cliTypePrefix + "-notification"
)
