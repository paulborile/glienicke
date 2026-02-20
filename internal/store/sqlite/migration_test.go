package sqlite

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationFromFreshDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := New(dbPath)
	require.NoError(t, err)
	defer store.Close()

	// Verify migrations table exists
	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 3, count, "should have applied 3 migrations")

	// Verify events table exists
	err = store.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	assert.NoError(t, err)

	// Verify channel_events table exists (migration 3)
	err = store.db.QueryRow("SELECT COUNT(*) FROM channel_events").Scan(&count)
	assert.NoError(t, err)
}

func TestMigrationIdempotency(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create store first time
	store1, err := New(dbPath)
	require.NoError(t, err)
	store1.Close()

	// Create store second time (simulating restart)
	store2, err := New(dbPath)
	require.NoError(t, err)
	defer store2.Close()

	// Should still have only 3 migrations applied
	var count int
	err = store2.db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 3, count, "migrations should not be re-applied")
}

func TestMigrationFromOldDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create old-style database with only events table (simulating pre-migration DB)
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			pubkey TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			kind INTEGER NOT NULL,
			tags TEXT,
			content TEXT NOT NULL,
			sig TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_events_pubkey ON events(pubkey);
	`)
	require.NoError(t, err)

	// Close the raw DB
	db.Close()

	// Now open with our store - should run migrations
	store, err := New(dbPath)
	require.NoError(t, err)
	defer store.Close()

	// Verify migrations were applied
	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 3, count, "should have applied missing migrations")

	// Verify channel_events table was created
	err = store.db.QueryRow("SELECT COUNT(*) FROM channel_events").Scan(&count)
	assert.NoError(t, err)

	// Verify old data still exists
	err = store.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	assert.NoError(t, err)
}
