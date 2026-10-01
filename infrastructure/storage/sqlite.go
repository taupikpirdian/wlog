package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

const latestSchemaVersion = 1

//go:embed migrations/*.sql
var migrationFiles embed.FS

type SQLiteInitializer struct{}

func NewSQLiteInitializer() *SQLiteInitializer { return &SQLiteInitializer{} }

func (s *SQLiteInitializer) Initialize(ctx context.Context, databasePath string) (resultErr error) {
	if strings.TrimSpace(databasePath) == "" {
		return errors.New("database path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	_, statErr := os.Stat(databasePath)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect database file: %w", statErr)
	}
	newDatabase := errors.Is(statErr, os.ErrNotExist)

	db, err := sql.Open("sqlite", databaseDSN(databasePath))
	if err != nil {
		return fmt.Errorf("open SQLite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer func() {
		if err := db.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close SQLite database: %w", err))
		}
	}()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect to SQLite database: %w", err)
	}
	if newDatabase {
		if err := os.Chmod(databasePath, 0o600); err != nil {
			return fmt.Errorf("secure database file: %w", err)
		}
	}
	if err := migrate(ctx, db); err != nil {
		return fmt.Errorf("migrate SQLite database: %w", err)
	}
	return nil
}

func databaseDSN(path string) string {
	uri := url.URL{Scheme: "file", Path: filepath.Clean(path)}
	return uri.String() + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
}

func migrate(ctx context.Context, db *sql.DB) error {
	var currentVersion int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&currentVersion); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if currentVersion > latestSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", currentVersion, latestSchemaVersion)
	}

	for version := currentVersion + 1; version <= latestSchemaVersion; version++ {
		name := fmt.Sprintf("migrations/%03d_initial.sql", version)
		script, err := migrationFiles.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %d: %w", version, err)
		}
		if err := applyMigration(ctx, db, version, string(script)); err != nil {
			return fmt.Errorf("apply migration %d: %w", version, err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, version int, script string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, statement := range strings.Split(script, ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("execute migration statement: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(version)); err != nil {
		return fmt.Errorf("record schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}
