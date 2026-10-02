//go:build unix

package githook

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	application "github.com/taupikpirdian/wlog/application/hook"
	domain "github.com/taupikpirdian/wlog/domain/hook"
)

var _ application.SnapshotReader = (*Store)(nil)

// Read inspects the installed hook without creating directories, locks or files.
func (*Store) Read(ctx context.Context, location domain.Location) (domain.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return domain.Snapshot{}, err
	}
	if err := domain.ValidateLocation(location); err != nil {
		return domain.Snapshot{}, err
	}
	if !filepath.IsAbs(location.HookPath) || filepath.Base(location.HookPath) != "post-commit" {
		return domain.Snapshot{}, domain.ErrInvalidLocation
	}
	directory := filepath.Dir(location.HookPath)
	if err := safeDirectory(directory); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.Snapshot{}, nil
		}
		return domain.Snapshot{}, err
	}
	target, err := readFile(location.HookPath)
	if err != nil {
		return domain.Snapshot{}, err
	}
	originalPath := filepath.Join(directory, domain.OriginalName)
	original, err := readFile(originalPath)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.Snapshot{}, err
	}
	if err := verifyFile(location.HookPath, target); err != nil {
		return domain.Snapshot{}, err
	}
	if err := verifyFile(originalPath, original); err != nil {
		return domain.Snapshot{}, err
	}
	return domain.Snapshot{Target: target.value, Original: original.value}, nil
}
