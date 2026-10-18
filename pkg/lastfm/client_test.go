package lastfm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/some-programs/mpd-scrobbler/pkg/tracker"
)

func TestDryRunClient(t *testing.T) {
	client, err := NewClient(Config{
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("expected NewClient to succeed in DryRun mode without credentials: %v", err)
	}

	track := tracker.Track{
		Artist:   "Boards of Canada",
		Title:    "Dayvan Cowboy",
		Album:    "The Campfire Headphase",
		Duration: 300 * time.Second,
	}

	ctx := context.Background()
	if err := client.NowPlaying(ctx, track); err != nil {
		t.Errorf("NowPlaying failed in dry-run mode: %v", err)
	}

	if err := client.Scrobble(ctx, track, time.Now()); err != nil {
		t.Errorf("Scrobble failed in dry-run mode: %v", err)
	}

	if err := client.FlushQueue(ctx); err != nil {
		t.Errorf("FlushQueue failed in dry-run mode: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Errorf("Close failed in dry-run mode: %v", err)
	}
}

func TestNewClient_MissingCredentials(t *testing.T) {
	_, err := NewClient(Config{
		DryRun: false,
	})
	if err == nil {
		t.Fatalf("expected error when credentials missing and not dry-run")
	}

	_, err = NewClient(Config{
		APIKey:    "test-key",
		APISecret: "test-secret",
	})
	if err == nil {
		t.Fatalf("expected error when session key and username/password are both missing")
	}
}

func TestQueueEnqueue(t *testing.T) {
	tmpDir := t.TempDir()
	backlogFile := filepath.Join(tmpDir, "backlog.json")

	c := &lastfmClient{
		cfg: Config{
			BacklogLimit: 2,
			BacklogFile:  backlogFile,
		},
		queue: make([]BacklogItem, 0),
	}

	t1 := tracker.Track{Artist: "Artist1", Title: "Track1"}
	t2 := tracker.Track{Artist: "Artist2", Title: "Track2"}
	t3 := tracker.Track{Artist: "Artist3", Title: "Track3"}

	c.enqueue(t1, time.Now())
	c.enqueue(t2, time.Now())
	if len(c.queue) != 2 {
		t.Fatalf("expected queue length 2, got %d", len(c.queue))
	}

	// Should drop oldest (t1) and keep t2, t3
	c.enqueue(t3, time.Now())
	if len(c.queue) != 2 {
		t.Fatalf("expected queue length 2 after dropping oldest, got %d", len(c.queue))
	}
	if c.queue[0].Track.Title != "Track2" || c.queue[1].Track.Title != "Track3" {
		t.Errorf("queue order incorrect: %+v", c.queue)
	}

	// Verify items were appended to disk file
	loaded, err := LoadBacklog(backlogFile)
	if err != nil {
		t.Fatalf("failed to load saved backlog: %v", err)
	}
	if len(loaded) != 3 || loaded[2].Track.Title != "Track3" {
		t.Errorf("persisted file mismatch: %+v", loaded)
	}
}

func TestBacklogDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	backlogFile := filepath.Join(tmpDir, "backlog.jsonl")

	c := &lastfmClient{
		cfg: Config{
			BacklogLimit: 0,
			BacklogFile:  backlogFile,
		},
		queue: make([]BacklogItem, 0),
	}

	tr := tracker.Track{Artist: "Artist", Title: "Track"}
	c.enqueue(tr, time.Now())

	if len(c.queue) != 0 {
		t.Fatalf("expected queue length 0 when backlog disabled, got %d", len(c.queue))
	}

	if _, err := os.Stat(backlogFile); !os.IsNotExist(err) {
		t.Fatalf("expected backlog file not to exist when backlog disabled")
	}

	if err := c.FlushQueue(context.Background()); err != nil {
		t.Errorf("expected FlushQueue to return nil when disabled, got: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Errorf("expected Close to return nil when disabled, got: %v", err)
	}

	if _, err := os.Stat(backlogFile); !os.IsNotExist(err) {
		t.Fatalf("expected backlog file not to be created on Close when backlog disabled")
	}
}
