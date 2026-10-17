package store_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/rachzy/antares/apps/api/internal/store"
	"github.com/rachzy/antares/apps/api/internal/testdb"
)

func insertTarget(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO targets (catalog, catalog_id, ra_deg, dec_deg, display_name, designation, created_at)
		VALUES ('KIC', '8120608', 298.65, 43.95, 'Kepler-186', 'KIC 8120608', ?)`, store.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func insertJob(db *sql.DB, id, key, state string, targetID int64) error {
	_, err := db.Exec(`INSERT INTO jobs (id, dedupe_key, kind, state, target_id, request_json, created_at)
		VALUES (?, ?, 'extract', ?, ?, '{}', ?)`, id, key, state, targetID, store.FormatTime(time.Now()))
	return err
}

func TestMigrateCreatesTheSchema(t *testing.T) {
	db := testdb.Open(t)
	for _, name := range []string{
		"targets", "target_aliases", "feature_cache", "prediction_cache",
		"jobs", "subscriptions", "job_milestones",
	} {
		var n int
		err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n)
		if err != nil || n != 1 {
			t.Errorf("table %s: count=%d err=%v", name, n, err)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := testdb.Open(t)
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("schema_migrations rows = %d (err %v), want 1", n, err)
	}
}

func TestPragmasAreApplied(t *testing.T) {
	db := testdb.Open(t)
	for pragma, want := range map[string]string{
		"journal_mode": "wal",
		"foreign_keys": "1",
		"busy_timeout": "5000",
	} {
		var got string
		if err := db.QueryRow("PRAGMA " + pragma).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("PRAGMA %s = %q, want %q", pragma, got, want)
		}
	}
}

func TestForeignKeysAreEnforced(t *testing.T) {
	db := testdb.Open(t)
	_, err := db.Exec(`INSERT INTO subscriptions (token_hash, job_id, created_at, expires_at)
		VALUES ('h', 'no-such-job', 'x', 'y')`)
	if err == nil || !strings.Contains(strings.ToUpper(err.Error()), "FOREIGN KEY") {
		t.Fatalf("want a foreign key error, got %v", err)
	}
}

func TestOnlyOneActiveJobPerDedupeKey(t *testing.T) {
	db := testdb.Open(t)
	target := insertTarget(t, db)
	if err := insertJob(db, "a", "key", "queued", target); err != nil {
		t.Fatal(err)
	}
	if err := insertJob(db, "b", "key", "running", target); err == nil {
		t.Fatal("a second active job with the same key was accepted")
	}
	if _, err := db.Exec(`UPDATE jobs SET state = 'done' WHERE id = 'a'`); err != nil {
		t.Fatal(err)
	}
	if err := insertJob(db, "c", "key", "queued", target); err != nil {
		t.Fatalf("a new job after the first finished must be allowed: %v", err)
	}
}

func TestTimeLayoutSortsLexicographically(t *testing.T) {
	whole := time.Date(2026, 10, 7, 12, 0, 5, 0, time.UTC)
	fractional := whole.Add(500 * time.Millisecond)
	if !(store.FormatTime(whole) < store.FormatTime(fractional)) {
		t.Fatalf("%q must sort before %q", store.FormatTime(whole), store.FormatTime(fractional))
	}
	back, err := store.ParseTime(store.FormatTime(fractional))
	if err != nil || !back.Equal(fractional) {
		t.Fatalf("round trip = %v, %v", back, err)
	}
}
