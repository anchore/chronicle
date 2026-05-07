package bus

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/chronicle/chronicle/event"
)

// channelPublisher is a minimal partybus.Publisher stub backed by a buffered channel.
type channelPublisher struct {
	events chan partybus.Event
}

func newChannelPublisher(buf int) *channelPublisher {
	return &channelPublisher{events: make(chan partybus.Event, buf)}
}

func (c *channelPublisher) Publish(e partybus.Event) {
	c.events <- e
}

// drain returns all currently-buffered events without blocking.
func (c *channelPublisher) drain() []partybus.Event {
	var out []partybus.Event
	for {
		select {
		case e := <-c.events:
			out = append(out, e)
		default:
			return out
		}
	}
}

func TestHelpers_NoPublisher_NoOps(t *testing.T) {
	// pre-condition: no publisher set
	Set(nil)
	resetSummaryCache()

	// PublishGroup must return non-nil even without a publisher
	g := PublishGroup("range", []event.GroupSlotInit{
		{Name: "since", Label: "since", Intent: "latest"},
		{Name: "until", Label: "until", Intent: "HEAD"},
	})
	require.NotNil(t, g)
	require.NotPanics(t, func() {
		g.Slot("since").Resolve("v0.1.0", "abc1234")
		g.Slot("until").Fail(nil)
		g.Close()
	})

	// PublishTree must return non-nil even without a publisher
	tr := PublishTree("evidence", []string{"commits", "issues"})
	require.NotNil(t, tr)
	require.NotPanics(t, func() {
		tr.Leaf("commits").Resolve("47", "32 associated")
		tr.Close()
	})

	// PublishTask returns a usable progress handle
	mon := PublishTask(event.Title{Default: "task"}, "ctx", -1)
	require.NotNil(t, mon)
	mon.Stage.Current = "running"
	mon.SetCompleted()

	// these must not panic with no publisher
	require.NotPanics(t, func() {
		Notify("hi")
		Report("report")
		Exit()
	})
}

func TestHelpers_PublishesCorrectEventTypes(t *testing.T) {
	pub := newChannelPublisher(16)
	Set(pub)
	defer Set(nil)
	resetSummaryCache()

	g := PublishGroup("range", []event.GroupSlotInit{{Name: "since", Label: "since"}})
	require.NotNil(t, g)

	tr := PublishTree("evidence", []string{"commits"})
	require.NotNil(t, tr)

	mon := PublishTask(event.Title{Default: "x"}, "ctx", 5)
	require.NotNil(t, mon)

	Notify("notify-me")
	Report("report-me")
	Exit()

	events := pub.drain()
	require.Len(t, events, 6)

	// preserved publish order
	require.Equal(t, event.GroupTaskType, events[0].Type)
	require.Equal(t, "range", events[0].Source)
	require.Equal(t, g, events[0].Value)

	require.Equal(t, event.TreeTaskType, events[1].Type)
	require.Equal(t, "evidence", events[1].Source)
	require.Equal(t, tr, events[1].Value)

	require.Equal(t, event.TaskType, events[2].Type)
	srcTask, ok := events[2].Source.(event.Task)
	require.True(t, ok)
	require.Equal(t, "x", srcTask.Title.Default)

	require.Equal(t, event.CLINotificationType, events[3].Type)
	require.Equal(t, "notify-me", events[3].Value)

	require.Equal(t, event.CLIReportType, events[4].Type)
	require.Equal(t, "report-me", events[4].Value)

	require.Equal(t, event.CLIExitType, events[5].Type)
}

func TestGet(t *testing.T) {
	Set(nil)
	require.Nil(t, Get())
	pub := newChannelPublisher(1)
	Set(pub)
	require.Equal(t, pub, Get())
	Set(nil)
}
