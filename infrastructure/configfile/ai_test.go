package configfile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taupikpirdian/wlog/application/bootstrap"
)

func TestSaveAIUsesExistingConfigAndPreservesFields(t *testing.T) {
	home := t.TempDir()
	loader := &Loader{homeDirectory: func() (string, error) { return home, nil }}
	initial, err := loader.LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if initial.ConfigFile != filepath.Join(home, ".worklog", "config.yaml") {
		t.Fatalf("config path=%s", initial.ConfigFile)
	}
	// Preserve custom settings and ai keys alongside the selected provider.
	file := []byte("database:\n  path: ~/.worklog/other.db\nticket:\n  pattern: '[A-Z]+-[0-9]+'\ncustom_setting: retain\nai:\n  enabled: false\n  include_diff: false\n  future_setting: retain\n")
	if err := os.WriteFile(initial.ConfigFile, file, 0600); err != nil {
		t.Fatal(err)
	}
	ai := bootstrap.AIConfig{Enabled: true, Provider: "custom", Providers: map[string]bootstrap.AIProviderConfig{"custom": {Command: "my-agent", Args: []string{"--prompt", "{{prompt}}"}, Progress: bootstrap.AIProgressConfig{Mode: "stderr"}}, "codex": {Command: "codex"}}}
	if err := loader.SaveAI(context.Background(), ai); err != nil {
		t.Fatal(err)
	}
	loaded, err := loader.LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DatabasePath != filepath.Join(home, ".worklog", "other.db") || loaded.AI.Provider != "custom" || !loaded.AI.Enabled || len(loaded.AI.Providers["custom"].Args) != 2 || loaded.AI.Providers["codex"].Command != "codex" {
		t.Fatalf("config=%+v", loaded)
	}
	if loaded.AI.Providers["custom"].Progress.Mode != "stderr" {
		t.Fatalf("progress config lost: %+v", loaded.AI)
	}
	body, err := os.ReadFile(initial.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"custom_setting: retain", "future_setting: retain", "include_diff: false"} {
		if !strings.Contains(string(body), field) {
			t.Fatalf("lost field %s: %s", field, body)
		}
	}
	info, err := os.Stat(initial.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions=%v", info.Mode())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := loader.SaveAI(ctx, ai); err == nil {
		t.Fatal("canceled write succeeded")
	}
}
