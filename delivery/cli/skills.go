package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
)

// Skills diagnostics use the same resolver as ticket generation, without
// launching an AI process or querying/modifying worklog records.
func NewSkillsCommand(config bootstrap.ConfigLoader, resolver application.TicketSkillResolver) *cobra.Command {
	root := &cobra.Command{Use: "skills", Short: "Inspect installed AI skills"}
	var provider, repository string
	check := &cobra.Command{
		Use: "check <skill>", Short: "Check and read installed skill instructions", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != application.TicketGeneratorSkill && args[0] != application.RootCauseSummarySkill {
				return errors.New("supported skills: ticket-generator, root-cause-summary")
			}
			if resolver == nil {
				return errors.New("AI skill resolver is unavailable")
			}
			if provider == "" && config != nil {
				value, err := config.LoadOrCreate(cmd.Context())
				if err != nil {
					return err
				}
				provider = value.AI.Provider
			}
			if provider == "" {
				provider = "codex"
			}
			switch provider {
			case "codex", "claude", "opencode", "custom":
			default:
				return errors.New("provider must be codex, claude, opencode or custom")
			}
			if repository == "" {
				var err error
				repository, err = os.Getwd()
				if err != nil {
					return err
				}
			}
			path, err := filepath.Abs(repository)
			if err != nil {
				return err
			}
			renderer := NewProgressRenderer(cmd.ErrOrStderr())
			skill, err := resolver.Resolve(cmd.Context(), provider, path, args[0], renderer.Render)
			if err != nil {
				return err
			}
			if err := renderer.Err(); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "AI Provider: %s\nSkill: %s\n", providerLabel(provider), args[0]); err != nil {
				return err
			}
			if len(skill.CheckedPaths) > 0 {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Checked locations:"); err != nil {
					return err
				}
				for _, path := range skill.CheckedPaths {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", safeText(path)); err != nil {
						return err
					}
				}
			}
			if !skill.Loaded {
				return fmt.Errorf("%s could not be loaded: %s; install SKILL.md in one of the checked locations", args[0], safeText(skill.FailureReason))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Status: Loaded\nPath: %s\n", safeText(skill.Path))
			return err
		},
	}
	check.Flags().StringVar(&provider, "provider", "", "AI provider (defaults to configured provider or codex)")
	check.Flags().StringVar(&repository, "repository", "", "Repository to check (defaults to current directory)")
	root.AddCommand(check)
	return root
}
