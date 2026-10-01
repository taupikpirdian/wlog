package gitcapture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	application "github.com/taupikpirdian/wlog/application/activity"
	"github.com/taupikpirdian/wlog/domain/activity"
)

type Runner func(context.Context, string, ...string) ([]byte, error)

type Reader struct {
	directory string
	run       Runner
}

func NewReader(directory string) *Reader { return NewReaderWithRunner(directory, runGit) }
func NewReaderWithRunner(directory string, runner Runner) *Reader {
	return &Reader{directory: directory, run: runner}
}

var _ application.GitReader = (*Reader)(nil)

func runGit(ctx context.Context, directory string, args ...string) ([]byte, error) {
	// Git must not hydrate missing objects from a remote or use replacement refs.
	arguments := append([]string{"--no-lazy-fetch", "--no-replace-objects", "-C", directory}, args...)
	return exec.CommandContext(ctx, "git", arguments...).Output()
}

func (r *Reader) Read(ctx context.Context, options application.CaptureOptions) (activity.Commit, error) {
	budget, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	rootBytes, err := r.run(budget, r.directory, "rev-parse", "--show-toplevel")
	if err != nil {
		return activity.Commit{}, fmt.Errorf("Git capture requires a worktree and Git with offline capture support: %w", err)
	}
	// Git adds exactly one output newline; preserve whitespace within the path.
	root := strings.TrimSuffix(string(rootBytes), "\n")
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return activity.Commit{}, fmt.Errorf("resolve Git worktree: %w", err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return activity.Commit{}, err
	}
	hashBytes, err := r.run(budget, root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return activity.Commit{}, fmt.Errorf("Git capture requires an existing HEAD commit: %w", err)
	}
	hash := strings.TrimSpace(string(hashBytes))
	if hash == "" {
		return activity.Commit{}, errors.New("Git returned an empty commit identity")
	}
	value := activity.Commit{Repository: root, Hash: hash, Metadata: activity.CommitMetadata{Status: map[string]string{}, DiffBasis: "not_collected", RenameMode: "delete_add", StatScope: "non_env_text_files"}}
	status := value.Metadata.Status
	optional := func(field string, args ...string) ([]byte, bool) {
		output, err := r.run(budget, root, args...)
		if err != nil {
			status[field] = "unavailable"
			value.Metadata.Warnings = append(value.Metadata.Warnings, field+"_unavailable")
			return nil, false
		}
		status[field] = "collected"
		return output, true
	}
	if output, ok := optional("message", "log", "-1", "--no-show-signature", "--format=format:%B", hash, "--"); ok {
		message := string(output)
		value.Message = &message
	}
	if output, ok := optional("timestamp", "log", "-1", "--no-show-signature", "--format=format:%cI", hash, "--"); ok {
		at, err := time.Parse(time.RFC3339, strings.TrimSpace(string(output)))
		if err != nil {
			status["timestamp"] = "unavailable"
			value.Metadata.Warnings = append(value.Metadata.Warnings, "timestamp_unavailable")
		} else {
			value.CreatedAt = at.UTC()
		}
	}
	branchBytes, branchErr := r.run(budget, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if branchErr == nil {
		branch := strings.TrimSuffix(string(branchBytes), "\n")
		latest, err := r.run(budget, root, "rev-parse", "--verify", "HEAD^{commit}")
		if err != nil || strings.TrimSpace(string(latest)) != hash {
			status["branch"] = "unstable"
			value.Metadata.Warnings = append(value.Metadata.Warnings, "branch_unstable")
		} else {
			value.Branch = &branch
			status["branch"] = "collected"
		}
	} else {
		var exit interface{ ExitCode() int }
		if errors.As(branchErr, &exit) && exit.ExitCode() == 1 {
			status["branch"] = "detached"
		} else {
			status["branch"] = "unavailable"
			value.Metadata.Warnings = append(value.Metadata.Warnings, "branch_unavailable")
		}
	}
	status["changed_files"], status["diff_stat"] = "disabled", "disabled"
	if options.ChangedFiles || options.DiffStat {
		parents, parentErr := r.run(budget, root, "rev-list", "--parents", "-n", "1", hash, "--")
		identities := strings.Fields(string(parents))
		if parentErr == nil && len(identities) > 0 && identities[0] == hash {
			comparison := []string{"--root", hash}
			value.Metadata.DiffBasis = "empty_tree"
			if len(identities) > 1 {
				comparison = []string{identities[1], hash}
				value.Metadata.DiffBasis = "first_parent"
			}
			base := []string{"--attr-source=" + hash, "diff-tree", "--no-commit-id", "-r", "--no-renames", "--no-ext-diff", "--no-textconv", "--no-color", "--diff-algorithm=myers", "--no-indent-heuristic"}
			if options.ChangedFiles {
				args := append(append(append([]string{}, base...), "--name-only", "-z"), comparison...)
				args = append(args, "--")
				if output, ok := optional("changed_files", args...); ok {
					files, err := parsePaths(output)
					if err != nil {
						status["changed_files"] = "unavailable"
						value.Metadata.Warnings = append(value.Metadata.Warnings, "changed_files_unavailable")
					} else {
						value.Files = files
					}
				}
			}
			if options.DiffStat {
				args := append(append(append([]string{}, base...), "--numstat", "-z"), comparison...)
				// Exclude before computing content statistics, never after parsing.
				args = append(args, "--", ".", ":(glob,exclude).env*", ":(glob,exclude)**/.env*")
				if output, ok := optional("diff_stat", args...); ok {
					added, removed, binary, err := parseNumstat(output)
					if err != nil {
						status["diff_stat"] = "unavailable"
						value.Metadata.Warnings = append(value.Metadata.Warnings, "diff_stat_unavailable")
					} else {
						value.Insertions, value.Deletions, value.Metadata.BinaryFiles = &added, &removed, binary
					}
				}
			}
		} else {
			value.Metadata.DiffBasis = "unavailable"
			if options.ChangedFiles {
				status["changed_files"] = "unavailable"
				value.Metadata.Warnings = append(value.Metadata.Warnings, "changed_files_unavailable")
			}
			if options.DiffStat {
				status["diff_stat"] = "unavailable"
				value.Metadata.Warnings = append(value.Metadata.Warnings, "diff_stat_unavailable")
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return activity.Commit{}, err
	}
	return value, nil
}

func parsePaths(output []byte) ([]string, error) {
	files := []string{}
	if len(output) == 0 {
		return files, nil
	}
	if output[len(output)-1] != 0 {
		return nil, errors.New("unterminated Git path output")
	}
	seen := map[string]bool{}
	for _, path := range bytes.Split(output[:len(output)-1], []byte{0}) {
		if len(path) == 0 {
			return nil, errors.New("empty Git path")
		}
		if !seen[string(path)] {
			files = append(files, string(path))
			seen[string(path)] = true
		}
	}
	sort.Strings(files)
	return files, nil
}

func parseNumstat(output []byte) (added, removed int64, binary []string, err error) {
	binary = []string{}
	if len(output) == 0 {
		return
	}
	if output[len(output)-1] != 0 {
		return 0, 0, nil, errors.New("unterminated Git numstat output")
	}
	for _, record := range bytes.Split(output[:len(output)-1], []byte{0}) {
		parts := bytes.SplitN(record, []byte{'\t'}, 3)
		if len(parts) != 3 || len(parts[2]) == 0 {
			return 0, 0, nil, errors.New("invalid Git numstat record")
		}
		if string(parts[0]) == "-" && string(parts[1]) == "-" {
			binary = append(binary, string(parts[2]))
			continue
		}
		a, e1 := strconv.ParseInt(string(parts[0]), 10, 64)
		d, e2 := strconv.ParseInt(string(parts[1]), 10, 64)
		if e1 != nil || e2 != nil || a < 0 || d < 0 || added > (1<<63-1)-a || removed > (1<<63-1)-d {
			return 0, 0, nil, errors.New("invalid Git numstat count")
		}
		added += a
		removed += d
	}
	sort.Strings(binary)
	return
}
