package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	application "github.com/taupikpirdian/wlog/application/session"
	domain "github.com/taupikpirdian/wlog/domain/session"
)

var _ application.ManualStore = (*SQLiteSessionStore)(nil)

func (s *SQLiteSessionStore) InsertManual(ctx context.Context, value domain.Session, observedAt time.Time) (domain.Session, error) {
	err := writeTransaction(ctx, s.db, func(conn *sql.Conn) error {
		existing, err := readDailySessions(ctx, conn)
		if err != nil {
			return err
		}
		if err := domain.CheckManualAvailability(value, existing); err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO tickets(ticket_key) VALUES(?) ON CONFLICT(ticket_key) DO NOTHING`, value.TicketKey); err != nil {
			return err
		}
		if err := conn.QueryRowContext(ctx, `SELECT id FROM tickets WHERE ticket_key=?`, value.TicketKey).Scan(&value.TicketID); err != nil {
			return err
		}
		var end *string
		if value.EndedAt != nil {
			formatted := value.EndedAt.UTC().Format(time.RFC3339Nano)
			end = &formatted
		}
		at := observedAt.UTC().Format(time.RFC3339Nano)
		return conn.QueryRowContext(ctx, `INSERT INTO work_sessions(ticket_id,title,repository,started_at,ended_at,duration_seconds,status,created_at,updated_at)
 VALUES(?,?,?,?,?,?,?,?,?) RETURNING id`, value.TicketID, value.Title, value.Repository, value.StartedAt.UTC().Format(time.RFC3339Nano), end, value.DurationSeconds, value.Status, at, at).Scan(&value.ID)
	})
	if err != nil {
		return domain.Session{}, fmt.Errorf("add manual session: %w", err)
	}
	return value, nil
}
