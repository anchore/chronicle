package event

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/go-partybus"
	"github.com/wagoodman/go-progress"
)

func TestParseTaskType(t *testing.T) {
	prog := &ManualStagedProgress{}

	tests := []struct {
		name      string
		event     partybus.Event
		wantTitle string
		wantErr   require.ErrorAssertionFunc
	}{
		{
			name: "valid task event",
			event: partybus.Event{
				Type:   TaskType,
				Source: Task{Title: Title{Default: "doing thing"}, Context: "ctx"},
				Value: progress.StagedProgressable(&struct {
					progress.Stager
					progress.Progressable
				}{
					Stager:       &prog.Stage,
					Progressable: &prog.Manual,
				}),
			},
			wantTitle: "doing thing",
		},
		{
			name:    "wrong type",
			event:   partybus.Event{Type: GroupTaskType},
			wantErr: require.Error,
		},
		{
			name: "bad source",
			event: partybus.Event{
				Type:   TaskType,
				Source: "not-a-task",
			},
			wantErr: require.Error,
		},
		{
			name: "bad value",
			event: partybus.Event{
				Type:   TaskType,
				Source: Task{Title: Title{Default: "x"}},
				Value:  "nope",
			},
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			task, p, err := ParseTaskType(tt.event)
			tt.wantErr(t, err)
			if err != nil {
				return
			}
			require.NotNil(t, task)
			require.NotNil(t, p)
			require.Equal(t, tt.wantTitle, task.Title.Default)
		})
	}
}

func TestParseGroupTaskType(t *testing.T) {
	g := NewGroup("range", []GroupSlotInit{{Name: "since", Label: "since"}})

	tests := []struct {
		name    string
		event   partybus.Event
		wantHdr string
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:    "valid group event",
			event:   partybus.Event{Type: GroupTaskType, Value: g},
			wantHdr: "range",
		},
		{
			name:    "wrong type",
			event:   partybus.Event{Type: TaskType, Value: g},
			wantErr: require.Error,
		},
		{
			name:    "bad value",
			event:   partybus.Event{Type: GroupTaskType, Value: "nope"},
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			got, err := ParseGroupTaskType(tt.event)
			tt.wantErr(t, err)
			if err != nil {
				return
			}
			require.NotNil(t, got)
			require.Equal(t, tt.wantHdr, got.Header)
		})
	}
}

func TestParseTreeTaskType(t *testing.T) {
	tr := NewTree("evidence", []string{"commits"})

	tests := []struct {
		name    string
		event   partybus.Event
		wantHdr string
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:    "valid tree event",
			event:   partybus.Event{Type: TreeTaskType, Value: tr},
			wantHdr: "evidence",
		},
		{
			name:    "wrong type",
			event:   partybus.Event{Type: TaskType, Value: tr},
			wantErr: require.Error,
		},
		{
			name:    "bad value",
			event:   partybus.Event{Type: TreeTaskType, Value: 42},
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			got, err := ParseTreeTaskType(tt.event)
			tt.wantErr(t, err)
			if err != nil {
				return
			}
			require.NotNil(t, got)
			require.Equal(t, tt.wantHdr, got.Header)
		})
	}
}

func TestParseCLIReportType(t *testing.T) {
	tests := []struct {
		name        string
		event       partybus.Event
		wantContext string
		wantReport  string
		wantErr     require.ErrorAssertionFunc
	}{
		{
			name:        "valid report",
			event:       partybus.Event{Type: CLIReportType, Source: "ctx", Value: "the report"},
			wantContext: "ctx",
			wantReport:  "the report",
		},
		{
			name:       "missing source becomes empty context",
			event:      partybus.Event{Type: CLIReportType, Value: "the report"},
			wantReport: "the report",
		},
		{
			name:    "wrong type",
			event:   partybus.Event{Type: TaskType, Value: "nope"},
			wantErr: require.Error,
		},
		{
			name:    "bad value",
			event:   partybus.Event{Type: CLIReportType, Value: 42},
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			ctx, rep, err := ParseCLIReportType(tt.event)
			tt.wantErr(t, err)
			if err != nil {
				return
			}
			require.Equal(t, tt.wantContext, ctx)
			require.Equal(t, tt.wantReport, rep)
		})
	}
}

func TestParseCLINotificationType(t *testing.T) {
	tests := []struct {
		name        string
		event       partybus.Event
		wantContext string
		wantNotif   string
		wantErr     require.ErrorAssertionFunc
	}{
		{
			name:        "valid notification",
			event:       partybus.Event{Type: CLINotificationType, Source: "ctx", Value: "msg"},
			wantContext: "ctx",
			wantNotif:   "msg",
		},
		{
			name:      "missing source becomes empty context",
			event:     partybus.Event{Type: CLINotificationType, Value: "msg"},
			wantNotif: "msg",
		},
		{
			name:    "wrong type",
			event:   partybus.Event{Type: TaskType, Value: "msg"},
			wantErr: require.Error,
		},
		{
			name:    "bad value",
			event:   partybus.Event{Type: CLINotificationType, Value: 42},
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			ctx, notif, err := ParseCLINotificationType(tt.event)
			tt.wantErr(t, err)
			if err != nil {
				return
			}
			require.Equal(t, tt.wantContext, ctx)
			require.Equal(t, tt.wantNotif, notif)
		})
	}
}

func TestNilSafety(t *testing.T) {
	// nil group methods do not panic
	var g *Group
	require.Nil(t, g.Slot("missing"))
	require.Nil(t, g.Names())
	require.NotPanics(t, func() { g.Close() })

	// nil slot methods do not panic
	var s *Slot
	require.NotPanics(t, func() { s.Resolve("v1") })
	require.NotPanics(t, func() { s.Fail(nil) })
	require.NotPanics(t, func() { s.Start() })
	require.NotPanics(t, func() { s.SetStage("x") })
	require.Equal(t, "", s.Name())
	require.Equal(t, "", s.Label())
	require.Equal(t, "", s.Intent())
	require.Equal(t, SlotPending, s.State())
	require.Nil(t, s.Values())
	require.Nil(t, s.Err())

	// nil tree methods do not panic
	var tr *Tree
	require.Nil(t, tr.Leaf("missing"))
	require.Nil(t, tr.Names())
	require.NotPanics(t, func() { tr.Close() })

	// nil leaf methods do not panic
	var l *Leaf
	require.NotPanics(t, func() { l.Resolve("1", "n") })
	require.NotPanics(t, func() { l.Fail(nil) })
	require.NotPanics(t, func() { l.Start() })
	require.NotPanics(t, func() { l.SetStage("x") })
	require.Equal(t, "", l.Name())
	require.Equal(t, "", l.Count())
	require.Equal(t, "", l.Note())
	require.Equal(t, SlotPending, l.State())
	require.Nil(t, l.Err())
}
