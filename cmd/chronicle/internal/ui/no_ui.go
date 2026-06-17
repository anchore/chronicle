package ui

import (
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/chronicle/chronicle/event"
	"github.com/anchore/clio"
)

var _ clio.UI = (*NoUI)(nil)

type NoUI struct {
	finalizeEvents []partybus.Event
	subscription   partybus.Unsubscribable
	quiet          bool
	recap          recap
}

func None() *NoUI {
	return &NoUI{}
}

func (n *NoUI) Setup(subscription partybus.Unsubscribable) error {
	n.subscription = subscription
	return nil
}

func (n *NoUI) Handle(e partybus.Event) error {
	// collect the raw groups/trees/summary figures for the recap block; with no
	// live TUI, the recap is the only place this info surfaces.
	n.recap.observe(e)

	switch e.Type {
	case event.CLIReportType, event.CLINotificationType:
		// keep these for when the UI is terminated to show to the screen (or perform other events)
		n.finalizeEvents = append(n.finalizeEvents, e)
	case event.CLIExitType:
		return n.subscription.Unsubscribe()
	}
	return nil
}

func (n NoUI) Teardown(_ bool) error {
	postUIEvents(n.quiet, n.recap.render(), n.finalizeEvents...)
	return nil
}
