package ui

import (
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/go-partybus"
	"github.com/wagoodman/go-progress"

	"github.com/anchore/bubbly"
	"github.com/anchore/bubbly/bubbles/taskprogress"
	"github.com/anchore/chronicle/chronicle/event"
	"github.com/anchore/chronicle/internal/bus"
	"github.com/anchore/chronicle/internal/log"
	syftEvent "github.com/anchore/syft/syft/event"
	"github.com/anchore/syft/syft/event/monitor"
)

var _ bubbly.EventHandler = (*Handler)(nil)

// Handler is chronicle's bubbly.EventHandler — translates partybus events into
// bubbletea models that the UI frame appends.
type Handler struct {
	state *State
	bubbly.EventHandler
}

type State struct {
	WindowSize tea.WindowSizeMsg
	Running    *sync.WaitGroup

	// Spinner is the singleton spinner.Model whose frame all bracket-group
	// slots and tree-group leaves render. The top-level UI is the sole source
	// of TickMsg advancement so all spinners stay in lock-step. Pointer so the
	// child models can pull the latest View() each render without each
	// constructing its own ticker.
	Spinner *spinner.Model
}

func New() *Handler {
	d := bubbly.NewEventDispatcher()

	sp := newChronicleSpinner()
	h := &Handler{
		EventHandler: d,
		state: &State{
			Running: &sync.WaitGroup{},
			Spinner: &sp,
		},
	}

	d.AddHandlers(map[partybus.EventType]bubbly.EventHandlerFn{
		event.TaskType:      h.handleTask,
		event.GroupTaskType: h.handleGroupTask,
		event.TreeTaskType:  h.handleTreeTask,
		// react to syft's cataloging events ourselves (rather than rendering
		// syft's own multi-row TUI) so the live package count rolls up onto the
		// source-sbom branch leaves.
		syftEvent.CatalogerTaskStarted: h.handleSyftCatalogerTask,
	})

	return h
}

func (m *Handler) State() *State {
	return m.state
}

func (m *Handler) handleTask(e partybus.Event) ([]tea.Model, tea.Cmd) {
	cmd, prog, err := event.ParseTaskType(e)
	if err != nil {
		log.Warnf("unable to parse event: %+v", err)
		return nil, nil
	}

	return m.handleStagedProgressable(prog, taskprogress.Title{
		Default: cmd.Title.Default,
		Running: cmd.Title.WhileRunning,
		Success: cmd.Title.OnSuccess,
		Failed:  cmd.Title.OnFail,
	}, cmd.Context), nil
}

func (m *Handler) handleStagedProgressable(prog progress.StagedProgressable, title taskprogress.Title, context ...string) []tea.Model {
	tsk := taskprogress.New(
		m.state.Running,
		taskprogress.WithStagedProgressable(prog),
	)
	tsk.HideProgressOnSuccess = true
	tsk.TitleOptions = title
	tsk.Context = context
	tsk.WindowSize = m.state.WindowSize

	return []tea.Model{tsk}
}

func (m *Handler) handleGroupTask(e partybus.Event) ([]tea.Model, tea.Cmd) {
	g, err := event.ParseGroupTaskType(e)
	if err != nil {
		log.Warnf("unable to parse event: %+v", err)
		return nil, nil
	}
	// the "range" group renders as the top-level project section: its title
	// is "Project: OWNER/REPO" with the bracket pair as its tree.
	title := ""
	if g.Header == "range" {
		if repo := bus.Repo(); repo != "" {
			title = boldStyle.Render("Project: " + repo)
		}
	}
	return []tea.Model{NewBracketGroup(g, title, m.state.Spinner)}, nil
}

func (m *Handler) handleTreeTask(e partybus.Event) ([]tea.Model, tea.Cmd) {
	t, err := event.ParseTreeTaskType(e)
	if err != nil {
		log.Warnf("unable to parse event: %+v", err)
		return nil, nil
	}
	return []tea.Model{NewTreeGroup(t, m.state.Spinner)}, nil
}

// handleSyftCatalogerTask consumes syft's cataloging events and routes the live
// package count onto the matching source-sbom branch leaf, instead of letting
// syft render its own rows. The top-level "cataloging" task carries the source
// ID in its context; the "package-cataloging" task that syft publishes right
// after carries the live "N packages" stage but no source ID, so we pair the
// two via the bus's bind queue. All other cataloger sub-tasks are ignored. It
// produces no UI model of its own.
func (m *Handler) handleSyftCatalogerTask(e partybus.Event) ([]tea.Model, tea.Cmd) {
	gt, ok := e.Source.(monitor.GenericTask)
	if !ok {
		return nil, nil
	}
	switch gt.ID {
	case monitor.TopLevelCatalogingTaskID:
		bus.EnqueueSBOMBind(gt.Context)
	case monitor.PackageCatalogingTaskID:
		if p, ok := e.Value.(progress.StagedProgressable); ok {
			bus.BindSBOMPackageProgress(p)
		}
	}
	return nil, nil
}
