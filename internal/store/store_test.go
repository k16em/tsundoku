package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "tsundoku.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return s
}

func TestOpenCreatesTheDatabaseFileAndItsParentDirectories(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "dir", "tsundoku.db")

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected database file to exist: %v", err)
	}
}

func TestOpenMigratesANewDatabaseToTheLatestUserVersion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tsundoku.db")

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()

	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != len(migrations) {
		t.Fatalf("user_version = %d, want %d", version, len(migrations))
	}
}

func TestOpeningTheSameDatabaseTwiceDoesNotReapplyMigrations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tsundoku.db")

	first, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	if _, err := first.Add("https://example.com", nil, "2026-09-08T00:00:00Z"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	second, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	defer second.Close()

	var version int
	if err := second.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != len(migrations) {
		t.Fatalf("user_version = %d, want %d", version, len(migrations))
	}

	all, err := second.All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("len(All()) = %d, want 1", len(all))
	}
}

func TestOpenReturnsErrSchemaTooNewWhenUserVersionExceedsKnownMigrations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tsundoku.db")

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := s.db.Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatalf("bump user_version: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err = Open(dbPath)
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("Open() error = %v, want ErrSchemaTooNew", err)
	}
	if !strings.Contains(err.Error(), "999") {
		t.Fatalf("Open() error = %q, want it to mention the database's user_version 999", err)
	}
	if !strings.Contains(err.Error(), strconv.Itoa(len(migrations))) {
		t.Fatalf("Open() error = %q, want it to mention this build's migration count %d", err, len(migrations))
	}
}

func TestForeignKeysStayEnabledAcrossReconnects(t *testing.T) {
	s := newTestStore(t)
	s.db.SetMaxIdleConns(0)

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		conn, err := s.db.Conn(ctx)
		if err != nil {
			t.Fatalf("Conn() #%d error = %v", i, err)
		}

		var enabled int
		if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil {
			t.Fatalf("read foreign_keys #%d: %v", i, err)
		}
		if enabled != 1 {
			t.Fatalf("connection #%d: foreign_keys = %d, want 1", i, enabled)
		}

		if err := conn.Close(); err != nil {
			t.Fatalf("conn.Close() #%d error = %v", i, err)
		}
	}
}

func TestOpenTreatsFilePrefixAsLiteralFilename(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	s, err := Open("file:chosen.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("https://example.com", nil, "2026-09-09T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "file:chosen.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "chosen.db")); !os.IsNotExist(err) {
		t.Fatalf("unexpected alternate database: %v", err)
	}
	s, err = Open("file:chosen.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	all, err := s.All()
	if err != nil || len(all) != 1 {
		t.Fatalf("reopened bookmarks = %v, error = %v", all, err)
	}
}
