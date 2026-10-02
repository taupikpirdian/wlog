package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	application "github.com/taupikpirdian/wlog/application/summary"
	domain "github.com/taupikpirdian/wlog/domain/summary"
)

type SummaryReader interface {
	Week(context.Context) ([]domain.Day, error)
	Summary(context.Context, time.Time) (application.Result, error)
	Tickets(context.Context, time.Time) ([]domain.TicketDay, error)
	WeekTickets(context.Context) ([]domain.TicketDay, error)
	TicketSummary(context.Context, time.Time, string) (application.Result, error)
	TicketContext(context.Context, time.Time, string) (application.TicketAIContext, error)
	TicketDescriptionContext(context.Context, string) (application.TicketAIContext, error)
}
type SummaryFactory func(context.Context) (SummaryReader, func() error, error)

type SummaryAIOptions struct {
	Config application.AIConfigStore
	Git    application.GitService
	Agents application.AIAgentFactory
	Skills application.TicketSkillResolver
}

func NewSummaryCommand(open SummaryFactory, options ...SummaryAIOptions) *cobra.Command {
	return &cobra.Command{
		Use: "summary", Short: "Select a date this week and print a copyable worklog summary", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (resultErr error) {
			reader, close, err := open(cmd.Context())
			if err != nil {
				return err
			}
			defer func() {
				resultErr = errors.Join(resultErr, close())
				if resultErr == nil {
					// Keep the copyable Jira output on stdout; the terminal-only
					// footer follows every successful flow, including AI fallbacks.
					_, resultErr = fmt.Fprint(cmd.ErrOrStderr(), "\nIf wlog is useful for your workflow, consider giving it a ⭐ on GitHub:\nhttps://github.com/taupikpirdian/wlog\n")
				}
			}()
			days, err := reader.Week(cmd.Context())
			if err != nil {
				return err
			}
			input := lineReader(cmd.InOrStdin())
			index, err := selectSummaryDate(input, cmd.ErrOrStderr(), days)
			if err != nil {
				return err
			}
			date := days[index].Date
			tickets, err := reader.Tickets(cmd.Context(), date)
			if err != nil {
				return err
			}
			// Empty dates and unassigned-only evidence retain the existing summary.
			if len(tickets) == 0 {
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "No ticket worklog on this date."); err != nil {
					return err
				}
				result, err := reader.Summary(cmd.Context(), date)
				if err != nil {
					return err
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), formatSummary(result))
				return err
			}
			key, err := selectSummaryTicket(input, cmd.ErrOrStderr(), tickets)
			if err != nil {
				return err
			}
			useAI, err := askYesNo(input, cmd.ErrOrStderr(), "Generate summary with AI?", false)
			if err != nil {
				return err
			}
			if useAI {
				if len(options) == 0 {
					return errors.New("AI services are unavailable; run wl config ai with an installed wl binary")
				}
				return runAISummary(cmd, input, reader, date, key, options[0])
			}
			result, err := reader.TicketSummary(cmd.Context(), date, key)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), formatSummary(result))
			return err
		},
	}
}

// Prompts go to stderr so stdout contains only the copyable summary.
func selectSummaryDate(in io.Reader, out io.Writer, days []domain.Day) (int, error) {
	if len(days) == 0 {
		return 0, errors.New("no dates available")
	}
	if _, err := fmt.Fprint(out, "Select worklog date:\n\n"); err != nil {
		return 0, err
	}
	for i, day := range days {
		duration := "No worklog"
		if day.HasWorklog {
			duration = dailyDuration(day.Seconds)
		}
		if _, err := fmt.Fprintf(out, "  %d. %s    %s\n", i+1, day.Date.Format("Mon, 02 Jan 2006"), duration); err != nil {
			return 0, err
		}
	}
	inputReader := lineReader(in)
	for {
		if _, err := fmt.Fprintf(out, "\nChoose date [1-%d], or q to cancel: ", len(days)); err != nil {
			return 0, err
		}
		input, err := readInputLine(inputReader)
		if err != nil {
			return 0, fmt.Errorf("date selection canceled: %w", err)
		}
		if strings.EqualFold(input, "q") {
			return 0, errors.New("date selection canceled")
		}
		n, err := strconv.Atoi(input)
		if err == nil && n >= 1 && n <= len(days) {
			if _, err := fmt.Fprintln(out); err != nil {
				return 0, err
			}
			return n - 1, nil
		}
		if _, err := fmt.Fprintf(out, "Enter a number from 1 to %d.\n", len(days)); err != nil {
			return 0, err
		}
	}
}
