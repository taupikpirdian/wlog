//go:build !unix

package githook

import (
	"context"
	"errors"

	application "github.com/taupikpirdian/wlog/application/hook"
	domain "github.com/taupikpirdian/wlog/domain/hook"
)

var ErrUnsupportedPlatform = errors.New("automatic Git hook installation requires macOS or Linux; use wl git for manual capture or run wl inside WSL")

type Inspector struct{}

func NewInspector(string) *Inspector { return &Inspector{} }

func (*Inspector) Inspect(context.Context) (domain.Location, error) {
	return domain.Location{}, ErrUnsupportedPlatform
}

type Store struct{}

func NewStore() *Store { return &Store{} }

func (*Store) Read(context.Context, domain.Location) (domain.Snapshot, error) {
	return domain.Snapshot{}, ErrUnsupportedPlatform
}

func (*Store) Apply(context.Context, domain.Location, application.Planner) (domain.Plan, error) {
	return domain.Plan{}, ErrUnsupportedPlatform
}

var _ application.Inspector = (*Inspector)(nil)
var _ application.Store = (*Store)(nil)
var _ application.SnapshotReader = (*Store)(nil)
