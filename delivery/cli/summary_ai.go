package cli

import (
	"bufio"
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

func lineReader(in io.Reader) *bufio.Reader {
	if r, ok := in.(*bufio.Reader); ok {
		return r
	}
	return bufio.NewReader(in)
}
func readInputLine(in *bufio.Reader) (string, error) {
	line, err := in.ReadString('\n')
	if err != nil && (err != io.EOF || len(line) == 0) {
		if err == io.EOF {
			return "", errors.New("no input")
		}
		return "", err
	}
	return strings.TrimSpace(line), nil
}
func askYesNo(in *bufio.Reader, out io.Writer, label string, defaultYes bool) (bool, error) {
	choices := "y/N"
	if defaultYes {
		choices = "Y/n"
	}
	for {
		if _, err := fmt.Fprintf(out, "\n%s [%s]: ", label, choices); err != nil {
			return false, err
		}
		value, err := readInputLine(in)
		if err != nil {
			return false, err
		}
		switch strings.ToLower(value) {
		case "":
			return defaultYes, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		case "q":
			return false, errors.New("summary canceled")
		}
		if _, err := fmt.Fprintln(out, "Enter Yes or No."); err != nil {
			return false, err
		}
	}
}
func selectSummaryTicket(in *bufio.Reader, out io.Writer, tickets []domain.TicketDay) (string, error) {
	if _, err := fmt.Fprintln(out, "\nSelect ticket:"); err != nil {
		return "", err
	}
	for i, ticket := range tickets {
		if _, err := fmt.Fprintf(out, "  %d. %-12s %s\n", i+1, safeText(ticket.Key), dailyDuration(ticket.Day.Seconds)); err != nil {
			return "", err
		}
	}
	for {
		if _, err := fmt.Fprintf(out, "Choose ticket [1-%d], or q to cancel: ", len(tickets)); err != nil {
			return "", err
		}
		value, err := readInputLine(in)
		if err != nil {
			return "", err
		}
		if strings.EqualFold(value, "q") {
			return "", errors.New("ticket selection canceled")
		}
		n, err := strconv.Atoi(value)
		if err == nil && n >= 1 && n <= len(tickets) {
			return tickets[n-1].Key, nil
		}
		if _, err := fmt.Fprintln(out, "Enter a ticket number from the list."); err != nil {
			return "", err
		}
	}
}

func runAISummary(cmd *cobra.Command, input *bufio.Reader, reader SummaryReader, date time.Time, key string, options SummaryAIOptions) error {
	return runAIGeneration(cmd, input, reader, date, key, options, false)
}

func runAIGeneration(cmd *cobra.Command, input *bufio.Reader, reader SummaryReader, date time.Time, key string, options SummaryAIOptions, ticketOnly bool) error {
	if options.Config == nil || options.Git == nil || options.Agents == nil {
		return errors.New("AI dependencies are unavailable")
	}
	config, err := options.Config.LoadOrCreate(cmd.Context())
	if err != nil {
		return err
	}
	plain := func() error {
		if ticketOnly {
			return errors.New("Jira ticket generation cancelled")
		}
		result, err := reader.TicketSummary(cmd.Context(), date, key)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), formatSummary(result))
		return err
	}
	if config.AI.Provider == "" || !config.AI.Enabled {
		if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "\nAI agent is not configured."); err != nil {
			return err
		}
		configure, err := askYesNo(input, cmd.ErrOrStderr(), "Configure AI agent now?", true)
		if err != nil {
			return err
		}
		if !configure {
			return plain()
		}
		config.AI, err = configureAI(cmd.Context(), input, cmd.ErrOrStderr(), options.Config)
		if err != nil {
			return err
		}
	}
	language, err := selectOutputLanguage(input, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	var additionalContext string
	if !ticketOnly {
		additionalContext, err = readAdditionalContext(input, cmd.ErrOrStderr())
		if err != nil {
			return err
		}
	}
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	renderer := NewProgressRenderer(cmd.ErrOrStderr())
	progress := func(event application.ProgressEvent) {
		renderer.Render(event)
		if renderer.Err() != nil {
			cancel()
		}
	}
	artifact := "summary"
	if ticketOnly {
		artifact = "ticket"
	}
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "\nGenerating Jira %s with %s...\n\nTicket:\n  %s\n", artifact, providerLabel(config.AI.Provider), safeText(key)); err != nil {
		return err
	}
	progress(application.ProgressEvent{Type: application.ProgressStatus, Message: "Loading ticket context and recorded worklogs"})
	var value application.TicketAIContext
	if ticketOnly {
		value, err = reader.TicketDescriptionContext(ctx, key)
	} else {
		value, err = reader.TicketContext(ctx, date, key)
	}
	if err != nil {
		return err
	}
	value.OutputLanguage = language
	value.AdditionalContext = additionalContext
	value, err = application.CollectCode(ctx, value, options.Git, progress)
	if err != nil {
		return err
	}
	for _, warning := range value.Warnings {
		renderer.Warning(warning)
	}
	if err := renderer.Err(); err != nil {
		return err
	}
	worklogsOnly := false
	if len(value.Repositories) == 0 {
		if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "\nSource-code context is unavailable."); err != nil {
			return err
		}
		worklogsOnly, err = askYesNo(input, cmd.ErrOrStderr(), "Generate AI "+artifact+" using worklogs only?", false)
		if err != nil {
			return err
		}
		if !worklogsOnly {
			return plain()
		}
		if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "AI will generate the %s using available worklog context only.\n", artifact); err != nil {
			return err
		}
	}
	var result application.AIResult
	var ticket application.GeneratedTicket
	if ticketOnly {
		ticket, err = application.GenerateTicketAI(ctx, value, config.AI, options.Agents, worklogsOnly, config.DataDirectory, options.Skills, progress)
	} else {
		result, err = application.GenerateAI(ctx, value, config.AI, options.Agents, worklogsOnly, config.DataDirectory, progress)
		if err == nil {
			result.Ticket, err = application.GenerateTicketAI(ctx, value, config.AI, options.Agents, worklogsOnly, config.DataDirectory, options.Skills, progress)
		}
	}
	if renderErr := renderer.Err(); renderErr != nil {
		return renderErr
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("AI generation cancelled: %w", err)
		}
		if ticketOnly {
			return fmt.Errorf("✗ %s failed.\n\nReason:\n%s", providerLabel(config.AI.Provider), safeText(err.Error()))
		}
		return fmt.Errorf("✗ %s failed.\n\nReason:\n%s\nSelect No on the AI prompt to generate the recorded worklog summary.", providerLabel(config.AI.Provider), safeText(err.Error()))
	}
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "\n✓ Jira %s generated.\n", artifact); err != nil {
		return err
	}
	if ticketOnly {
		_, err = fmt.Fprint(cmd.OutOrStdout(), ticket.Content)
		return err
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), formatAISummary(result))
	return err
}

// Assert the CLI consumes the complete application reader contract.
var _ SummaryReader = (application.TicketReader)(nil)
