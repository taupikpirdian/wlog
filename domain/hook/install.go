package hook

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrUnsafe          = errors.New("hook path is a symlink or nonregular file")
	ErrConflict        = errors.New("hook state conflicts with managed integration; inspect the reported path")
	ErrBusy            = errors.New("hook installer lock exists; retry or inspect the lock manually")
	ErrUnsupported     = errors.New("custom core.hooksPath is unsupported; integrate wl git manually")
	ErrInvalidLocation = errors.New("hook installation requires a valid Git worktree location")
)

const (
	Installed        = "installed"
	AlreadyInstalled = "already-installed"
	Repaired         = "repaired"
	Updated          = "updated"
	OriginalName     = "post-commit.wlog-original"
	LockName         = "post-commit.wlog.lock"
	marker           = "wlog-managed-post-commit"
)

type Location struct {
	Root, CommonDirectory, HookPath string
	Shared, Custom                  bool
}
type File struct {
	Exists, Regular, Executable bool
	Content                     []byte
	Mode                        uint32
}
type Snapshot struct{ Target, Original File }
type Plan struct {
	Status                        string
	Executable                    string
	Content                       []byte
	Mode                          uint32
	PreserveOriginal, HasOriginal bool
}

func ValidateLocation(location Location) error {
	if location.Custom {
		return ErrUnsupported
	}
	if location.Root == "" || location.HookPath == "" {
		return ErrInvalidLocation
	}
	return nil
}

func PlanInstall(snapshot Snapshot) (Plan, error) {
	return PlanInstallWithExecutable(snapshot, "")
}

// PlanInstallWithExecutable pins capture to the installed binary, independently
// of the PATH inherited by Git from a terminal or editor.
func PlanInstallWithExecutable(snapshot Snapshot, executable string) (Plan, error) {
	if executable != "" && !validExecutable(executable) {
		return Plan{}, errors.New("hook capture executable must be an absolute path without control characters")
	}
	for _, file := range []File{snapshot.Target, snapshot.Original} {
		if file.Exists && !file.Regular {
			return Plan{}, ErrUnsafe
		}
	}
	if !snapshot.Target.Exists {
		if snapshot.Original.Exists {
			return Plan{}, ErrConflict
		}
		return Plan{Status: Installed, Content: renderCaptureWrapper(File{}, executable), Mode: 0755, Executable: executable}, nil
	}
	if bytes.Contains(snapshot.Target.Content, []byte(marker)) {
		installedExecutable, valid := managedExecutable(snapshot)
		if !valid {
			return Plan{}, ErrConflict
		}
		if executable == "" {
			executable = installedExecutable
		}
		expected := renderCaptureWrapper(snapshot.Original, executable)
		status := AlreadyInstalled
		if !snapshot.Target.Executable {
			status = Repaired
		}
		if !bytes.Equal(expected, snapshot.Target.Content) {
			status = Updated
		}
		return Plan{Status: status, Content: expected, Mode: snapshot.Target.Mode | 0100, HasOriginal: snapshot.Original.Exists, Executable: executable}, nil
	}
	if snapshot.Original.Exists {
		return Plan{}, ErrConflict
	}
	return Plan{Status: Installed, Content: renderCaptureWrapper(snapshot.Target, executable), Mode: snapshot.Target.Mode | 0100, PreserveOriginal: true, HasOriginal: true, Executable: executable}, nil
}

func validExecutable(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.ContainsAny(path, "\x00\r\n")
}

// Verify the complete managed wrapper and original backup, including wrappers
// installed by an older binary or at a different executable location.
func managedExecutable(snapshot Snapshot) (string, bool) {
	if bytes.Equal(snapshot.Target.Content, renderWrapper(snapshot.Original)) {
		return "", true
	}
	lines := strings.SplitN(string(snapshot.Target.Content), "\n", 3)
	if len(lines) < 2 {
		return "", false
	}
	_, quoted, found := strings.Cut(lines[1], " command=")
	if !found {
		return "", false
	}
	executable, err := strconv.Unquote(quoted)
	if err != nil || !validExecutable(executable) {
		return "", false
	}
	return executable, bytes.Equal(snapshot.Target.Content, renderCaptureWrapper(snapshot.Original, executable))
}

func renderCaptureWrapper(original File, executable string) []byte {
	legacy := renderWrapper(original)
	if executable == "" {
		return legacy
	}
	wrapper := strings.Replace(string(legacy), " v1 original=", " v2 original=", 1)
	lines := strings.SplitN(wrapper, "\n", 3)
	lines[1] += " command=" + strconv.Quote(executable)
	wrapper = strings.Join(lines, "\n")
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "'"
	return []byte(strings.Replace(wrapper, "\nwl git ", "\n"+quoted+" git ", 1))
}

func renderWrapper(original File) []byte {
	digest := "none"
	if original.Exists {
		digest = fmt.Sprintf("%x", sha256.Sum256(original.Content))
	}
	return []byte(fmt.Sprintf(`#!/bin/sh
# wlog-managed-post-commit v1 original=%s mode=%04o executable=%t
_wlog_original_status=0
if %t; then
    _wlog_hook_dir=${0%%/*}
    if [ "$_wlog_hook_dir" = "$0" ]; then
        _wlog_hook_dir=.
    fi
    "$_wlog_hook_dir/post-commit.wlog-original" "$@" || _wlog_original_status=$?
fi
wl git >/dev/null 2>&1 || true
exit "$_wlog_original_status"
`, digest, original.Mode, original.Executable, original.Exists && original.Executable))
}
