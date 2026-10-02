//go:build !unix

package githook

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	domain "github.com/taupikpirdian/wlog/domain/hook"
)

func TestUnsupportedInstallerDoesNotInspectOrModifyHooks(t *testing.T) {
	directory := t.TempDir()
	hookPath := filepath.Join(directory, "post-commit")
	if err := os.WriteFile(hookPath, []byte("existing hook"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewInspector(directory).Inspect(context.Background()); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatal(err)
	}
	planned := false
	_, err := NewStore().Apply(context.Background(), domain.Location{HookPath: hookPath}, func(domain.Snapshot) (domain.Plan, error) { planned = true; return domain.Plan{}, nil })
	if !errors.Is(err, ErrUnsupportedPlatform) || planned {
		t.Fatalf("error=%v planner called=%t", err, planned)
	}
	contents, err := os.ReadFile(hookPath)
	if err != nil || string(contents) != "existing hook" {
		t.Fatalf("hook changed: %q %v", contents, err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("unexpected filesystem changes: %v %v", entries, err)
	}
}
