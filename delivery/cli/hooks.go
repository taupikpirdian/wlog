package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	application "github.com/taupikpirdian/wlog/application/hook"
	domain "github.com/taupikpirdian/wlog/domain/hook"
)

type HookInstaller interface {
	Install(context.Context) (application.Result, error)
}
type HookFactory func(context.Context) (HookInstaller, error)

func NewInstallHooksCommand(open HookFactory) *cobra.Command {
	return &cobra.Command{
		Use: "install-hooks", Aliases: []string{"install-hook"}, Short: "Install automatic Git post-commit capture", Args: sessionArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) (resultErr error) {
			if open == nil {
				return errors.New("hook installer service is unavailable")
			}
			service, err := open(cmd.Context())
			if err != nil {
				return fmt.Errorf("initialize hook installer: %w", err)
			}
			result, err := service.Install(cmd.Context())
			if err != nil {
				return err
			}
			var title string
			switch result.Status {
			case domain.Installed:
				title = "✓ Git integration installed"
			case domain.AlreadyInstalled:
				title = "✓ Git integration already installed"
			case domain.Repaired:
				title = "✓ Git hook executable permission repaired"
			default:
				return errors.New("hook installer returned an unknown outcome")
			}
			defer func() {
				if resultErr != nil {
					resultErr = fmt.Errorf("integration may already be installed; rerun to verify: %w", resultErr)
				}
			}()
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n\nRepository: %s\nHook: %s\n", title, result.Location.Root, result.Location.HookPath); err != nil {
				return err
			}
			if result.HasOriginal {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Original hook preserved: %s\nThe original hook runs from this backup path; hooks that depend on their filename require manual integration.\n", filepath.Join(filepath.Dir(result.Location.HookPath), domain.OriginalName)); err != nil {
					return err
				}
			}
			if result.Location.Shared {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Scope: all worktrees sharing this repository's default hooks directory."); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "\nCommits will automatically be captured when wl is available on the Git process PATH.")
			return err
		},
	}
}
