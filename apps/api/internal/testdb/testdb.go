package testdb

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/rachzy/antares/apps/api/internal/store"
)

func Open(t testing.TB) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
