package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/taupikpirdian/wlog/application/bootstrap"
)

// Initializer is the application behavior required by the root CLI command.
type Initializer interface {
	Initialize(context.Context) (bootstrap.Result, error)
}

func NewRootCommand(initializer Initializer, version string, sessions SessionFactory, notes NoteFactory, captures ...GitFactory) *cobra.Command {
	root := &cobra.Command{
		Use:           "wl",
		Short:         "Capture developer work activity and prepare Jira worklogs",
		Long:          "Developer Worklog CLI prepares local worklog storage and provides commands for capturing work activity.",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := initializer.Initialize(cmd.Context())
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "wl ready\nData directory: %s\nConfig: %s\nDatabase: %s\n",
				result.DataDirectory,
				result.ConfigFile,
				result.DatabasePath,
			)
			return err
		},
	}
	root.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Root().Help()
			}
			target, _, err := cmd.Root().Find(args)
			if err != nil {
				return err
			}
			return target.Help()
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "wl version %s\n", version)
		},
	})
	addSessionCommands(root, sessions)
	addNoteCommand(root, notes)
	var capture GitFactory
	if len(captures) > 0 {
		capture = captures[0]
	}
	addGitCommand(root, capture)
	return root
}
