package cli

import (
	"errors"
	"time"

	"github.com/spf13/cobra"
)

// Cobra aliases route to this same command and RunE, not another implementation.
func NewGenerateTicketCommand(open SummaryFactory, options SummaryAIOptions) *cobra.Command {
	return &cobra.Command{
		Use: "generate-ticket", Aliases: []string{"gt"}, Short: "Generate Jira ticket from recorded worklogs and code changes (alias: gt)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (resultErr error) {
			reader, close, err := open(cmd.Context())
			if err != nil {
				return err
			}
			defer func() { resultErr = errors.Join(resultErr, close()) }()
			tickets, err := reader.WeekTickets(cmd.Context())
			if err != nil {
				return err
			}
			if len(tickets) == 0 {
				return errors.New("no ticket worklogs this week; record ticket activity with wl start or wl note and retry")
			}
			input := lineReader(cmd.InOrStdin())
			key, err := selectSummaryTicket(input, cmd.ErrOrStderr(), tickets)
			if err != nil {
				return err
			}
			return runAIGeneration(cmd, input, reader, time.Time{}, key, options, true)
		},
	}
}
