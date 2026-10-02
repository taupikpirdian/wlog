package dashboard

import (
	"context"
	"time"

	domain "github.com/taupikpirdian/wlog/domain/dashboard"
)

type SnapshotStore interface {
	ReadSnapshot(context.Context) (domain.Snapshot, error)
}
type Reader interface {
	Read(context.Context) (domain.View, error)
}
type service struct {
	store    SnapshotStore
	now      func() time.Time
	location *time.Location
}

func NewService(store SnapshotStore, now func() time.Time, location *time.Location) Reader {
	return &service{store: store, now: now, location: location}
}

func (s *service) Read(ctx context.Context) (domain.View, error) {
	if err := ctx.Err(); err != nil {
		return domain.View{}, err
	}
	now := s.now()
	snapshot, err := s.store.ReadSnapshot(ctx)
	if err != nil {
		return domain.View{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.View{}, err
	}
	return domain.Build(snapshot, now, s.location)
}
