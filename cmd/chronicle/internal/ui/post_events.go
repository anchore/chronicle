package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/chronicle/chronicle/event"
	"github.com/anchore/chronicle/internal/log"
)

// postUIEvents flushes accumulated finalize events after the UI has torn down:
//   - CLISummaryType  → stderr (the post-run recap block)
//   - CLINotificationType → stderr in magenta (transient status)
//   - CLIReportType   → stdout (the changelog product; bytes the user piped)
//
// Order matters: in TTY mode where stdout and stderr land in the same
// terminal, we want the recap block + status notes (stderr) to appear above
// the bulk product output (stdout). This matches the reading order users
// expect — "here's what happened" first, then the artifact.
//
// Routing reports through this function (rather than letting the worker
// write directly to os.Stdout) is what keeps stdout clean while bubbletea
// is alive on stderr.
func postUIEvents(quiet bool, events ...partybus.Event) {
	writeSummaries(os.Stderr, events...)

	if !quiet {
		writeNotifications(os.Stderr, events...)
	}

	writeReports(os.Stdout, events...)
}

func writeReports(w *os.File, events ...partybus.Event) {
	var reports []string
	for _, e := range events {
		if e.Type != event.CLIReportType {
			continue
		}
		source, report, err := event.ParseCLIReportType(e)
		if err != nil {
			log.WithFields("error", err).
				Warnf("failed to gather final report for %q", source)
			continue
		}
		reports = append(reports, strings.TrimRight(report, "\n ")+"\n")
	}
	if len(reports) == 0 {
		return
	}
	_, _ = fmt.Fprint(w, strings.Join(reports, "\n"))
}

func writeSummaries(w *os.File, events ...partybus.Event) {
	var blocks []string
	for _, e := range events {
		if e.Type != event.CLISummaryType {
			continue
		}
		source, summary, err := event.ParseCLISummaryType(e)
		if err != nil {
			log.WithFields("error", err).
				Warnf("failed to gather summary for %q", source)
			continue
		}
		blocks = append(blocks, strings.TrimRight(summary, "\n ")+"\n")
	}
	if len(blocks) == 0 {
		return
	}
	// trailing blank line separates the summary block from whatever comes
	// next on the same stream (the changelog product on stdout in TTY mode).
	_, _ = fmt.Fprint(w, strings.Join(blocks, "\n")+"\n")
}

func writeNotifications(w *os.File, events ...partybus.Event) {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	for _, e := range events {
		if e.Type != event.CLINotificationType {
			continue
		}
		source, notification, err := event.ParseCLINotificationType(e)
		if err != nil {
			log.WithFields("error", err).
				Warnf("failed to gather notification for %q", source)
			continue
		}
		_, _ = fmt.Fprintln(w, style.Render(notification))
	}
}
