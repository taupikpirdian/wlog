package summary

import (
	"context"
	"fmt"
	"strings"

	"github.com/taupikpirdian/wlog/application/bootstrap"
)

type AIConfigStore interface {
	LoadOrCreate(context.Context) (bootstrap.Config, error)
	SaveAI(context.Context, bootstrap.AIConfig) error
}

func ValidateAIConfig(config bootstrap.AIConfig) error {
	switch config.Provider {
	case "codex", "claude", "opencode", "custom":
	default:
		return fmt.Errorf("unsupported AI provider %q; run wl config ai", config.Provider)
	}
	provider, ok := config.Providers[config.Provider]
	if !ok || strings.TrimSpace(provider.Command) == "" {
		return fmt.Errorf("AI command for %s is missing; run wl config ai", config.Provider)
	}
	switch provider.Progress.Mode {
	case "", "none", "stderr", "jsonl":
	default:
		return fmt.Errorf("unsupported AI progress mode %q; use none, stderr, or jsonl", provider.Progress.Mode)
	}
	return nil
}
