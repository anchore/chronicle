package ui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/bubbly/bubbles/frame"
	"github.com/anchore/chronicle/chronicle/event"
	handler "github.com/anchore/chronicle/cmd/chronicle/cli/ui"
	"github.com/anchore/chronicle/internal/bus"
	"github.com/anchore/chronicle/internal/log"
	"github.com/anchore/clio"
	"github.com/anchore/go-logger"
)

var _ interface {
	tea.Model
	partybus.Responder
	clio.UI
} = (*UI)(nil)

// UI is the bubbletea-based clio.UI. The "Project: OWNER/REPO" title is
// rendered by the range bracketGroup itself (handler reads it from
// bus.Repo()), so the UI doesn't carry a separate header model.
type UI struct {
	program        *tea.Program
	running        *sync.WaitGroup
	quiet          bool
	subscription   partybus.Unsubscribable
	finalizeEvents []partybus.Event

	version string
	repo    string

	handler *handler.Handler
	frame   tea.Model
}

func New(version, repo string, _, quiet bool) *UI {
	h := handler.New()
	return &UI{
		handler: h,
		frame:   frame.New(),
		running: &sync.WaitGroup{},
		quiet:   quiet,
		version: version,
		repo:    repo,
	}
}

func (m *UI) Setup(subscription partybus.Unsubscribable) error {
	// redirect log output into the frame footer so log lines appear under the
	// frame instead of garbling it.
	if logWrapper, ok := log.Get().(logger.Controller); ok {
		logWrapper.SetOutput(m.frame.(*frame.Frame).Footer())
	}

	m.subscription = subscription
	// WithoutSignalHandler: let cobra/clio own SIGINT/SIGTERM so Ctrl+C tears
	// down through the normal bus.Exit() path instead of bubbletea's internal
	// handler (which can leave the terminal in a weird state on cancellation).
	m.program = tea.NewProgram(m,
		tea.WithOutput(os.Stderr),
		tea.WithInput(os.Stdin),
		tea.WithoutSignalHandler(),
	)
	m.running.Add(1)

	go func() {
		defer m.running.Done()
		if _, err := m.program.Run(); err != nil {
			log.Errorf("unable to start UI: %+v", err)
			m.exit()
		}
	}()

	return nil
}

func (m *UI) exit() {
	bus.Exit()
}

func (m *UI) Handle(e partybus.Event) error {
	if m.program != nil {
		m.program.Send(e)
		if e.Type == event.CLIExitType {
			return m.subscription.Unsubscribe()
		}
	}
	return nil
}

// Teardown waits for the bubbletea program to finish. The program is already
// on its way out: receiving the application exit event in Update queued the
// expire→render→quit sequence, so we just join the goroutine here.
func (m *UI) Teardown(force bool) error {
	defer func() {
		// allow traditional logging to resume now that the UI is shutting down.
		if logWrapper, ok := log.Get().(logger.Controller); ok {
			logWrapper.SetOutput(os.Stderr)
		}
	}()

	if !force {
		m.running.Wait()
	} else {
		// hard stop: tell bubbletea to exit immediately, then wait briefly.
		m.program.Quit()
		_ = runWithTimeout(250*time.Millisecond, func() error {
			m.running.Wait()
			return nil
		})
	}

	postUIEvents(m.quiet, m.finalizeEvents...)
	return nil
}

// bubbletea.Model functions

func (m *UI) Init() tea.Cmd {
	// kick off the singleton spinner. All bracket-group slots and tree-group
	// leaves render its current frame at View time, so this is the only Tick
	// command we ever need.
	cmds := []tea.Cmd{m.frame.Init()}
	if sp := m.handler.State().Spinner; sp != nil {
		cmds = append(cmds, sp.Tick)
	}
	return tea.Batch(cmds...)
}

func (m *UI) RespondsTo() []partybus.EventType {
	return append([]partybus.EventType{
		event.CLIReportType,
		event.CLISummaryType,
		event.CLINotificationType,
		event.CLIExitType,
	}, m.handler.RespondsTo()...)
}

func (m *UI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// pointer receiver so the same UI instance owns finalizeEvents through
	// teardown.
	var cmds []tea.Cmd

	// singleton spinner: advance the shared instance from one place so all
	// bracket-group slots and tree-group leaves render the same frame.
	if tickMsg, ok := msg.(spinner.TickMsg); ok {
		if sp := m.handler.State().Spinner; sp != nil {
			next, cmd := sp.Update(tickMsg)
			*sp = next
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.exit()
			return m, tea.Quit
		}

	case partybus.Event:
		switch msg.Type {
		case event.CLIReportType, event.CLISummaryType, event.CLINotificationType:
			// stash for post-teardown emission; they don't drive the frame.
			m.finalizeEvents = append(m.finalizeEvents, msg)
			return m, nil

		case event.CLIExitType, clio.ExitEventType:
			// shutdown sequence:
			//   1. ExpireGroupsMsg flips groups' expired flag (View still
			//      renders them on this frame, capturing their resolved state)
			//   2. one frame budget (~16ms at 60fps default) for the renderer
			//      to actually paint that resolved frame, then PruneTickMsg
			//      drives a frame.Update where IsAlive=false prunes the
			//      groups and the renderer paints the now-shorter view —
			//      bubbletea emits cursor-up + erase-line escapes that the
			//      terminal uses to remove the resolved-state lines
			//   3. tea.Quit exits the program; the empty live area means
			//      postUIEvents writes the summary block onto a clean stretch
			//      of terminal below where the TUI used to be.
			return m, tea.Sequence(
				func() tea.Msg { return handler.ExpireGroupsMsg{} },
				tea.Tick(20*time.Millisecond, func(time.Time) tea.Msg { return handler.PruneTickMsg{} }),
				tea.Quit,
			)
		}

		newModels, cmd := m.handler.Handle(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		for _, newModel := range newModels {
			if newModel == nil {
				continue
			}
			cmds = append(cmds, newModel.Init())
			f := m.frame.(*frame.Frame)
			f.AppendModel(newModel)
			m.frame = f
		}
		// fall through to update the frame model
	}

	frameModel, cmd := m.frame.Update(msg)
	m.frame = frameModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *UI) View() string {
	return m.frame.View()
}

func runWithTimeout(timeout time.Duration, fn func() error) (err error) {
	c := make(chan struct{}, 1)
	go func() {
		err = fn()
		c <- struct{}{}
	}()
	select {
	case <-c:
	case <-time.After(timeout):
		return fmt.Errorf("timed out after %v", timeout)
	}
	return err
}
