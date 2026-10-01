package session

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrEmptyTitle     = errors.New("session title must not be empty")
	ErrNoActive       = errors.New("no active session; start with wl s <ticket> \"<title>\"")
	ErrConflict       = errors.New("active session changed; retry the command")
	ErrClockBackwards = errors.New("end time precedes start time; correct the device clock and retry")
)

const (
	Active    = "ACTIVE"
	Completed = "COMPLETED"
)

type Session struct {
	ID              int64
	TicketID        int64
	TicketKey       string
	Title           string
	Repository      *string
	StartedAt       time.Time
	EndedAt         *time.Time
	DurationSeconds *int64
	Status          string
}

func ValidateTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", ErrEmptyTitle
	}
	return title, nil
}

func (s Session) Complete(at time.Time) (Session, error) {
	if s.Status != Active {
		return Session{}, ErrConflict
	}
	at = at.UTC()
	if at.Before(s.StartedAt) {
		return Session{}, ErrClockBackwards
	}
	seconds := int64(at.Sub(s.StartedAt) / time.Second)
	s.EndedAt, s.DurationSeconds, s.Status = &at, &seconds, Completed
	return s, nil
}
