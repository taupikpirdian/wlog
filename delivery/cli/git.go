package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/taupikpirdian/wlog/domain/activity"
)

type GitCaptureService interface {
	Capture(context.Context) (activity.CaptureResult, error)
}
type GitFactory func(context.Context) (GitCaptureService, func() error, error)

func addGitCommand(root *cobra.Command, open GitFactory) {
	root.AddCommand(&cobra.Command{
		Use: "git", Short: "Capture the current Git commit", Args: sessionArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) (resultErr error) {
			if open == nil {
				return errors.New("Git capture service is unavailable")
			}
			service, close, err := open(cmd.Context())
			if err != nil {
				return fmt.Errorf("⚠ Worklog capture failed: %w", err)
			}
			defer func() { resultErr = errors.Join(resultErr, close()) }()
			result, err := service.Capture(cmd.Context())
			if err != nil {
				return fmt.Errorf("⚠ Worklog capture failed: %w", err)
			}
			switch {
			case result.AlreadyCaptured:
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "✓ Commit already captured.")
			case result.Attribution.TicketKey == "":
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "✓ Commit captured (UNASSIGNED).")
			case result.Attribution.SessionID == nil:
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "✓ Commit captured under %s (UNSESSIONED).\n", result.Attribution.TicketKey)
			default:
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "✓ Commit captured under %s.\n", result.Attribution.TicketKey)
			}
			if err != nil {
				return err
			}
			for _, warning := range result.Warnings {
				var message string
				switch warning {
				case "ticket_mismatch":
					message = fmt.Sprintf("⚠ Ticket mismatch\n\nActive session : %s\nCommit ticket  : %s\n\nCommit saved under %s.\n", result.Attribution.ActiveTicket, result.Attribution.TicketKey, result.Attribution.TicketKey)
				case "unassigned":
					message = "⚠ No ticket detected. Commit saved without a ticket or session.\n"
				case "full_diff_unsupported":
					message = "⚠ Full diff capture is not available in this feature; commit metadata was saved.\n"
				default:
					message = fmt.Sprintf("⚠ Commit metadata incomplete (%s); available data was saved.\n", warning)
				}
				if _, err := fmt.Fprint(cmd.ErrOrStderr(), message); err != nil {
					return err
				}
			}
			return nil
		},
	})
}
