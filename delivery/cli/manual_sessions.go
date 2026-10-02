package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	domain "github.com/taupikpirdian/wlog/domain/session"
)

type ManualSessionService interface {
	CreateCompleted(context.Context, string, string, string, string) (domain.Session, error)
	StartSince(context.Context, string, string, string) (domain.Session, error)
}
type ManualSessionFactory func(context.Context) (ManualSessionService, func() error, error)

// AddManualSessionCommands extends an existing root without changing its
// ordinary start/stop factory or its confirmation behavior.
func AddManualSessionCommands(root *cobra.Command, open ManualSessionFactory) {
	var from, to, title string
	command := &cobra.Command{
		Use: "session <ticket>", Short: "Add a completed work session with explicit local times",
		Args: sessionArgs(func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("session requires exactly one ticket argument")
			}
			if err := domain.ValidateWallTime(from); err != nil {
				return err
			}
			if err := domain.ValidateWallTime(to); err != nil {
				return err
			}
			_, err := domain.ValidateTitle(title)
			return err
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runManualSession(cmd, open, func(service ManualSessionService) (domain.Session, error) {
				return service.CreateCompleted(cmd.Context(), args[0], title, from, to)
			}, false)
		},
	}
	command.Flags().StringVar(&from, "from", "", "Start time HH:mm on today's local date")
	command.Flags().StringVar(&to, "to", "", "End time HH:mm on today's local date")
	command.Flags().StringVar(&title, "title", "", "Session title")
	for _, flag := range []string{"from", "to", "title"} {
		_ = command.MarkFlagRequired(flag)
	}
	root.AddCommand(command)
	for _, start := range root.Commands() {
		if start.Name() != "start" {
			continue
		}
		var since string
		start.Flags().StringVarP(&since, "since", "s", "", "Start from HH:mm today without switching an active session")
		originalArgs, originalRun := start.Args, start.RunE
		start.Args = func(cmd *cobra.Command, args []string) error {
			if err := originalArgs(cmd, args); err != nil {
				return err
			}
			if !cmd.Flags().Changed("since") {
				return nil
			}
			if err := domain.ValidateWallTime(since); err != nil {
				return err
			}
			_, err := domain.ValidateTitle(args[1])
			return err
		}
		start.RunE = func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("since") {
				return originalRun(cmd, args)
			}
			return runManualSession(cmd, open, func(service ManualSessionService) (domain.Session, error) {
				return service.StartSince(cmd.Context(), args[0], args[1], since)
			}, true)
		}
	}
}

func runManualSession(cmd *cobra.Command, open ManualSessionFactory, call func(ManualSessionService) (domain.Session, error), active bool) (resultErr error) {
	service, close, err := open(cmd.Context())
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, close()) }()
	value, err := call(service)
	if err != nil {
		return manualSessionError(err)
	}
	if active {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "✓ Session started\n\n%s\n%s\n\nStarted: %s\n", safeText(value.TicketKey), safeText(value.Title), value.StartedAt.Local().Format("15:04"))
	} else {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "✓ Manual session added\n\n%s\n%s\n\n%s → %s\nDuration: %s\n", safeText(value.TicketKey), safeText(value.Title), value.StartedAt.Local().Format("15:04"), value.EndedAt.Local().Format("15:04"), dailyDuration(*value.DurationSeconds))
	}
	return err
}

func manualSessionError(err error) error {
	var conflict *domain.ManualConflict
	if !errors.As(err, &conflict) {
		return err
	}
	if conflict.Active {
		return fmt.Errorf("✗ Active session exists: %s\n%w", safeText(conflict.Existing.TicketKey), err)
	}
	old := conflict.Existing
	end := "active"
	if old.EndedAt != nil {
		end = old.EndedAt.Local().Format("02 Jan 2006 15:04")
	}
	return fmt.Errorf("✗ Session overlaps %s (session #%d)\nExisting: %s → %s\nChoose a non-overlapping range.\n%w", safeText(old.TicketKey), old.ID, old.StartedAt.Local().Format("02 Jan 2006 15:04"), end, err)
}
