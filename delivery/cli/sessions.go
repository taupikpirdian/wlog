package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	domain "github.com/taupikpirdian/wlog/domain/session"
)

type SessionService interface {
	Validate(string, string) error
	Active(context.Context) (*domain.Session, error)
	Start(context.Context, string, string, *domain.Session) (domain.Session, error)
	Stop(context.Context) (domain.Session, error)
}

type SessionFactory func(context.Context) (SessionService, func() error, error)

func addSessionCommands(root *cobra.Command, open SessionFactory) {
	root.AddCommand(&cobra.Command{
		Use: "start <ticket> <title>", Aliases: []string{"s"}, Short: "Start a work session", Args: sessionArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) (resultErr error) {
			service, close, err := open(cmd.Context())
			if err != nil {
				return err
			}
			defer func() { resultErr = errors.Join(resultErr, close()) }()
			if err := service.Validate(args[0], args[1]); err != nil {
				_ = cmd.Usage()
				return err
			}
			active, err := service.Active(cmd.Context())
			if err != nil {
				return err
			}
			if active != nil {
				confirmed, err := confirmSwitch(cmd, *active, args[0], terminalInput(cmd.InOrStdin()))
				if err != nil {
					return err
				}
				if !confirmed {
					_, err = fmt.Fprintln(cmd.OutOrStdout(), "Session start cancelled")
					return err
				}
			}
			value, err := service.Start(cmd.Context(), args[0], args[1], active)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "✓ Session started\n\n%s\n%s\n\nStarted: %s\n", value.TicketKey, value.Title, value.StartedAt.Local().Format("15:04"))
			return err
		},
	})
	root.AddCommand(&cobra.Command{
		Use: "stop", Aliases: []string{"x"}, Short: "Complete the active work session", Args: sessionArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) (resultErr error) {
			service, close, err := open(cmd.Context())
			if err != nil {
				return err
			}
			defer func() { resultErr = errors.Join(resultErr, close()) }()
			value, err := service.Stop(cmd.Context())
			if err != nil {
				return err
			}
			start, end := value.StartedAt.Local(), value.EndedAt.Local()
			layout := "15:04"
			if start.Format("2006-01-02") != end.Format("2006-01-02") {
				layout = "2006-01-02 15:04"
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "✓ Session completed\n\n%s\n%s\n\n%s → %s\nDuration: %s\n", value.TicketKey, value.Title, start.Format(layout), end.Format(layout), formatDuration(*value.DurationSeconds))
			return err
		},
	})
}

func sessionArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validate(cmd, args); err != nil {
			_ = cmd.Usage()
			return err
		}
		return nil
	}
}

func terminalInput(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	return ok && isatty.IsTerminal(file.Fd())
}

func confirmSwitch(cmd *cobra.Command, active domain.Session, key string, interactive bool) (bool, error) {
	seconds := int64(time.Since(active.StartedAt) / time.Second)
	if seconds < 0 {
		seconds = 0
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Active session detected\n\n%s\n%s\nStarted: %s\nElapsed: %s\n\n", active.TicketKey, active.Title, active.StartedAt.Local().Format("2006-01-02 15:04"), formatDuration(seconds)); err != nil {
		return false, err
	}
	if !interactive {
		return false, errors.New("session remains active; switching requires confirmation from an interactive terminal, or run wl stop first")
	}
	reader := bufio.NewReader(cmd.InOrStdin())
	for {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Stop this session and start %s? [Y/n] ", key); err != nil {
			return false, err
		}
		answer, err := reader.ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("session remains active; confirmation was not received: %w", err)
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "", "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Please answer yes or no."); err != nil {
				return false, err
			}
		}
	}
}

func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%dh %dm", seconds/3600, seconds%3600/60)
}
