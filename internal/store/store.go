package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"repoharbor/internal/store/db"
)

// Store owns the SQLite connection and generated query set.
type Store struct {
	DB *sql.DB
	Q  *db.Queries
}

// Open opens SQLite and applies all embedded goose migrations.
func Open(ctx context.Context, path string, migrations embed.FS) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return &Store{DB: sqlDB, Q: db.New(sqlDB)}, nil
}

func (s *Store) Close() error { return s.DB.Close() }

// Now returns the canonical UTC RFC3339 timestamp used across the database.
func Now() string { return time.Now().UTC().Format(time.RFC3339) }
