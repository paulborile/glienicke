// Package nip36 implements NIP-36 content-warning enforcement.
//
// The relay reads a vocabulary of NSFW trigger words/patterns from a file
// and rejects events containing any of those terms unless they carry a
// content-warning tag. The vocabulary file is reloaded automatically when
// its mtime changes on disk.
package nip36

import (
	"bufio"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paul/glienicke/pkg/event"
)

// Policy enforces NIP-36 content-warning requirements based on a configurable
// vocabulary loaded from disk and reloaded automatically on file change.
type Policy struct {
	path       string
	terms      atomic.Pointer[[]string] // immutable snapshot of lowercased terms
	mtime      atomic.Int64
	stopReload chan struct{}
	mu         sync.Mutex
}

// New creates a Policy backed by the given vocabulary file. If the file does
// not exist, the policy starts empty and will pick up the file when it
// appears. Returns the loaded Policy regardless of file state.
func New(path string) *Policy {
	p := &Policy{
		path:       path,
		stopReload: make(chan struct{}),
	}
	empty := []string{}
	p.terms.Store(&empty)
	p.reload()
	return p
}

// StartWatcher begins polling the vocabulary file every interval and reloads
// when the file's mtime changes. Stop the watcher with Close.
func (p *Policy) StartWatcher(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-p.stopReload:
				return
			case <-ticker.C:
				p.reload()
			}
		}
	}()
}

// Close stops the file watcher.
func (p *Policy) Close() {
	close(p.stopReload)
}

// reload reads the vocabulary file if its mtime has changed.
func (p *Policy) reload() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.path == "" {
		return
	}

	info, err := os.Stat(p.path)
	if err != nil {
		// File missing: clear vocabulary so policy becomes a no-op.
		if p.mtime.Load() != 0 {
			empty := []string{}
			p.terms.Store(&empty)
			p.mtime.Store(0)
			log.Printf("NIP-36: vocabulary file %s missing, policy disabled", p.path)
		}
		return
	}

	mtime := info.ModTime().UnixNano()
	if mtime == p.mtime.Load() {
		return // unchanged
	}

	terms, err := loadFile(p.path)
	if err != nil {
		log.Printf("NIP-36: failed to read vocabulary file %s: %v", p.path, err)
		return
	}

	p.terms.Store(&terms)
	p.mtime.Store(mtime)
	log.Printf("NIP-36: loaded %d trigger terms from %s", len(terms), p.path)
}

// loadFile parses the vocabulary file. Format: one term per line, blank lines
// and lines starting with '#' are ignored. Terms are lowercased and trimmed.
func loadFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var terms []string
	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		t := strings.ToLower(line)
		if seen[t] {
			continue
		}
		seen[t] = true
		terms = append(terms, t)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return terms, nil
}

// HasContentWarning returns true if the event has a content-warning tag.
func HasContentWarning(evt *event.Event) bool {
	for _, tag := range evt.Tags {
		if len(tag) >= 1 && tag[0] == "content-warning" {
			return true
		}
	}
	return false
}

// MatchedTerm returns the first vocabulary term found in the event content
// or hashtag tags, or "" if none match. Case-insensitive substring match.
func (p *Policy) MatchedTerm(evt *event.Event) string {
	terms := p.terms.Load()
	if terms == nil || len(*terms) == 0 {
		return ""
	}

	content := strings.ToLower(evt.Content)
	for _, t := range *terms {
		if strings.Contains(content, t) {
			return t
		}
	}

	// Also check hashtag-style 't' tags
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "t" {
			tagVal := strings.ToLower(tag[1])
			for _, t := range *terms {
				if strings.Contains(tagVal, t) {
					return t
				}
			}
		}
	}
	return ""
}

// ShouldReject returns a non-empty reason string if the event must be rejected
// because it matches the NSFW vocabulary but lacks a content-warning tag.
func (p *Policy) ShouldReject(evt *event.Event) string {
	if HasContentWarning(evt) {
		return ""
	}
	if term := p.MatchedTerm(evt); term != "" {
		return "blocked: NSFW content must include a content-warning tag (NIP-36)"
	}
	return ""
}

// TermCount returns the number of currently loaded terms (for diagnostics).
func (p *Policy) TermCount() int {
	terms := p.terms.Load()
	if terms == nil {
		return 0
	}
	return len(*terms)
}
