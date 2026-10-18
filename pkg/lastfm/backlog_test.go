package lastfm

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/some-programs/mpd-scrobbler/pkg/tracker"
)

func TestAppendAndLoadBacklog(t *testing.T) {
	tmpDir := t.TempDir()
	backlogPath := filepath.Join(tmpDir, "sub", "backlog.jsonl")

	now := time.Now().Truncate(time.Second)
	item1 := BacklogItem{
		Track: tracker.Track{
			Artist:   "Boards of Canada",
			Title:    "Music is Math",
			Album:    "Geogaddi",
			Duration: 321 * time.Second,
		},
		Timestamp: now.Add(-10 * time.Minute),
	}
	item2 := BacklogItem{
		Track: tracker.Track{
			Artist:   "Aphex Twin",
			Title:    "Xtal",
			Album:    "Selected Ambient Works 85-92",
			Duration: 294 * time.Second,
		},
		Timestamp: now.Add(-5 * time.Minute),
	}

	// Test sequential appending without rewriting
	if err := AppendBacklog(backlogPath, item1); err != nil {
		t.Fatalf("AppendBacklog item1 failed: %v", err)
	}
	if err := AppendBacklog(backlogPath, item2); err != nil {
		t.Fatalf("AppendBacklog item2 failed: %v", err)
	}

	loaded, err := LoadBacklog(backlogPath)
	if err != nil {
		t.Fatalf("LoadBacklog failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 items, got %d", len(loaded))
	}
	if loaded[0].Track.Artist != "Boards of Canada" || loaded[0].Track.Title != "Music is Math" {
		t.Errorf("item 0 mismatch: %+v", loaded[0])
	}
	if loaded[1].Track.Artist != "Aphex Twin" || loaded[1].Track.Title != "Xtal" {
		t.Errorf("item 1 mismatch: %+v", loaded[1])
	}

	// Test rewriting remaining items (e.g. after item1 was submitted)
	remaining := []BacklogItem{item2}
	if err := RewriteBacklog(backlogPath, remaining); err != nil {
		t.Fatalf("RewriteBacklog failed: %v", err)
	}

	loadedAfterRewrite, err := LoadBacklog(backlogPath)
	if err != nil {
		t.Fatalf("LoadBacklog after rewrite failed: %v", err)
	}
	if len(loadedAfterRewrite) != 1 || loadedAfterRewrite[0].Track.Title != "Xtal" {
		t.Errorf("expected 1 item after rewrite, got %+v", loadedAfterRewrite)
	}

	// Test clearing backlog
	if err := RewriteBacklog(backlogPath, nil); err != nil {
		t.Fatalf("RewriteBacklog nil failed: %v", err)
	}
	cleared, err := LoadBacklog(backlogPath)
	if err != nil {
		t.Fatalf("LoadBacklog after clear failed: %v", err)
	}
	if len(cleared) != 0 {
		t.Errorf("expected 0 items after clear, got %d", len(cleared))
	}
}

func TestLoadBacklog_FiltersOldItems(t *testing.T) {
	tmpDir := t.TempDir()
	backlogPath := filepath.Join(tmpDir, "backlog.jsonl")

	now := time.Now()
	oldItem := BacklogItem{
		Track:     tracker.Track{Artist: "Old Artist", Title: "Old Track"},
		Timestamp: now.Add(-20 * 24 * time.Hour), // 20 days old (>14d cutoff)
	}
	recentItem := BacklogItem{
		Track:     tracker.Track{Artist: "Recent Artist", Title: "Recent Track"},
		Timestamp: now.Add(-2 * time.Hour), // 2 hours old
	}

	if err := AppendBacklog(backlogPath, oldItem); err != nil {
		t.Fatalf("AppendBacklog failed: %v", err)
	}
	if err := AppendBacklog(backlogPath, recentItem); err != nil {
		t.Fatalf("AppendBacklog failed: %v", err)
	}

	loaded, err := LoadBacklog(backlogPath)
	if err != nil {
		t.Fatalf("LoadBacklog failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("expected 1 item after filtering old items, got %d", len(loaded))
	}
	if loaded[0].Track.Title != "Recent Track" {
		t.Errorf("expected 'Recent Track', got '%s'", loaded[0].Track.Title)
	}
}

func TestLoadBacklog_LegacyJSONCompatibility(t *testing.T) {
	tmpDir := t.TempDir()
	backlogPath := filepath.Join(tmpDir, "backlog.jsonl")
	legacyJSONPath := filepath.Join(tmpDir, "backlog.json")

	// Create legacy JSON array file
	legacyData := []byte(`[
		{"track":{"artist":"Legacy Artist","title":"Legacy Track"},"timestamp":"2026-10-18T20:00:00Z"}
	]`)
	if err := os.WriteFile(legacyJSONPath, legacyData, 0644); err != nil {
		t.Fatalf("failed to write legacy json: %v", err)
	}

	// Loading backlog.jsonl when only backlog.json exists should load legacy file
	loaded, err := LoadBacklog(backlogPath)
	if err != nil {
		t.Fatalf("LoadBacklog legacy failed: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Track.Title != "Legacy Track" {
		t.Errorf("expected legacy track loaded, got %+v", loaded)
	}
}
