package activity

import (
	"context"
	"time"

	"github.com/taupikpirdian/wlog/domain/activity"
	"github.com/taupikpirdian/wlog/domain/session"
)

type NoteStore interface {
	Active(context.Context) (*session.Session, error)
	AddNote(context.Context, activity.Note, session.Session) (activity.Note, error)
}

type Service struct {
	store NoteStore
	now   func() time.Time
}

func NewService(store NoteStore, now func() time.Time) *Service {
	return &Service{store: store, now: now}
}

func (s *Service) AddNote(ctx context.Context, description string) (activity.Note, error) {
	if _, err := activity.ValidateDescription(description); err != nil {
		return activity.Note{}, err
	}
	active, err := s.store.Active(ctx)
	if err != nil {
		return activity.Note{}, err
	}
	if active == nil {
		return activity.Note{}, session.ErrNoActive
	}
	value, err := activity.NewNote(description, *active, s.now())
	if err != nil {
		return activity.Note{}, err
	}
	return s.store.AddNote(ctx, value, *active)
}
