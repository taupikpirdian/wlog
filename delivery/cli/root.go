package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewRootCommand(dashboard DashboardFactory, version string, sessions SessionFactory, notes NoteFactory, captures ...GitFactory) *cobra.Command {
	root := &cobra.Command{
		Use:   "wl",
		Short: "Capture developer work activity and prepare Jira worklogs",
		Long:  "Developer Worklog CLI displays today's tracked work and captures local work activity.",
		Example: `  wl s OOT-3751 "Fix tax calculation"
  wl n "Check tax calculation"
  wl today
  wl x`,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE:          runDaily(dashboard, false),
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
	addTodayCommand(root, dashboard)
	var capture GitFactory
	if len(captures) > 0 {
		capture = captures[0]
	}
	addGitCommand(root, capture)
	return root
}
