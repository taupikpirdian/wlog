package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	applicationticket "github.com/taupikpirdian/wlog/application/ticket"
	domainticket "github.com/taupikpirdian/wlog/domain/ticket"
)

const sqliteConstraintUnique = 2067 // SQLITE_CONSTRAINT_UNIQUE

type SQLiteTicketStore struct {
	db *sql.DB
}

func NewSQLiteTicketStore(db *sql.DB) *SQLiteTicketStore {
	return &SQLiteTicketStore{db: db}
}

var _ applicationticket.TicketStore = (*SQLiteTicketStore)(nil)

func (s *SQLiteTicketStore) Create(ctx context.Context, value domainticket.Ticket) (domainticket.Ticket, error) {
	query := `
		INSERT INTO tickets (ticket_key, title)
		VALUES (?, ?)
		RETURNING id, ticket_key, title, created_at, updated_at`
	created, err := scanTicket(s.db.QueryRowContext(ctx, query, value.Key, value.Title))
	if err != nil {
		if isUniqueConstraint(err) {
			return domainticket.Ticket{}, fmt.Errorf("%w: %s", domainticket.ErrAlreadyExists, value.Key)
		}
		return domainticket.Ticket{}, fmt.Errorf("create ticket %q: %w", value.Key, err)
	}
	return created, nil
}

func (s *SQLiteTicketStore) FindByKey(ctx context.Context, key string) (domainticket.Ticket, error) {
	query := `
		SELECT id, ticket_key, title, created_at, updated_at
		FROM tickets
		WHERE ticket_key = ?`
	found, err := scanTicket(s.db.QueryRowContext(ctx, query, key))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainticket.Ticket{}, fmt.Errorf("%w: %s", domainticket.ErrNotFound, key)
		}
		return domainticket.Ticket{}, fmt.Errorf("find ticket %q: %w", key, err)
	}
	return found, nil
}

func (s *SQLiteTicketStore) Upsert(ctx context.Context, value domainticket.Ticket) (domainticket.Ticket, error) {
	query := `
		INSERT INTO tickets (ticket_key, title)
		VALUES (?, ?)
		ON CONFLICT(ticket_key) DO UPDATE SET
			title = CASE
				WHEN excluded.title IS NULL THEN tickets.title
				ELSE excluded.title
			END,
			updated_at = CASE
				WHEN excluded.title IS NOT NULL AND excluded.title IS NOT tickets.title
					THEN CURRENT_TIMESTAMP
				ELSE tickets.updated_at
			END
		RETURNING id, ticket_key, title, created_at, updated_at`
	updated, err := scanTicket(s.db.QueryRowContext(ctx, query, value.Key, value.Title))
	if err != nil {
		return domainticket.Ticket{}, fmt.Errorf("upsert ticket %q: %w", value.Key, err)
	}
	return updated, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanTicket(row rowScanner) (domainticket.Ticket, error) {
	var (
		value     domainticket.Ticket
		title     sql.NullString
		createdAt string
		updatedAt string
	)
	if err := row.Scan(&value.ID, &value.Key, &title, &createdAt, &updatedAt); err != nil {
		return domainticket.Ticket{}, err
	}
	if title.Valid {
		value.Title = &title.String
	}
	createdTime, err := parseSQLiteTimestamp(createdAt)
	if err != nil {
		return domainticket.Ticket{}, fmt.Errorf("parse ticket created_at: %w", err)
	}
	value.CreatedAt = createdTime
	updatedTime, err := parseSQLiteTimestamp(updatedAt)
	if err != nil {
		return domainticket.Ticket{}, fmt.Errorf("parse ticket updated_at: %w", err)
	}
	value.UpdatedAt = updatedTime
	return value, nil
}

func parseSQLiteTimestamp(value string) (time.Time, error) {
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		time.RFC3339Nano,
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported SQLite timestamp %q", value)
}

func isUniqueConstraint(err error) bool {
	var codedError interface{ Code() int }
	return errors.As(err, &codedError) && codedError.Code() == sqliteConstraintUnique
}
