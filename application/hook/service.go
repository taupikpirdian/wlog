package hook

import (
	"context"
	"fmt"

	domain "github.com/taupikpirdian/wlog/domain/hook"
)

type Inspector interface {
	Inspect(context.Context) (domain.Location, error)
}
type Planner func(domain.Snapshot) (domain.Plan, error)
type Store interface {
	Apply(context.Context, domain.Location, Planner) (domain.Plan, error)
}
type Result struct {
	Location    domain.Location
	Status      string
	HasOriginal bool
}
type Service struct {
	inspector Inspector
	store     Store
}

func NewService(inspector Inspector, store Store) *Service {
	return &Service{inspector: inspector, store: store}
}

func (s *Service) Install(ctx context.Context) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	location, err := s.inspector.Inspect(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("inspect Git hooks: %w", err)
	}
	if err := domain.ValidateLocation(location); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	plan, err := s.store.Apply(ctx, location, domain.PlanInstall)
	if err != nil {
		return Result{}, fmt.Errorf("install hook %q: %w", location.HookPath, err)
	}
	return Result{Location: location, Status: plan.Status, HasOriginal: plan.HasOriginal}, nil
}
