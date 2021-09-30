package bus

import "github.com/wagoodman/go-partybus"

var publisher partybus.Publisher
var active bool

// SetPublisher sets the singleton event bus publisher. This is optional; if no bus is provided, the library will
// behave no differently than if a bus had been provided.
func SetPublisher(p partybus.Publisher) {
	publisher = p
	if p != nil {
		active = true
	}
}

// Publish an event onto the bus. If there is no bus set by the calling application, this does nothing.
func Publish(event partybus.Event) {
	if active {
		publisher.Publish(event)
	}
}
