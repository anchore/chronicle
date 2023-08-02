package chronicle

import (
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/chronicle/internal/bus"
	"github.com/anchore/chronicle/internal/log"
	"github.com/anchore/go-logger"
)

// SetLogger sets the logger object used for all logging calls.
func SetLogger(logger logger.Logger) {
	log.Set(logger)
}

// SetBus sets the event bus for all published events onto (in-library subscriptions are not allowed).
func SetBus(b *partybus.Bus) {
	bus.SetPublisher(b)
}
