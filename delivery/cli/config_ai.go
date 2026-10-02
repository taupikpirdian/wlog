package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
)

func NewConfigCommand(store application.AIConfigStore) *cobra.Command {
	root := &cobra.Command{Use: "config", Short: "Configure wlog"}
	root.AddCommand(&cobra.Command{Use: "ai", Short: "Select and configure an AI CLI provider", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := configureAI(cmd.Context(), lineReader(cmd.InOrStdin()), cmd.ErrOrStderr(), store)
		return err
	}})
	return root
}

func configureAI(ctx context.Context, input *bufio.Reader, out io.Writer, store application.AIConfigStore) (bootstrap.AIConfig, error) {
	current, err := store.LoadOrCreate(ctx)
	if err != nil {
		return bootstrap.AIConfig{}, err
	}
	if _, err := fmt.Fprint(out, "\nSelect AI Agent:\n  1. Codex\n  2. Claude\n  3. OpenCode\n  4. Custom\n"); err != nil {
		return bootstrap.AIConfig{}, err
	}
	providers := []string{"codex", "claude", "opencode", "custom"}
	var selected string
	for {
		if _, err := fmt.Fprint(out, "Choose agent [1-4]: "); err != nil {
			return bootstrap.AIConfig{}, err
		}
		value, err := readInputLine(input)
		if err != nil {
			return bootstrap.AIConfig{}, err
		}
		if strings.EqualFold(value, "q") {
			return bootstrap.AIConfig{}, fmt.Errorf("AI configuration canceled")
		}
		n, err := strconv.Atoi(value)
		if err == nil && n >= 1 && n <= 4 {
			selected = providers[n-1]
			break
		}
	}
	config := current.AI
	if config.Providers == nil {
		config.Providers = map[string]bootstrap.AIProviderConfig{}
	}
	provider := config.Providers[selected]
	if provider.Command == "" && selected != "custom" {
		provider.Command = selected
	}
	if _, err := fmt.Fprintf(out, "Executable [%s]: ", safeText(provider.Command)); err != nil {
		return bootstrap.AIConfig{}, err
	}
	command, err := readInputLine(input)
	if err != nil {
		return bootstrap.AIConfig{}, err
	}
	if command != "" {
		provider.Command = command
	}
	if selected == "custom" {
		if _, err := fmt.Fprint(out, "Arguments as JSON array (Enter for prompt on stdin; {{prompt}} supported): "); err != nil {
			return bootstrap.AIConfig{}, err
		}
		args, err := readInputLine(input)
		if err != nil {
			return bootstrap.AIConfig{}, err
		}
		if args != "" {
			if err := json.Unmarshal([]byte(args), &provider.Args); err != nil {
				return bootstrap.AIConfig{}, fmt.Errorf("invalid argument array: %w", err)
			}
		} else {
			provider.Args = nil
		}
	}
	config.Provider = selected
	config.Enabled = true
	config.Providers[selected] = provider
	if err := store.SaveAI(ctx, config); err != nil {
		return bootstrap.AIConfig{}, err
	}
	if _, err := fmt.Fprintf(out, "AI provider %s saved.\n", selected); err != nil {
		return bootstrap.AIConfig{}, err
	}
	return config, nil
}
