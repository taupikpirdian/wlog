package storage

import (
	"context"
	"database/sql"
	"fmt"

	application "github.com/taupikpirdian/wlog/application/dashboard"
	domain "github.com/taupikpirdian/wlog/domain/dashboard"
	"github.com/taupikpirdian/wlog/domain/session"
)

type SQLiteDashboardStore struct{ db *sql.DB }

func NewSQLiteDashboardStore(db *sql.DB) *SQLiteDashboardStore { return &SQLiteDashboardStore{db: db} }

var _ application.SnapshotStore = (*SQLiteDashboardStore)(nil)

func (s *SQLiteDashboardStore) ReadSnapshot(ctx context.Context) (domain.Snapshot, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("begin daily snapshot: %w", err)
	}
	defer tx.Rollback()
	value, err := readDailySnapshot(ctx, tx)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("read daily snapshot: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.Snapshot{}, fmt.Errorf("finish daily snapshot: %w", err)
	}
	return value, nil
}

type snapshotQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readDailySnapshot(ctx context.Context, q snapshotQuerier) (domain.Snapshot, error) {
	sessions, err := readDailySessions(ctx, q)
	if err != nil {
		return domain.Snapshot{}, err
	}
	activities, err := readDailyActivities(ctx, q)
	if err != nil {
		return domain.Snapshot{}, err
	}
	return domain.Snapshot{Sessions: sessions, Activities: activities}, nil
}

func readDailySessions(ctx context.Context, q snapshotQuerier) (_ []session.Session, resultErr error) {
	rows, err := q.QueryContext(ctx, `SELECT s.id,s.ticket_id,t.ticket_key,s.title,s.started_at,s.ended_at,s.status,s.repository
 FROM work_sessions s JOIN tickets t ON t.id=s.ticket_id`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); resultErr == nil {
			resultErr = err
		}
	}()
	var values []session.Session
	for rows.Next() {
		var value session.Session
		var start string
		var end, repository sql.NullString
		if err := rows.Scan(&value.ID, &value.TicketID, &value.TicketKey, &value.Title, &start, &end, &value.Status, &repository); err != nil {
			return nil, err
		}
		if repository.Valid {
			value.Repository = &repository.String
		}
		value.StartedAt, err = parseSQLiteTimestamp(start)
		if err != nil {
			return nil, fmt.Errorf("session %d started_at: %w", value.ID, err)
		}
		if end.Valid {
			at, err := parseSQLiteTimestamp(end.String)
			if err != nil {
				return nil, fmt.Errorf("session %d ended_at: %w", value.ID, err)
			}
			value.EndedAt = &at
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func readDailyActivities(ctx context.Context, q snapshotQuerier) (_ []domain.Activity, resultErr error) {
	rows, err := q.QueryContext(ctx, `SELECT a.id,t.ticket_key,a.session_id,a.type,
 CASE WHEN a.type='NOTE' THEN a.description ELSE a.commit_message END,a.commit_hash,a.created_at,COALESCE(a.repository,s.repository)
 FROM work_activities a LEFT JOIN tickets t ON t.id=a.ticket_id
 LEFT JOIN work_sessions s ON s.id=a.session_id`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); resultErr == nil {
			resultErr = err
		}
	}()
	var values []domain.Activity
	for rows.Next() {
		var value domain.Activity
		var key, text, hash, repository sql.NullString
		var sid sql.NullInt64
		var at string
		if err := rows.Scan(&value.ID, &key, &sid, &value.Type, &text, &hash, &at, &repository); err != nil {
			return nil, err
		}
		value.TicketKey, value.Text, value.Hash = key.String, text.String, hash.String
		value.Repository = repository.String
		if sid.Valid {
			id := sid.Int64
			value.SessionID = &id
		}
		value.At, err = parseSQLiteTimestamp(at)
		if err != nil {
			return nil, fmt.Errorf("activity %d created_at: %w", value.ID, err)
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
