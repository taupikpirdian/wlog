package hook

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
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
	for _, file := range []File{snapshot.Target, snapshot.Original} {
		if file.Exists && !file.Regular {
			return Plan{}, ErrUnsafe
		}
	}
	if !snapshot.Target.Exists {
		if snapshot.Original.Exists {
			return Plan{}, ErrConflict
		}
		return Plan{Status: Installed, Content: renderWrapper(File{}), Mode: 0755}, nil
	}
	if bytes.Contains(snapshot.Target.Content, []byte(marker)) {
		expected := renderWrapper(snapshot.Original)
		if !bytes.Equal(expected, snapshot.Target.Content) {
			return Plan{}, ErrConflict
		}
		status := AlreadyInstalled
		if !snapshot.Target.Executable {
			status = Repaired
		}
		return Plan{Status: status, Content: expected, Mode: snapshot.Target.Mode | 0100, HasOriginal: snapshot.Original.Exists}, nil
	}
	if snapshot.Original.Exists {
		return Plan{}, ErrConflict
	}
	return Plan{Status: Installed, Content: renderWrapper(snapshot.Target), Mode: snapshot.Target.Mode | 0100, PreserveOriginal: true, HasOriginal: true}, nil
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
