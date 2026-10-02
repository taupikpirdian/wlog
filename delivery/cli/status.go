package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	application "github.com/taupikpirdian/wlog/application/hook"
	domain "github.com/taupikpirdian/wlog/domain/hook"
)

type HookStatusReader interface {
	Status(context.Context) (application.Result, error)
}

type HookStatusFactory func(context.Context) (HookStatusReader, error)

func NewStatusCommand(dashboard DashboardFactory, hooks HookStatusFactory) *cobra.Command {
	return &cobra.Command{
		Use: "status", Short: "Show work dashboard and Git hook status for the current repository", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runDaily(dashboard, false)(cmd, args); err != nil {
				return err
			}
			if hooks == nil {
				return errors.New("hook status service is unavailable")
			}
			reader, err := hooks(cmd.Context())
			var result application.Result
			if err == nil {
				result, err = reader.Status(cmd.Context())
			}
			if cmd.Context().Err() != nil {
				return cmd.Context().Err()
			}
			if err != nil {
				_, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "\nGit hook: unavailable (%s)\n", safeText(err.Error()))
				return writeErr
			}
			output, err := renderHookStatus(result)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), output)
			return err
		},
	}
}

func renderHookStatus(result application.Result) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "\nRepository: %s\n", safeText(result.Location.Root))
	switch result.Status {
	case domain.Installed:
		b.WriteString("Git hook: installed (automatic commit capture)\n")
		if result.Executable == "" {
			b.WriteString("Legacy hook depends on Git's PATH. Run wl install-hooks to enable capture from editors using the installed binary path.\n")
		}
	case application.NotInstalled:
		b.WriteString("Git hook: not installed\nRun wl install-hooks in this repository to enable automatic commit capture.\n")
	case application.NeedsRepair:
		b.WriteString("Git hook: needs repair (not executable)\nRun wl install-hooks in this repository to repair executable permissions.\n")
	case application.Modified:
		b.WriteString("Git hook: modified or conflicting; automatic capture could not be verified\nInspect the hook and its backup before reinstalling.\n")
	case application.CustomHooks:
		b.WriteString("Git hook: custom core.hooksPath; automatic capture could not be verified\nIntegrate wl git into your existing hook configuration manually.\n")
	default:
		return "", errors.New("hook status service returned an unknown outcome")
	}
	if result.Location.HookPath != "" {
		fmt.Fprintf(&b, "Hook: %s\n", safeText(result.Location.HookPath))
	}
	if result.Executable != "" {
		fmt.Fprintf(&b, "Capture executable: %s\n", safeText(result.Executable))
	}
	if result.Location.Shared {
		b.WriteString("Scope: all worktrees sharing this repository's default hooks directory.\n")
	}
	return b.String(), nil
}
