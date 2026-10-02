package githook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	application "github.com/taupikpirdian/wlog/application/hook"
	domain "github.com/taupikpirdian/wlog/domain/hook"
	"golang.org/x/sys/unix"
)

type Store struct{ checkpoint func(string) error }

func NewStore() *Store { return &Store{} }

var _ application.Store = (*Store)(nil)

type fileState struct {
	value domain.File
	info  os.FileInfo
}

func readFile(path string) (fileState, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileState{}, nil
	}
	if err != nil {
		return fileState{}, err
	}
	if !info.Mode().IsRegular() {
		return fileState{}, fmt.Errorf("%w: %s", domain.ErrUnsafe, path)
	}
	file, err := os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return fileState{}, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return fileState{}, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return fileState{}, fmt.Errorf("%w: %s", domain.ErrConflict, path)
	}
	body, err := io.ReadAll(file)
	if err != nil {
		return fileState{}, err
	}
	after, err := file.Stat()
	if err != nil {
		return fileState{}, err
	}
	if opened.Size() != after.Size() || opened.Mode() != after.Mode() || !opened.ModTime().Equal(after.ModTime()) {
		return fileState{}, fmt.Errorf("%w: %s", domain.ErrConflict, path)
	}
	return fileState{value: domain.File{Exists: true, Regular: true, Content: body, Mode: uint32(after.Mode().Perm()), Executable: unix.Access(path, unix.X_OK) == nil}, info: after}, nil
}

func sameFile(a, b fileState) bool {
	if a.value.Exists != b.value.Exists {
		return false
	}
	if !a.value.Exists {
		return true
	}
	return os.SameFile(a.info, b.info) && a.info.Mode() == b.info.Mode() && a.info.ModTime().Equal(b.info.ModTime()) && bytes.Equal(a.value.Content, b.value.Content)
}

func verifyFile(path string, expected fileState) error {
	current, err := readFile(path)
	if err != nil {
		return err
	}
	if !sameFile(current, expected) {
		return fmt.Errorf("%w: %s", domain.ErrConflict, path)
	}
	return nil
}

func removeOwned(path string, expected fileState) error {
	if err := verifyFile(path, expected); err != nil {
		return fmt.Errorf("preserved file; inspect recovery path %s: %w", path, err)
	}
	return os.Remove(path)
}

func safeDirectory(directory string) error {
	abs, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	current := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(abs, string(filepath.Separator)), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s", domain.ErrUnsafe, current)
		}
	}
	return nil
}

func prepareFile(directory string, body []byte, mode uint32) (path string, state fileState, resultErr error) {
	file, err := os.CreateTemp(directory, ".wlog-hook-*")
	if err != nil {
		return "", fileState{}, err
	}
	path = file.Name()
	var owned fileState
	defer func() {
		if resultErr != nil {
			if owned.info == nil {
				resultErr = fmt.Errorf("preserved temporary file; inspect recovery path %s: %w", path, resultErr)
			} else {
				resultErr = errors.Join(resultErr, removeOwned(path, owned))
			}
		}
	}()
	n, writeErr := file.Write(body)
	modeErr := file.Chmod(os.FileMode(mode))
	syncErr := file.Sync()
	info, statErr := file.Stat()
	if statErr == nil {
		owned = fileState{value: domain.File{Exists: true, Regular: true, Content: body[:n], Mode: uint32(info.Mode().Perm())}, info: info}
	}
	closeErr := file.Close()
	if err := errors.Join(writeErr, modeErr, syncErr, statErr, closeErr); err != nil {
		return path, fileState{}, err
	}
	state, err = readFile(path)
	return path, state, err
}

func (s *Store) Apply(ctx context.Context, location domain.Location, planner application.Planner) (result domain.Plan, resultErr error) {
	published := false
	defer func() {
		if published && resultErr != nil {
			resultErr = fmt.Errorf("integration may already be installed; rerun to verify: %w", resultErr)
		}
	}()
	if err := ctx.Err(); err != nil {
		return domain.Plan{}, err
	}
	if err := domain.ValidateLocation(location); err != nil {
		return domain.Plan{}, err
	}
	if !filepath.IsAbs(location.HookPath) || filepath.Base(location.HookPath) != "post-commit" {
		return domain.Plan{}, domain.ErrInvalidLocation
	}
	directory := filepath.Dir(location.HookPath)
	if err := safeDirectory(filepath.Dir(directory)); err != nil {
		return domain.Plan{}, err
	}
	if err := os.Mkdir(directory, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return domain.Plan{}, err
	}
	if err := safeDirectory(directory); err != nil {
		return domain.Plan{}, err
	}
	lockPath := filepath.Join(directory, domain.LockName)
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			info, inspectErr := os.Lstat(lockPath)
			if inspectErr != nil {
				return domain.Plan{}, inspectErr
			}
			if !info.Mode().IsRegular() {
				return domain.Plan{}, fmt.Errorf("%w: %s", domain.ErrUnsafe, lockPath)
			}
			return domain.Plan{}, fmt.Errorf("%w: %s", domain.ErrBusy, lockPath)
		}
		return domain.Plan{}, err
	}
	lockInfo, statErr := lock.Stat()
	closeErr := lock.Close()
	if statErr != nil {
		return domain.Plan{}, errors.Join(statErr, closeErr)
	}
	lockState := fileState{value: domain.File{Exists: true, Regular: true, Mode: 0600}, info: lockInfo}
	defer func() { resultErr = errors.Join(resultErr, removeOwned(lockPath, lockState)) }()
	if closeErr != nil {
		return domain.Plan{}, closeErr
	}
	target, err := readFile(location.HookPath)
	if err != nil {
		return domain.Plan{}, err
	}
	originalPath := filepath.Join(directory, domain.OriginalName)
	original, err := readFile(originalPath)
	if err != nil {
		return domain.Plan{}, err
	}
	result, err = planner(domain.Snapshot{Target: target.value, Original: original.value})
	if err != nil {
		return domain.Plan{}, fmt.Errorf("inspect hook %s and backup %s: %w", location.HookPath, originalPath, err)
	}
	if err := ctx.Err(); err != nil {
		return domain.Plan{}, err
	}
	if result.Status == domain.AlreadyInstalled {
		return result, nil
	}
	if result.Status == domain.Repaired {
		if err := safeDirectory(directory); err != nil {
			return domain.Plan{}, err
		}
		if err := verifyFile(location.HookPath, target); err != nil {
			return domain.Plan{}, err
		}
		if err := verifyFile(originalPath, original); err != nil {
			return domain.Plan{}, err
		}
		file, err := os.OpenFile(location.HookPath, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
		if err != nil {
			return domain.Plan{}, err
		}
		info, statErr := file.Stat()
		if statErr != nil || !os.SameFile(info, target.info) {
			return domain.Plan{}, errors.Join(statErr, domain.ErrConflict, file.Close())
		}
		if err := ctx.Err(); err != nil {
			return domain.Plan{}, errors.Join(err, file.Close())
		}
		modeErr := file.Chmod(os.FileMode(result.Mode))
		if modeErr == nil {
			published = true
		}
		return result, errors.Join(modeErr, file.Close())
	}
	staged, stagedState, err := prepareFile(directory, result.Content, result.Mode)
	if err != nil {
		return domain.Plan{}, err
	}
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, removeOwned(staged, stagedState))
		}
	}()
	var backupState fileState
	backupCreated := false
	defer func() {
		if backupCreated && !published {
			resultErr = errors.Join(resultErr, removeOwned(originalPath, backupState))
		}
	}()
	if result.PreserveOriginal {
		copyPath, copyState, err := prepareFile(directory, target.value.Content, target.value.Mode)
		if err != nil {
			return domain.Plan{}, err
		}
		defer func() { resultErr = errors.Join(resultErr, removeOwned(copyPath, copyState)) }()
		if err := os.Link(copyPath, originalPath); err != nil {
			return domain.Plan{}, fmt.Errorf("preserve original %s: %w", originalPath, err)
		}
		backupCreated = true
		backupState = copyState
	}
	if s.checkpoint != nil {
		if err := s.checkpoint("before-publish"); err != nil {
			return domain.Plan{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return domain.Plan{}, err
	}
	if err := safeDirectory(directory); err != nil {
		return domain.Plan{}, err
	}
	if err := verifyFile(location.HookPath, target); err != nil {
		return domain.Plan{}, err
	}
	if backupCreated {
		if err := verifyFile(originalPath, backupState); err != nil {
			return domain.Plan{}, err
		}
	} else {
		if err := verifyFile(originalPath, original); err != nil {
			return domain.Plan{}, err
		}
	}
	if err := verifyFile(staged, stagedState); err != nil {
		return domain.Plan{}, err
	}
	if err := os.Rename(staged, location.HookPath); err != nil {
		return domain.Plan{}, err
	}
	published = true
	if s.checkpoint != nil {
		if err := s.checkpoint("published"); err != nil {
			return result, err
		}
	}
	dir, err := os.Open(directory)
	if err != nil {
		return result, err
	}
	return result, errors.Join(dir.Sync(), dir.Close())
}
