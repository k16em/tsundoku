package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-sqlite3"
)

// ErrNotFound is returned when a bookmark id does not exist.
var ErrNotFound = errors.New("bookmark not found")

// ErrSchemaTooNew is returned when the database was written by a newer build.
var ErrSchemaTooNew = errors.New("database schema is newer than this build supports")

const driverName = "sqlite3_tsundoku"

var pragmas = []string{
	"PRAGMA journal_mode = WAL",
	"PRAGMA foreign_keys = ON",
	"PRAGMA busy_timeout = 5000",
}

func init() {
	sql.Register(driverName, &sqlite3.SQLiteDriver{
		ConnectHook: func(c *sqlite3.SQLiteConn) error {
			for _, pragma := range pragmas {
				if _, err := c.Exec(pragma, nil); err != nil {
					return err
				}
			}
			return nil
		},
	})
}

const schemaV1 = `
CREATE TABLE bookmarks (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    url        TEXT NOT NULL UNIQUE,
    read       INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE tags (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE bookmark_tags (
    bookmark_id INTEGER NOT NULL REFERENCES bookmarks(id) ON DELETE CASCADE,
    tag_id      INTEGER NOT NULL REFERENCES tags(id)      ON DELETE CASCADE,
    PRIMARY KEY (bookmark_id, tag_id)
);

CREATE INDEX idx_bookmark_tags_tag ON bookmark_tags(tag_id);
CREATE INDEX idx_bookmarks_created ON bookmarks(created_at);
CREATE INDEX idx_bookmarks_read    ON bookmarks(read);
`

var migrations = []string{
	schemaV1,
}

// Store owns the database connection.
type Store struct {
	db *sql.DB
}

// Open connects to the database at path and migrates it to the latest schema.
func Open(path string) (*Store, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory %q: %w", dir, err)
		}
	}

	db, err := sql.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("open database %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	if version > len(migrations) {
		return fmt.Errorf("%w: database schema version %d is newer than this build supports (%d)", ErrSchemaTooNew, version, len(migrations))
	}

	for i := version; i < len(migrations); i++ {
		if err := applyMigration(db, i); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(db *sql.DB, index int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", index+1, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(migrations[index]); err != nil {
		return fmt.Errorf("apply migration %d: %w", index+1, err)
	}

	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", index+1)); err != nil {
		return fmt.Errorf("set schema version to %d: %w", index+1, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", index+1, err)
	}

	return nil
}

func placeholders(n int) string {
	if n == 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func int64Args(ids []int64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}
