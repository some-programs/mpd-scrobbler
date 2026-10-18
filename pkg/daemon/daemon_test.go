package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/some-programs/mpd-scrobbler/pkg/tracker"
)

type mockScrobbler struct {
	nowPlaying []tracker.Track
	scrobbles  []tracker.Track
}

func (m *mockScrobbler) NowPlaying(ctx context.Context, track tracker.Track) error {
	m.nowPlaying = append(m.nowPlaying, track)
	return nil
}

func (m *mockScrobbler) Scrobble(ctx context.Context, track tracker.Track, timestamp time.Time) error {
	m.scrobbles = append(m.scrobbles, track)
	return nil
}

func (m *mockScrobbler) FlushQueue(ctx context.Context) error {
	return nil
}

func (m *mockScrobbler) Close() error {
	return nil
}

func TestNewDaemon(t *testing.T) {
	cfg := DefaultConfig()
	mock := &mockScrobbler{}

	d := New(cfg, mock)
	if d == nil {
		t.Fatalf("expected New daemon to not be nil")
	}
	if d.tracker == nil {
		t.Fatalf("expected tracker to be initialized")
	}
}

func TestDaemonSyncState(t *testing.T) {
	mock := &mockScrobbler{}
	cfg := DefaultConfig()
	d := New(cfg, mock)

	// syncState with nil client should not panic
	d.syncState(context.Background())

	// tick with nil client should not panic
	d.tick(context.Background())
}
