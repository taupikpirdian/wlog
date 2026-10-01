package activity

import (
	"errors"
	"strings"
	"time"

	"github.com/taupikpirdian/wlog/domain/session"
)

var (
	ErrEmptyDescription   = errors.New("note description must not be empty")
	ErrClockBeforeSession = errors.New("note time precedes session start; correct the device clock and retry")
)

const NoteType = "NOTE"

type Note struct {
	ID          int64
	TicketID    int64
	TicketKey   string
	SessionID   int64
	Description string
	Repository  *string
	CreatedAt   time.Time
}

func ValidateDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return "", ErrEmptyDescription
	}
	return description, nil
}

func NewNote(description string, active session.Session, at time.Time) (Note, error) {
	description, err := ValidateDescription(description)
	if err != nil {
		return Note{}, err
	}
	if active.Status != session.Active {
		return Note{}, session.ErrConflict
	}
	at = at.UTC()
	if at.Before(active.StartedAt) {
		return Note{}, ErrClockBeforeSession
	}
	value := Note{TicketID: active.TicketID, TicketKey: active.TicketKey, SessionID: active.ID, Description: description, CreatedAt: at}
	if active.Repository != nil {
		repository := *active.Repository
		value.Repository = &repository
	}
	return value, nil
}
