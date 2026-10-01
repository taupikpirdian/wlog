package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	application "github.com/taupikpirdian/wlog/application/session"
	domain "github.com/taupikpirdian/wlog/domain/session"
)

type SQLiteSessionStore struct{ db *sql.DB }

func NewSQLiteSessionStore(db *sql.DB) *SQLiteSessionStore { return &SQLiteSessionStore{db: db} }

var _ application.Store = (*SQLiteSessionStore)(nil)

const activeSessionQuery = `SELECT s.id, s.ticket_id, t.ticket_key, s.title, s.repository, s.started_at, s.status
 FROM work_sessions s JOIN tickets t ON t.id=s.ticket_id WHERE s.status='ACTIVE'`

func (s *SQLiteSessionStore) Active(ctx context.Context) (*domain.Session, error) {
	return scanActive(s.db.QueryRowContext(ctx, activeSessionQuery))
}

func scanActive(row rowScanner) (*domain.Session, error) {
	var value domain.Session
	var repository sql.NullString
	var started string
	if err := row.Scan(&value.ID, &value.TicketID, &value.TicketKey, &value.Title, &repository, &started, &value.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("read active session: %w", err)
	}
	at, err := parseSQLiteTimestamp(started)
	if err != nil {
		return nil, err
	}
	value.StartedAt = at
	if repository.Valid {
		value.Repository = &repository.String
	}
	return &value, nil
}

// BEGIN IMMEDIATE serializes writers before reading the expected active session.
// No connection or transaction is held while delivery prompts for confirmation.
func (s *SQLiteSessionStore) write(ctx context.Context, operation func(*sql.Conn) error) (resultErr error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, conn.Close()) }()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin session transaction: %w", err)
	}
	defer func() {
		// Cleanup must still run when the command context is cancelled.
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if resultErr != nil {
			_, rollbackErr := conn.ExecContext(cleanup, "ROLLBACK")
			resultErr = errors.Join(resultErr, rollbackErr)
		}
	}()
	if err := operation(conn); err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, "COMMIT")
	return err
}

func (s *SQLiteSessionStore) Start(ctx context.Context, value domain.Session, previous *domain.Session) (domain.Session, error) {
	err := s.write(ctx, func(conn *sql.Conn) error {
		active, err := scanActive(conn.QueryRowContext(ctx, activeSessionQuery))
		if err != nil {
			return err
		}
		if previous == nil {
			if active != nil {
				return domain.ErrConflict
			}
		} else {
			if active == nil || active.ID != previous.ID || !active.StartedAt.Equal(previous.StartedAt) {
				return domain.ErrConflict
			}
			if err := completeSession(ctx, conn, *previous); err != nil {
				return err
			}
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO tickets(ticket_key) VALUES (?) ON CONFLICT(ticket_key) DO NOTHING`, value.TicketKey); err != nil {
			return err
		}
		if err := conn.QueryRowContext(ctx, `SELECT id FROM tickets WHERE ticket_key=?`, value.TicketKey).Scan(&value.TicketID); err != nil {
			return err
		}
		err = conn.QueryRowContext(ctx, `INSERT INTO work_sessions(ticket_id,title,repository,started_at,status,created_at,updated_at)
 VALUES (?,?,?,?,'ACTIVE',?,?) RETURNING id`, value.TicketID, value.Title, value.Repository,
			value.StartedAt.UTC().Format(time.RFC3339Nano), value.StartedAt.UTC().Format(time.RFC3339Nano), value.StartedAt.UTC().Format(time.RFC3339Nano)).Scan(&value.ID)
		return err
	})
	if err != nil {
		return domain.Session{}, fmt.Errorf("start session: %w", err)
	}
	return value, nil
}

func completeSession(ctx context.Context, conn *sql.Conn, value domain.Session) error {
	result, err := conn.ExecContext(ctx, `UPDATE work_sessions SET ended_at=?,duration_seconds=?,status='COMPLETED',updated_at=? WHERE id=? AND status='ACTIVE'`,
		value.EndedAt.UTC().Format(time.RFC3339Nano), *value.DurationSeconds, value.EndedAt.UTC().Format(time.RFC3339Nano), value.ID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return domain.ErrConflict
	}
	return nil
}

func (s *SQLiteSessionStore) Complete(ctx context.Context, value domain.Session) error {
	return s.write(ctx, func(conn *sql.Conn) error { return completeSession(ctx, conn, value) })
}
