package gitevidence

import (
	"context"
	"fmt"
	"strings"
)

func validateRevision(rev string) error {
	if !hashPattern.MatchString(rev) {
		return fmt.Errorf("invalid recorded revision")
	}
	return nil
}

func (g *Service) Files(ctx context.Context, path, revision string) ([]string, error) {
	if err := validateRevision(revision); err != nil {
		return nil, err
	}
	text, err := g.command(ctx, path, "ls-tree", "-r", "--name-only", "-z", revision, "--")
	return nulPaths(text), err
}

func (g *Service) ChangedFiles(ctx context.Context, path, start, end string) ([]string, error) {
	if err := validateRevision(end); err != nil {
		return nil, err
	}
	var text string
	var err error
	if start == "" {
		text, err = g.command(ctx, path, "diff-tree", "--root", "--no-commit-id", "--name-only", "--no-renames", "-r", "-z", end, "--")
	} else {
		if err := validateRevision(start); err != nil {
			return nil, err
		}
		text, err = g.command(ctx, path, "diff", "--no-ext-diff", "--no-textconv", "--name-only", "--no-renames", "-z", start+".."+end, "--")
	}
	return nulPaths(text), err
}

func (g *Service) ReadFile(ctx context.Context, path, revision, file string) (string, error) {
	if err := validateRevision(revision); err != nil {
		return "", err
	}
	// Git reads the immutable blob; never runtime environment or working files.
	return g.command(ctx, path, "show", revision+":"+file)
}

func nulPaths(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\x00"), "\x00")
}
