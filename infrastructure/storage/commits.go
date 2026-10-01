package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	application "github.com/taupikpirdian/wlog/application/activity"
	"github.com/taupikpirdian/wlog/domain/activity"
)

var _ application.CaptureStore = (*SQLiteActivityStore)(nil)

func (s *SQLiteActivityStore) Capture(ctx context.Context, value activity.Commit, resolve application.AttributionResolver) (activity.CaptureResult, error) {
	var result activity.CaptureResult
	err := writeTransaction(ctx, s.db, func(conn *sql.Conn) error {
		err := conn.QueryRowContext(ctx, `SELECT id FROM work_activities WHERE repository=? AND commit_hash=?`, value.Repository, value.Hash).Scan(&result.ID)
		if err == nil {
			result.AlreadyCaptured = true
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		active, err := scanActive(conn.QueryRowContext(ctx, activeSessionQuery))
		if err != nil {
			return err
		}
		result.Attribution = resolve(active)
		var ticketID *int64
		if result.Attribution.TicketKey != "" {
			if _, err := conn.ExecContext(ctx, `INSERT INTO tickets(ticket_key) VALUES (?) ON CONFLICT(ticket_key) DO NOTHING`, result.Attribution.TicketKey); err != nil {
				return err
			}
			var id int64
			if err := conn.QueryRowContext(ctx, `SELECT id FROM tickets WHERE ticket_key=?`, result.Attribution.TicketKey).Scan(&id); err != nil {
				return err
			}
			ticketID = &id
		}
		value.Metadata.TicketSource = result.Attribution.Source
		value.Metadata.Warnings = append(value.Metadata.Warnings, result.Attribution.Warnings...)
		if value.Metadata.Warnings == nil {
			value.Metadata.Warnings = []string{}
		}
		result.Warnings = value.Metadata.Warnings
		metadata, err := json.Marshal(value.Metadata)
		if err != nil {
			return err
		}
		var files any
		if value.Files != nil {
			encoded, err := json.Marshal(value.Files)
			if err != nil {
				return err
			}
			files = string(encoded)
		}
		return conn.QueryRowContext(ctx, `INSERT INTO work_activities(ticket_id,session_id,type,repository,branch,commit_hash,commit_message,changed_files,insertions,deletions,metadata,created_at)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?) RETURNING id`, ticketID, result.Attribution.SessionID, activity.CommitType, value.Repository, value.Branch, value.Hash, value.Message, files, value.Insertions, value.Deletions, string(metadata), value.CreatedAt.UTC().Format(time.RFC3339Nano)).Scan(&result.ID)
	})
	if err != nil {
		return activity.CaptureResult{}, fmt.Errorf("save Git capture: %w", err)
	}
	return result, nil
}
