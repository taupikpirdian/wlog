// Package gitevidence reads immutable changes from captured commit identities.
package gitevidence

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	application "github.com/taupikpirdian/wlog/application/summary"
)

const maxGitOutput = 256 * 1024

var hashPattern = regexp.MustCompile(`^[a-fA-F0-9]{4,64}$`)

type Service struct{}

func NewService() *Service { return &Service{} }

// All commands target the database repository explicitly and stay offline.
func (g *Service) command(ctx context.Context, repository string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-lazy-fetch", "-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Dir = repository
	// Inherited Git overrides must never redirect the database repository.
	for _, env := range os.Environ() {
		key := strings.SplitN(env, "=", 2)[0]
		if strings.HasPrefix(key, "GIT_") {
			continue
		}
		cmd.Env = append(cmd.Env, env)
	}
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
	var output cappedBuffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if output.exceeded {
		return "", fmt.Errorf("Git output exceeds %d bytes; source-code context omitted for this commit", maxGitOutput)
	}
	if err != nil {
		return "", fmt.Errorf("Git %s failed: %w (%s)", args[0], err, strings.TrimSpace(output.String()))
	}
	return output.String(), nil
}

type cappedBuffer struct {
	bytes.Buffer
	exceeded bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := maxGitOutput - b.Len()
	if n > remaining {
		b.exceeded = true
		p = p[:remaining]
	}
	_, err := b.Buffer.Write(p)
	return n, err
}

func (g *Service) ValidateRepository(ctx context.Context, path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("recorded repository path must be absolute: %s", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("repository not found: %s; restore the recorded directory", path)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repository path is not a directory: %s", path)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	output, err := g.command(ctx, canonical, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("directory is no longer a Git worktree: %w", err)
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(output))
	if err != nil {
		return "", err
	}
	if root != canonical {
		return "", fmt.Errorf("recorded path is not the repository root: %s (root: %s)", canonical, root)
	}
	return root, nil
}

func (g *Service) Change(ctx context.Context, path, hash string) (application.CodeChange, error) {
	if !hashPattern.MatchString(hash) {
		return application.CodeChange{}, fmt.Errorf("invalid or missing captured commit hash %q", hash)
	}
	resolved, err := g.command(ctx, path, "rev-parse", "--verify", hash+"^{commit}")
	if err != nil {
		return application.CodeChange{}, fmt.Errorf("commit %s was not found in the repository; restore its Git history: %w", hash, err)
	}
	end := strings.TrimSpace(resolved)
	parents, err := g.command(ctx, path, "show", "-s", "--format=%P", end, "--")
	if err != nil {
		return application.CodeChange{}, err
	}
	change := application.CodeChange{CommitRange: application.CommitRange{End: end}}
	if fields := strings.Fields(parents); len(fields) > 0 {
		change.Start = fields[0]
		change.Diff, err = g.command(ctx, path, "diff", "--no-ext-diff", "--no-textconv", "--no-color", change.Start+".."+end, "--")
	} else {
		// Root commits have no start object; Git's root diff is the exact equivalent.
		change.Diff, err = g.command(ctx, path, "show", "--root", "--first-parent", "--format=", "--no-ext-diff", "--no-textconv", "--no-color", end, "--")
	}
	if err != nil {
		return application.CodeChange{}, fmt.Errorf("read diff for captured commit %s: %w", hash, err)
	}
	return change, nil
}
