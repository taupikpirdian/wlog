package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	application "github.com/taupikpirdian/wlog/application/activity"
	"github.com/taupikpirdian/wlog/domain/activity"
	"github.com/taupikpirdian/wlog/domain/session"
)

type SQLiteActivityStore struct{ db *sql.DB }

func NewSQLiteActivityStore(db *sql.DB) *SQLiteActivityStore { return &SQLiteActivityStore{db: db} }

var _ application.NoteStore = (*SQLiteActivityStore)(nil)

func (s *SQLiteActivityStore) Active(ctx context.Context) (*session.Session, error) {
	return scanActive(s.db.QueryRowContext(ctx, activeSessionQuery))
}

func (s *SQLiteActivityStore) AddNote(ctx context.Context, value activity.Note, expected session.Session) (activity.Note, error) {
	err := writeTransaction(ctx, s.db, func(conn *sql.Conn) error {
		active, err := scanActive(conn.QueryRowContext(ctx, activeSessionQuery))
		if err != nil {
			return err
		}
		if active == nil || active.ID != expected.ID || active.TicketID != expected.TicketID || !active.StartedAt.Equal(expected.StartedAt) {
			return session.ErrConflict
		}
		// Rebuild from the verified session so persistence cannot mix ticket/session context.
		value, err = activity.NewNote(value.Description, *active, value.CreatedAt)
		if err != nil {
			return err
		}
		return conn.QueryRowContext(ctx, `INSERT INTO work_activities(ticket_id,session_id,type,description,repository,created_at)
 VALUES (?,?,?,?,?,?) RETURNING id`, value.TicketID, value.SessionID, activity.NoteType, value.Description, value.Repository,
			value.CreatedAt.UTC().Format(time.RFC3339Nano)).Scan(&value.ID)
	})
	if err != nil {
		return activity.Note{}, fmt.Errorf("add note: %w", err)
	}
	return value, nil
}
