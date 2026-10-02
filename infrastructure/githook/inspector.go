package githook

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	application "github.com/taupikpirdian/wlog/application/hook"
	domain "github.com/taupikpirdian/wlog/domain/hook"
)

type Inspector struct{ directory string }

func NewInspector(directory string) *Inspector { return &Inspector{directory: directory} }

var _ application.Inspector = (*Inspector)(nil)

func (i *Inspector) Inspect(ctx context.Context) (domain.Location, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	run := func(dir string, args ...string) (string, error) {
		output, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
		return strings.TrimSuffix(string(output), "\n"), err
	}
	root, err := run(i.directory, "rev-parse", "--show-toplevel")
	if err != nil {
		return domain.Location{}, fmt.Errorf("requires a non-bare Git worktree: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return domain.Location{}, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return domain.Location{}, err
	}
	_, err = run(root, "config", "--get", "core.hooksPath")
	if err == nil {
		return domain.Location{Root: root, Custom: true}, nil
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		return domain.Location{}, fmt.Errorf("read core.hooksPath: %w", err)
	}
	common, err := run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return domain.Location{}, err
	}
	path, err := run(root, "rev-parse", "--path-format=absolute", "--git-path", "hooks/post-commit")
	if err != nil {
		return domain.Location{}, err
	}
	relative, err := filepath.Rel(common, path)
	if err != nil || relative != filepath.Join("hooks", "post-commit") {
		return domain.Location{}, domain.ErrInvalidLocation
	}
	common, err = filepath.EvalSymlinks(common)
	if err != nil {
		return domain.Location{}, err
	}
	worktrees, err := run(root, "worktree", "list", "--porcelain")
	if err != nil {
		return domain.Location{}, err
	}
	count := 0
	for _, line := range strings.Split(worktrees, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			count++
		}
	}
	return domain.Location{Root: root, CommonDirectory: common, HookPath: filepath.Join(common, relative), Shared: count > 1}, nil
}
