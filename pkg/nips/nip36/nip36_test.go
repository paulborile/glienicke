package nip36

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paul/glienicke/pkg/event"
	"github.com/stretchr/testify/assert"
)

func writeVocab(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "vocab.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write vocab: %v", err)
	}
	return path
}

func TestHasContentWarning(t *testing.T) {
	evt := &event.Event{Tags: [][]string{{"content-warning", "nsfw"}}}
	assert.True(t, HasContentWarning(evt))

	evt2 := &event.Event{Tags: [][]string{{"t", "nsfw"}}}
	assert.False(t, HasContentWarning(evt2))
}

func TestPolicy_LoadAndMatch(t *testing.T) {
	dir := t.TempDir()
	path := writeVocab(t, dir, "# comment\nporn\nnsfw\n\nyande.re\n")

	p := New(path)
	assert.Equal(t, 3, p.TermCount())

	tests := []struct {
		name       string
		content    string
		tags       [][]string
		expectTerm string
	}{
		{"clean text", "Hello world", nil, ""},
		{"content match porn", "buy our porn now", nil, "porn"},
		{"content match case insensitive", "Watch NSFW VIDEOS", nil, "nsfw"},
		{"hashtag match", "check this out", [][]string{{"t", "NSFW"}}, "nsfw"},
		{"url match", "https://yande.re/post/show/1259888", nil, "yande.re"},
		{"no match", "totally clean content", [][]string{{"t", "bitcoin"}}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := p.MatchedTerm(&event.Event{Content: tc.content, Tags: tc.tags})
			assert.Equal(t, tc.expectTerm, got)
		})
	}
}

func TestPolicy_ShouldReject(t *testing.T) {
	dir := t.TempDir()
	path := writeVocab(t, dir, "porn\n")
	p := New(path)

	clean := &event.Event{Content: "hello"}
	assert.Empty(t, p.ShouldReject(clean))

	dirty := &event.Event{Content: "free porn here"}
	assert.NotEmpty(t, p.ShouldReject(dirty))

	tagged := &event.Event{
		Content: "free porn here",
		Tags:    [][]string{{"content-warning", "nsfw"}},
	}
	assert.Empty(t, p.ShouldReject(tagged))
}

func TestPolicy_EmptyVocabIsNoop(t *testing.T) {
	p := New("/nonexistent/path/vocab.txt")
	assert.Equal(t, 0, p.TermCount())
	evt := &event.Event{Content: "any content with porn"}
	assert.Empty(t, p.ShouldReject(evt))
}

func TestPolicy_ReloadOnMtimeChange(t *testing.T) {
	dir := t.TempDir()
	path := writeVocab(t, dir, "porn\n")
	p := New(path)
	defer p.Close()
	assert.Equal(t, 1, p.TermCount())

	// Sleep long enough to guarantee a different mtime, then update file.
	time.Sleep(20 * time.Millisecond)
	writeVocab(t, dir, "porn\nnsfw\nyande.re\n")
	// Force a stat-based reload by manually nudging the mtime
	now := time.Now().Add(1 * time.Second)
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	// Trigger reload manually (we don't want to wait for the watcher in tests)
	p.reload()
	assert.Equal(t, 3, p.TermCount())

	// Match against new term
	evt := &event.Event{Content: "https://yande.re/post"}
	assert.NotEmpty(t, p.ShouldReject(evt))
}

func TestPolicy_FileDisappears(t *testing.T) {
	dir := t.TempDir()
	path := writeVocab(t, dir, "porn\n")
	p := New(path)
	defer p.Close()
	assert.Equal(t, 1, p.TermCount())

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	p.reload()
	assert.Equal(t, 0, p.TermCount())
	assert.Empty(t, p.ShouldReject(&event.Event{Content: "free porn"}))
}
