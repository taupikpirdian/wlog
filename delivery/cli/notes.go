package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/taupikpirdian/wlog/domain/activity"
	"github.com/taupikpirdian/wlog/domain/session"
)

type NoteService interface {
	AddNote(context.Context, string) (activity.Note, error)
}
type NoteFactory func(context.Context) (NoteService, func() error, error)

func addNoteCommand(root *cobra.Command, open NoteFactory) {
	root.AddCommand(&cobra.Command{
		Use: "note <description>", Aliases: []string{"n"}, Short: "Add a note to the active work session",
		Args: func(cmd *cobra.Command, args []string) error {
			err := cobra.ExactArgs(1)(cmd, args)
			if err == nil {
				_, err = activity.ValidateDescription(args[0])
			}
			if err != nil {
				_ = cmd.Usage()
			}
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) (resultErr error) {
			service, close, err := open(cmd.Context())
			if err != nil {
				return err
			}
			defer func() { resultErr = errors.Join(resultErr, close()) }()
			value, err := service.AddNote(cmd.Context(), args[0])
			if err != nil {
				if errors.Is(err, session.ErrNoActive) {
					return noActiveNoteError{}
				}
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "✓ Note added to %s\n", value.TicketKey)
			return err
		},
	})
}

type noActiveNoteError struct{}

func (noActiveNoteError) Error() string {
	return "✗ No active session.\n\nStart one using:\n\nwl s <ticket> \"<title>\""
}
func (noActiveNoteError) Unwrap() error { return session.ErrNoActive }
