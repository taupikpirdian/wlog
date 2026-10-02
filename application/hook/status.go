package hook

import (
	"context"
	"errors"
	"fmt"

	domain "github.com/taupikpirdian/wlog/domain/hook"
)

const (
	NotInstalled = "not-installed"
	NeedsRepair  = "needs-repair"
	Modified     = "modified"
	CustomHooks  = "custom-hooks"
)

type SnapshotReader interface {
	Read(context.Context, domain.Location) (domain.Snapshot, error)
}

type StatusService struct {
	inspector Inspector
	reader    SnapshotReader
}

func NewStatusService(inspector Inspector, reader SnapshotReader) *StatusService {
	return &StatusService{inspector: inspector, reader: reader}
}

func (s *StatusService) Status(ctx context.Context) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	location, err := s.inspector.Inspect(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("inspect Git hooks: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	result := Result{Location: location}
	if location.Custom {
		result.Status = CustomHooks
		return result, nil
	}
	if err := domain.ValidateLocation(location); err != nil {
		return Result{}, err
	}
	snapshot, err := s.reader.Read(ctx, location)
	if err != nil {
		return Result{}, fmt.Errorf("read hook %q: %w", location.HookPath, err)
	}
	plan, err := domain.PlanInstall(snapshot)
	if err != nil {
		if !errors.Is(err, domain.ErrConflict) && !errors.Is(err, domain.ErrUnsafe) {
			return Result{}, err
		}
		result.Status = Modified
		return result, nil
	}
	result.HasOriginal = plan.HasOriginal
	switch plan.Status {
	case domain.AlreadyInstalled:
		result.Status = domain.Installed
	case domain.Repaired:
		result.Status = NeedsRepair
	default:
		result.Status = NotInstalled
	}
	return result, nil
}
