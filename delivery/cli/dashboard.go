package cli

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	domain "github.com/taupikpirdian/wlog/domain/dashboard"
)

type DashboardReader interface {
	Read(context.Context) (domain.View, error)
}
type DashboardFactory func(context.Context) (DashboardReader, func() error, error)

func runDaily(open DashboardFactory, timeline bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) (resultErr error) {
		reader, close, err := open(cmd.Context())
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, close()) }()
		value, err := reader.Read(cmd.Context())
		if err != nil {
			return err
		}
		var output string
		if timeline {
			output = renderTimeline(value)
		} else {
			output = renderDashboard(value)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), output)
		return err
	}
}

func addTodayCommand(root *cobra.Command, open DashboardFactory) {
	root.AddCommand(&cobra.Command{Use: "today", Short: "Show today's chronological work timeline", Args: cobra.NoArgs, RunE: runDaily(open, true)})
}

func renderDashboard(v domain.View) string {
	var b strings.Builder
	b.WriteString("DEV WORKLOG\n\n")
	if v.Active == nil {
		b.WriteString("No active session\n")
	} else {
		active := v.Active
		layout := "15:04"
		if active.Session.StartedAt.In(v.Location).Format("2006-01-02") != v.Start.Format("2006-01-02") {
			layout = "2006-01-02 15:04"
		}
		fmt.Fprintf(&b, "Active Session\n%s — %s\n\nStarted : %s\nDuration: %s\n", safeText(active.Session.TicketKey), safeText(active.Session.Title), active.Session.StartedAt.In(v.Location).Format(layout), dailyDuration(active.ElapsedSeconds))
		repository := ""
		if active.Session.Repository != nil {
			repository = *active.Session.Repository
		}
		fmt.Fprintf(&b, "Repo    : %s\n", repositoryName(repository))
	}
	b.WriteString("\nToday\n─────────────────────────────\n")
	if len(v.Tickets) == 0 {
		b.WriteString("No tracked sessions today\n")
	}
	for _, ticket := range v.Tickets {
		title := strings.TrimSpace(safeText(ticket.Title))
		if title == "" {
			title = "-"
		}
		fmt.Fprintf(&b, "%-12s %s", safeText(ticket.Key), dailyDuration(ticket.Seconds))
		if ticket.CommitCount > 0 {
			label := "commits"
			if ticket.CommitCount == 1 {
				label = "commit"
			}
			fmt.Fprintf(&b, " (%d %s)", ticket.CommitCount, label)
		}
		fmt.Fprintf(&b, "  [repo: %s] — %s\n", repositoryNames(ticket.Repositories), title)
	}
	fmt.Fprintf(&b, "Total        %s\n", dailyDuration(v.TotalSeconds))
	if len(v.Unassigned) > 0 {
		b.WriteString("\nUnassigned\n")
		for _, event := range v.Unassigned {
			writeEvent(&b, event, v)
		}
	}
	return b.String()
}

func renderTimeline(v domain.View) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n─────────────────────────────\n", v.Start.Format("02 Jan 2006"))
	if len(v.Events) == 0 {
		b.WriteString("No activities today\n")
	}
	for _, event := range v.Events {
		writeEvent(&b, event, v)
	}
	fmt.Fprintf(&b, "\nTotal tracked: %s\n", dailyDuration(v.TotalSeconds))
	return b.String()
}

func writeEvent(b *strings.Builder, event domain.Event, v domain.View) {
	key := event.TicketKey
	if key == "" {
		key = "UNASSIGNED"
	}
	fmt.Fprintf(b, "%s  %-6s  %s", event.At.In(v.Location).Format("15:04"), event.Kind, safeText(key))
	if event.Text != "" {
		fmt.Fprintf(b, "  %s", safeText(event.Text))
	}
	if event.Kind == "COMMIT" && event.Hash != "" {
		hash := []rune(safeText(event.Hash))
		if len(hash) > 7 {
			hash = hash[:7]
		}
		fmt.Fprintf(b, " [%s]", string(hash))
	}
	fmt.Fprintf(b, " [repo: %s]\n", repositoryName(event.Repository))
}

func repositoryName(repository string) string {
	// Stored paths may originate from Windows even when viewed on macOS/Linux.
	name := path.Base(strings.TrimRight(strings.ReplaceAll(repository, "\\", "/"), "/"))
	if name == "." || name == "" {
		return "-"
	}
	return safeText(name)
}

func repositoryNames(repositories []string) string {
	if len(repositories) == 0 {
		return "-"
	}
	seen := map[string]bool{}
	var names []string
	for _, repository := range repositories {
		name := repositoryName(repository)
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func dailyDuration(seconds int64) string {
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	if minutes%60 == 0 {
		return fmt.Sprintf("%dh", minutes/60)
	}
	return fmt.Sprintf("%dh %dm", minutes/60, minutes%60)
}

// safeText consumes terminal escape sequences before stripping control runes.
// It handles CSI/OSC/DCS as well as C1 equivalents and never emits ESC or BEL.
func safeText(value string) string {
	var b strings.Builder
	runes := []rune(value)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\x1b' || r == '\u009b' || r == '\u009d' || r == '\u0090' {
			kind := r
			if r == '\x1b' {
				i++
				if i == len(runes) {
					break
				}
				kind = runes[i]
			}
			switch kind {
			case '[', '\u009b':
				for i++; i < len(runes); i++ {
					if runes[i] >= 0x40 && runes[i] <= 0x7e {
						break
					}
				}
			case ']', 'P', 'X', '^', '_', '\u009d', '\u0090':
				for i++; i < len(runes); i++ {
					if runes[i] == '\a' || runes[i] == '\u009c' {
						break
					}
					if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '\\' {
						i++
						break
					}
				}
			default:
				// Generic escape: zero or more intermediate bytes, then a final byte.
				for i < len(runes) && runes[i] >= 0x20 && runes[i] <= 0x2f {
					i++
				}
			}
			continue
		}
		if unicode.IsSpace(r) {
			b.WriteByte(' ')
		} else if !unicode.IsControl(r) && !unicode.Is(unicode.Cf, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
