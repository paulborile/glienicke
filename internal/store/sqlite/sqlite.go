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

// Store is a SQLite implementation of storage.Store
type Store struct {
	db *sql.DB
}

// Ensure Store implements storage.Store
var _ storage.Store = (*Store)(nil)

// New creates a new SQLite store with autoconfiguration
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: db}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the necessary tables if they don't exist
func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		pubkey TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		kind INTEGER NOT NULL,
		tags TEXT, -- JSON array
		content TEXT NOT NULL,
		sig TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS deleted_events (
		id TEXT PRIMARY KEY,
		deleter_pubkey TEXT NOT NULL,
		deleted_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_events_pubkey ON events(pubkey);
	CREATE INDEX IF NOT EXISTS idx_events_kind ON events(kind);
	CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at);
	CREATE INDEX IF NOT EXISTS idx_events_kind_created_at ON events(kind, created_at);
	`

	_, err := s.db.Exec(schema)
	return err
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

// QueryEvents retrieves events matching the filters
func (s *Store) QueryEvents(ctx context.Context, filters []*event.Filter) ([]*event.Event, error) {
	if len(filters) == 0 {
		return nil, nil
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

	var events []*event.Event
	for rows.Next() {
		evt := &event.Event{}
		var tagsJSON sql.NullString

		err := rows.Scan(&evt.ID, &evt.PubKey, &evt.CreatedAt, &evt.Kind, &tagsJSON, &evt.Content, &evt.Sig)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse tags JSON if present
		if tagsJSON.Valid && tagsJSON.String != "" && tagsJSON.String != "[]" {
			// Simple JSON parsing for tags array
			// This is a basic implementation - in production you might want a proper JSON parser
			evt.Tags = parseTagsJSON(tagsJSON.String)
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
		return nil
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
	evt := &event.Event{}
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
		evt.Tags = parseTagsJSON(tagsJSON.String)
	}

	return evt, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}
