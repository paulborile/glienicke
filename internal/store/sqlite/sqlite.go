package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/storage"
)

// Options holds database configuration options
type Options struct {
	// MaxOpenConns is the maximum number of open connections to the database.
	// If MaxOpenConns is 0 or negative, there is no limit.
	MaxOpenConns int

	// MaxIdleConns is the maximum number of idle connections to the database.
	// If MaxIdleConns is negative, no idle connections are retained.
	MaxIdleConns int

	// ConnMaxLifetime sets the maximum duration of time that a database
	// connection may be reused.
	// If ConnMaxLifetime is 0, connections are reused forever.
	ConnMaxLifetime time.Duration

	// EnableWAL enables Write-Ahead Logging mode for better concurrency.
	// Recommended for production use.
	EnableWAL bool

	// CacheSize sets the database cache size in pages.
	// Negative values mean the default size (usually 2000).
	// Value is in KB (e.g., -2000 = 2MB cache).
	CacheSize int

	// BusyTimeout sets the busy timeout in milliseconds.
	// Default is 5000ms (5 seconds).
	BusyTimeout time.Duration
}

// DefaultOptions returns default database options
func DefaultOptions() *Options {
	return &Options{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		EnableWAL:       true,
		CacheSize:       -2000, // 2MB cache
		BusyTimeout:     5 * time.Second,
	}
}

// Store is a SQLite implementation of storage.Store
type Store struct {
	db *sql.DB
}

// Ensure Store implements storage.Store
var _ storage.Store = (*Store)(nil)

// New creates a new SQLite store with autoconfiguration
func New(dbPath string) (*Store, error) {
	return NewWithOptions(dbPath, DefaultOptions())
}

// NewWithOptions creates a new SQLite store with custom options
func NewWithOptions(dbPath string, opts *Options) (*Store, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	dsn := buildDSN(dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: db}

	// Apply performance settings
	if err := store.configurePerformance(opts); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure performance: %w", err)
	}

	// Configure connection pool
	if opts.MaxOpenConns > 0 {
		db.SetMaxOpenConns(opts.MaxOpenConns)
	}
	if opts.MaxIdleConns >= 0 {
		db.SetMaxIdleConns(opts.MaxIdleConns)
	}
	if opts.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(opts.ConnMaxLifetime)
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// buildDSN builds the SQLite DSN with appropriate settings
func buildDSN(dbPath string) string {
	return dbPath
}

// configurePerformance applies performance optimizations
func (s *Store) configurePerformance(opts *Options) error {
	// Enable WAL mode for better concurrency (recommended for production)
	if opts.EnableWAL {
		if _, err := s.db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
			return fmt.Errorf("failed to enable WAL mode: %w", err)
		}
	}

	// Set cache size (negative value = KB)
	if opts.CacheSize != 0 {
		if _, err := s.db.Exec(fmt.Sprintf("PRAGMA cache_size=%d;", opts.CacheSize)); err != nil {
			return fmt.Errorf("failed to set cache size: %w", err)
		}
	}

	// Set busy timeout
	if opts.BusyTimeout > 0 {
		timeoutMs := int(opts.BusyTimeout.Milliseconds())
		if _, err := s.db.Exec(fmt.Sprintf("PRAGMA busy_timeout=%d;", timeoutMs)); err != nil {
			return fmt.Errorf("failed to set busy timeout: %w", err)
		}
	}

	// Enable foreign keys
	if _, err := s.db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Synchronous mode: NORMAL is good balance of safety and performance
	// For maximum performance in non-critical apps, can use OFF
	// For maximum safety, use FULL
	if _, err := s.db.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		return fmt.Errorf("failed to set synchronous mode: %w", err)
	}

	// Temp store: MEMORY for better performance
	if _, err := s.db.Exec("PRAGMA temp_store=MEMORY;"); err != nil {
		return fmt.Errorf("failed to set temp store: %w", err)
	}

	return nil
}

// initSchema creates the necessary tables if they don't exist
func (s *Store) initSchema() error {
	// Create migrations table if not exists
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Run migrations
	if err := s.runMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

type migration struct {
	version int
	sql     string
}

var migrations = []migration{
	{
		version: 1,
		sql: `
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
		CREATE INDEX IF NOT EXISTS idx_events_kind ON events(kind);
		CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at);
		CREATE INDEX IF NOT EXISTS idx_events_kind_created_at ON events(kind, created_at);
		`,
	},
	{
		version: 2,
		sql: `
		CREATE TABLE IF NOT EXISTS deleted_events (
			id TEXT PRIMARY KEY,
			deleter_pubkey TEXT NOT NULL,
			deleted_at INTEGER NOT NULL
		);
		`,
	},
	{
		version: 3,
		sql: `
		CREATE TABLE IF NOT EXISTS channel_events (
			id TEXT PRIMARY KEY,
			channel_id TEXT NOT NULL,
			pubkey TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			kind INTEGER NOT NULL,
			tags TEXT,
			content TEXT NOT NULL,
			sig TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_channel_events_channel_id ON channel_events(channel_id);
		CREATE INDEX IF NOT EXISTS idx_channel_events_kind ON channel_events(kind);
		CREATE INDEX IF NOT EXISTS idx_channel_events_created_at ON channel_events(created_at);
		CREATE INDEX IF NOT EXISTS idx_channel_events_channel_created ON channel_events(channel_id, created_at);
		`,
	},
}

func (s *Store) runMigrations() error {
	for _, m := range migrations {
		// Check if already applied
		var count int
		err := s.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", m.version).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration %d: %w", m.version, err)
		}

		if count > 0 {
			continue // Already applied
		}

		// Apply migration
		_, err = s.db.Exec(m.sql)
		if err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", m.version, err)
		}

		// Record migration
		_, err = s.db.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", m.version, time.Now().Unix())
		if err != nil {
			return fmt.Errorf("failed to record migration %d: %w", m.version, err)
		}
	}

	return nil
}

// SaveEvent stores an event in SQLite
func (s *Store) SaveEvent(ctx context.Context, evt *event.Event) error {
	// Check if event is deleted
	var deleted bool
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM deleted_events WHERE id = ?", evt.ID).Scan(&deleted)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check deletion status: %w", err)
	}
	if deleted {
		return fmt.Errorf("event has been deleted")
	}

	// Insert or replace event
	query := `
	INSERT OR REPLACE INTO events (id, pubkey, created_at, kind, tags, content, sig)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	tagsJSON := "[]"
	if len(evt.Tags) > 0 {
		// Convert tags to JSON array string
		var tagStrings []string
		for _, tag := range evt.Tags {
			tagStr := "["
			for i, part := range tag {
				if i > 0 {
					tagStr += ","
				}
				tagStr += `"` + strings.ReplaceAll(part, `"`, `\"`) + `"`
			}
			tagStr += "]"
			tagStrings = append(tagStrings, tagStr)
		}
		tagsJSON = "[" + strings.Join(tagStrings, ",") + "]"
	}

	_, err = s.db.ExecContext(ctx, query, evt.ID, evt.PubKey, evt.CreatedAt, evt.Kind, tagsJSON, evt.Content, evt.Sig)
	if err != nil {
		return fmt.Errorf("failed to save event: %w", err)
	}

	return nil
}

// SaveEvents stores multiple events in a batch using a transaction
func (s *Store) SaveEvents(ctx context.Context, events []*event.Event) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO events (id, pubkey, created_at, kind, tags, content, sig)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, evt := range events {
		// Check if event is deleted
		var deleted bool
		err := tx.QueryRowContext(ctx, "SELECT 1 FROM deleted_events WHERE id = ?", evt.ID).Scan(&deleted)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to check deletion status: %w", err)
		}
		if deleted {
			continue // Skip deleted events
		}

		tagsJSON := "[]"
		if len(evt.Tags) > 0 {
			var tagStrings []string
			for _, tag := range evt.Tags {
				tagStr := "["
				for i, part := range tag {
					if i > 0 {
						tagStr += ","
					}
					tagStr += `"` + strings.ReplaceAll(part, `"`, `\"`) + `"`
				}
				tagStr += "]"
				tagStrings = append(tagStrings, tagStr)
			}
			tagsJSON = "[" + strings.Join(tagStrings, ",") + "]"
		}

		if _, err := stmt.ExecContext(ctx, evt.ID, evt.PubKey, evt.CreatedAt, evt.Kind, tagsJSON, evt.Content, evt.Sig); err != nil {
			return fmt.Errorf("failed to save event %s: %w", evt.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// QueryEvents retrieves events matching the filters
func (s *Store) QueryEvents(ctx context.Context, filters []*event.Filter) ([]*event.Event, error) {
	if len(filters) == 0 {
		return []*event.Event{}, nil
	}

	var results []*event.Event
	seen := make(map[string]bool)

	// Process each filter (OR'd together)
	for _, filter := range filters {
		events, err := s.queryFilter(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to query filter: %w", err)
		}

		for _, evt := range events {
			if !seen[evt.ID] {
				results = append(results, evt)
				seen[evt.ID] = true
			}
		}
	}

	// Sort by created_at descending (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt > results[j].CreatedAt
	})

	// Apply limit if specified (use first filter's limit)
	if len(filters) > 0 && filters[0].Limit != nil {
		limit := *filters[0].Limit
		if len(results) > limit {
			results = results[:limit]
		}
	}

	return results, nil
}

// queryFilter builds and executes a query for a single filter
func (s *Store) queryFilter(ctx context.Context, filter *event.Filter) ([]*event.Event, error) {
	var conditions []string
	var args []interface{}

	// Build WHERE clause
	if filter.IDs != nil {
		placeholders := make([]string, len(filter.IDs))
		for i, id := range filter.IDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(conditions, "id IN ("+strings.Join(placeholders, ",")+")")
	}

	if filter.Authors != nil {
		placeholders := make([]string, len(filter.Authors))
		for i, author := range filter.Authors {
			placeholders[i] = "?"
			args = append(args, author)
		}
		conditions = append(conditions, "pubkey IN ("+strings.Join(placeholders, ",")+")")
	}

	if filter.Kinds != nil {
		placeholders := make([]string, len(filter.Kinds))
		for i, kind := range filter.Kinds {
			placeholders[i] = "?"
			args = append(args, kind)
		}
		conditions = append(conditions, "kind IN ("+strings.Join(placeholders, ",")+")")
	}

	if filter.Since != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, *filter.Since)
	}

	if filter.Until != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, *filter.Until)
	}

	// Build base query
	query := "SELECT id, pubkey, created_at, kind, tags, content, sig FROM events"

	// Add WHERE clause if we have conditions
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Add ORDER BY
	query += " ORDER BY created_at DESC"

	// Add LIMIT if specified
	if filter.Limit != nil {
		query += " LIMIT ?"
		args = append(args, *filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	events := make([]*event.Event, 0)
	for rows.Next() {
		evt := &event.Event{
			Tags: [][]string{}, // Initialize to prevent null
		}
		var tagsJSON sql.NullString

		err := rows.Scan(&evt.ID, &evt.PubKey, &evt.CreatedAt, &evt.Kind, &tagsJSON, &evt.Content, &evt.Sig)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse tags JSON if present
		if tagsJSON.Valid && tagsJSON.String != "" && tagsJSON.String != "[]" {
			// Simple JSON parsing for tags array
			// This is a basic implementation - in production you might want a proper JSON parser
			parsedTags := parseTagsJSON(tagsJSON.String)
			if parsedTags != nil {
				evt.Tags = parsedTags
			} else {
				evt.Tags = [][]string{}
			}
		}

		events = append(events, evt)
	}

	return events, rows.Err()
}

// parseTagsJSON is a simple JSON parser for Nostr tags arrays
// This is a basic implementation for the specific format we use
func parseTagsJSON(jsonStr string) [][]string {
	// Remove outer brackets
	jsonStr = strings.TrimSpace(jsonStr)
	if !strings.HasPrefix(jsonStr, "[") || !strings.HasSuffix(jsonStr, "]") {
		return [][]string{}
	}
	jsonStr = jsonStr[1 : len(jsonStr)-1]

	var tags [][]string
	if jsonStr == "" {
		return tags
	}

	// Split by top-level commas (simplified)
	var depth int
	var current strings.Builder
	for _, r := range jsonStr {
		switch r {
		case '[':
			depth++
			current.WriteRune(r)
		case ']':
			depth--
			current.WriteRune(r)
		case ',':
			if depth == 0 {
				tagStr := strings.TrimSpace(current.String())
				if tagStr != "" {
					tag := parseTagString(tagStr)
					tags = append(tags, tag)
				}
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}

	// Add the last tag
	if current.Len() > 0 {
		tagStr := strings.TrimSpace(current.String())
		if tagStr != "" {
			tag := parseTagString(tagStr)
			tags = append(tags, tag)
		}
	}

	return tags
}

// parseTagString parses a single tag like ["a","b","c"]
func parseTagString(tagStr string) []string {
	tagStr = strings.TrimSpace(tagStr)
	if !strings.HasPrefix(tagStr, "[") || !strings.HasSuffix(tagStr, "]") {
		return nil
	}
	tagStr = tagStr[1 : len(tagStr)-1]

	if tagStr == "" {
		return []string{}
	}

	// Split by commas and clean up quotes
	parts := strings.Split(tagStr, ",")
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, `"`) && strings.HasSuffix(part, `"`) {
			part = part[1 : len(part)-1]
			// Unescape quotes
			part = strings.ReplaceAll(part, `\"`, `"`)
		}
		result = append(result, part)
	}

	return result
}

// DeleteEvent marks an event as deleted
func (s *Store) DeleteEvent(ctx context.Context, eventID string, deleterPubKey string) error {
	// Check if event exists and get author
	var author string
	err := s.db.QueryRowContext(ctx, "SELECT pubkey FROM events WHERE id = ?", eventID).Scan(&author)
	if err != nil {
		if err == sql.ErrNoRows {
			return storage.ErrNotFound
		}
		return fmt.Errorf("failed to check event: %w", err)
	}

	// Verify deletion authorization (only author can delete)
	if author != deleterPubKey {
		return fmt.Errorf("unauthorized: only event author can delete")
	}

	// Mark as deleted
	_, err = s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO deleted_events (id, deleter_pubkey, deleted_at) VALUES (?, ?, ?)",
		eventID, deleterPubKey, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to mark event as deleted: %w", err)
	}

	return nil
}

// DeleteAllEventsByPubKey deletes all events from a specific pubkey (NIP-62)
func (s *Store) DeleteAllEventsByPubKey(ctx context.Context, pubkey string) error {
	// Get all event IDs from the pubkey
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM events WHERE pubkey = ?", pubkey)
	if err != nil {
		return fmt.Errorf("failed to query events for pubkey: %w", err)
	}
	defer rows.Close()

	var eventIDs []string
	for rows.Next() {
		var eventID string
		if err := rows.Scan(&eventID); err != nil {
			return fmt.Errorf("failed to scan event ID: %w", err)
		}
		eventIDs = append(eventIDs, eventID)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating events: %w", err)
	}

	// Mark all events as deleted
	if len(eventIDs) > 0 {
		// This is a bit tricky with SQLite, so we'll do individual inserts
		for _, eventID := range eventIDs {
			_, err := s.db.ExecContext(ctx,
				"INSERT OR IGNORE INTO deleted_events (id, deleter_pubkey, deleted_at) VALUES (?, ?, ?)",
				eventID, pubkey, time.Now().Unix())
			if err != nil {
				return fmt.Errorf("failed to mark event %s as deleted: %w", eventID, err)
			}
		}
	}

	return nil
}

// GetEvent retrieves a single event by ID
func (s *Store) GetEvent(ctx context.Context, eventID string) (*event.Event, error) {
	// Check if event is deleted
	var deleted bool
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM deleted_events WHERE id = ?", eventID).Scan(&deleted)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check deletion status: %w", err)
	}
	if deleted {
		return nil, fmt.Errorf("event has been deleted")
	}

	// Get event
	evt := &event.Event{
		Tags: [][]string{}, // Initialize to prevent null
	}
	var tagsJSON sql.NullString

	err = s.db.QueryRowContext(ctx,
		"SELECT id, pubkey, created_at, kind, tags, content, sig FROM events WHERE id = ?",
		eventID).Scan(&evt.ID, &evt.PubKey, &evt.CreatedAt, &evt.Kind, &tagsJSON, &evt.Content, &evt.Sig)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	// Parse tags JSON if present
	if tagsJSON.Valid && tagsJSON.String != "" && tagsJSON.String != "[]" {
		parsedTags := parseTagsJSON(tagsJSON.String)
		if parsedTags != nil {
			evt.Tags = parsedTags
		} else {
			evt.Tags = [][]string{}
		}
	}

	return evt, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// CountEvents returns the count of events matching the filters
func (s *Store) CountEvents(ctx context.Context, filters []*event.Filter) (int, error) {
	if len(filters) == 0 {
		return 0, nil
	}

	var totalCount int
	seen := make(map[string]bool)

	// Process each filter (OR'd together)
	for _, filter := range filters {
		count, err := s.countFilter(ctx, filter, seen)
		if err != nil {
			return 0, fmt.Errorf("failed to count filter: %w", err)
		}
		totalCount += count
	}

	return totalCount, nil
}

// countFilter builds and executes a count query for a single filter
func (s *Store) countFilter(ctx context.Context, filter *event.Filter, seen map[string]bool) (int, error) {
	var conditions []string
	var args []interface{}

	// Build WHERE clause (similar to queryFilter)
	if filter.IDs != nil {
		placeholders := make([]string, len(filter.IDs))
		for i, id := range filter.IDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(conditions, "id IN ("+strings.Join(placeholders, ",")+")")
	}

	if filter.Authors != nil {
		placeholders := make([]string, len(filter.Authors))
		for i, author := range filter.Authors {
			placeholders[i] = "?"
			args = append(args, author)
		}
		conditions = append(conditions, "pubkey IN ("+strings.Join(placeholders, ",")+")")
	}

	if filter.Kinds != nil {
		placeholders := make([]string, len(filter.Kinds))
		for i, kind := range filter.Kinds {
			placeholders[i] = "?"
			args = append(args, kind)
		}
		conditions = append(conditions, "kind IN ("+strings.Join(placeholders, ",")+")")
	}

	if filter.Since != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, *filter.Since)
	}

	if filter.Until != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, *filter.Until)
	}

	// Build base query
	query := "SELECT COUNT(*) FROM events"

	// Add WHERE clause if we have conditions
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// For simplicity, we'll count all matching events including duplicates
	// A more sophisticated implementation would handle the 'seen' map to avoid double-counting
	// across multiple filters, but for COUNT this is less critical than for QueryEvents

	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to execute count query: %w", err)
	}

	return count, nil
}

// DeleteEventsOlderThan deletes all events older than the specified duration
// This is useful for implementing event retention policies
func (s *Store) DeleteEventsOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-age).Unix()

	result, err := s.db.ExecContext(ctx, "DELETE FROM events WHERE created_at < ?", cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old events: %w", err)
	}

	return result.RowsAffected()
}

// PruneDeletedEvents removes old entries from the deleted_events table
// This helps keep the database size manageable
func (s *Store) PruneDeletedEvents(ctx context.Context, age time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-age).Unix()

	result, err := s.db.ExecContext(ctx, "DELETE FROM deleted_events WHERE deleted_at < ?", cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to prune deleted events: %w", err)
	}

	return result.RowsAffected()
}

// Vacuum runs the SQLite VACUUM command to reclaim unused space
// This should be run during low-traffic periods
func (s *Store) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}
	return nil
}

// Stats returns database statistics for monitoring
type Stats struct {
	EventCount        int64
	DeletedEventCount int64
	DatabaseSizeKB    int64
}

// GetStats returns database statistics
func (s *Store) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{}

	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&stats.EventCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count events: %w", err)
	}

	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM deleted_events").Scan(&stats.DeletedEventCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count deleted events: %w", err)
	}

	// Get database file size (approximate)
	var pageCount int64
	var pageSize int64
	err = s.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount)
	if err == nil {
		err = s.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize)
		if err == nil {
			stats.DatabaseSizeKB = (pageCount * pageSize) / 1024
		}
	}

	return stats, nil
}

func tagsToJSON(tags [][]string) string {
	if len(tags) == 0 {
		return "[]"
	}
	var tagStrings []string
	for _, tag := range tags {
		tagStr := "["
		for i, part := range tag {
			if i > 0 {
				tagStr += ","
			}
			tagStr += `"` + strings.ReplaceAll(part, `"`, `\"`) + `"`
		}
		tagStr += "]"
		tagStrings = append(tagStrings, tagStr)
	}
	return "[" + strings.Join(tagStrings, ",") + "]"
}

func jsonToTags(jsonStr string) [][]string {
	if jsonStr == "" || jsonStr == "[]" {
		return [][]string{}
	}
	return parseTagsJSON(jsonStr)
}

func (s *Store) SaveChannelEvent(ctx context.Context, evt *event.Event) error {
	tagsJSON := tagsToJSON(evt.Tags)

	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO channel_events (id, channel_id, pubkey, created_at, kind, tags, content, sig)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, evt.ID, getChannelID(evt), evt.PubKey, evt.CreatedAt, evt.Kind, tagsJSON, evt.Content, evt.Sig)
	if err != nil {
		return fmt.Errorf("failed to save channel event: %w", err)
	}
	return nil
}

func (s *Store) GetChannelEvent(ctx context.Context, eventID string) (*event.Event, error) {
	var evt event.Event
	var tagsJSON string
	var channelID string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, channel_id, pubkey, created_at, kind, tags, content, sig
		FROM channel_events WHERE id = ?
	`, eventID).Scan(&evt.ID, &channelID, &evt.PubKey, &evt.CreatedAt, &evt.Kind, &tagsJSON, &evt.Content, &evt.Sig)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get channel event: %w", err)
	}

	evt.Tags = jsonToTags(tagsJSON)
	return &evt, nil
}

func (s *Store) QueryChannelEvents(ctx context.Context, channelID string, since, until *int64, limit *int) ([]*event.Event, error) {
	query := "SELECT id, channel_id, pubkey, created_at, kind, tags, content, sig FROM channel_events WHERE channel_id = ?"
	args := []interface{}{channelID}

	if since != nil {
		query += " AND created_at >= ?"
		args = append(args, *since)
	}
	if until != nil {
		query += " AND created_at <= ?"
		args = append(args, *until)
	}

	query += " ORDER BY created_at DESC"

	if limit != nil && *limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", *limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query channel events: %w", err)
	}
	defer rows.Close()

	var events []*event.Event
	for rows.Next() {
		var evt event.Event
		var tagsJSON string
		var channelID string

		if err := rows.Scan(&evt.ID, &channelID, &evt.PubKey, &evt.CreatedAt, &evt.Kind, &tagsJSON, &evt.Content, &evt.Sig); err != nil {
			return nil, fmt.Errorf("failed to scan channel event: %w", err)
		}

		evt.Tags = jsonToTags(tagsJSON)
		events = append(events, &evt)
	}

	return events, rows.Err()
}

func (s *Store) GetChannelMetadata(ctx context.Context, channelID string) (*event.Event, error) {
	var evt event.Event
	var tagsJSON string
	var storedChannelID string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, channel_id, pubkey, created_at, kind, tags, content, sig
		FROM channel_events
		WHERE channel_id = ? AND kind IN (40, 41)
		ORDER BY created_at DESC
		LIMIT 1
	`, channelID).Scan(&evt.ID, &storedChannelID, &evt.PubKey, &evt.CreatedAt, &evt.Kind, &tagsJSON, &evt.Content, &evt.Sig)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get channel metadata: %w", err)
	}

	evt.Tags = jsonToTags(tagsJSON)
	return &evt, nil
}

func (s *Store) ListChannels(ctx context.Context, limit int) ([]*event.Event, error) {
	query := `
		SELECT id, channel_id, pubkey, created_at, kind, tags, content, sig
		FROM channel_events
		WHERE kind IN (40, 41)
		GROUP BY channel_id
		HAVING created_at = MAX(created_at)
		ORDER BY created_at DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list channels: %w", err)
	}
	defer rows.Close()

	var channels []*event.Event
	for rows.Next() {
		var evt event.Event
		var tagsJSON string
		var channelID string

		if err := rows.Scan(&evt.ID, &channelID, &evt.PubKey, &evt.CreatedAt, &evt.Kind, &tagsJSON, &evt.Content, &evt.Sig); err != nil {
			return nil, fmt.Errorf("failed to scan channel: %w", err)
		}

		evt.Tags = jsonToTags(tagsJSON)
		channels = append(channels, &evt)
	}

	return channels, rows.Err()
}

func getChannelID(evt *event.Event) string {
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "channel_id" {
			return tag[1]
		}
	}
	return ""
}
