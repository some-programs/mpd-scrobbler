package main

import (
	"testing"
	"time"

	"github.com/some-programs/mpd-scrobbler/pkg/daemon"
	"github.com/some-programs/mpd-scrobbler/pkg/lastfm"
	"github.com/some-programs/mpd-scrobbler/pkg/tracker"
)

func TestDefaultConfigurations(t *testing.T) {
	dCfg := daemon.DefaultConfig()
	if dCfg.MPDNetwork != "tcp" {
		t.Errorf("expected default network 'tcp', got %s", dCfg.MPDNetwork)
	}
	if dCfg.MPDAddress != "127.0.0.1:6600" {
		t.Errorf("expected default address '127.0.0.1:6600', got %s", dCfg.MPDAddress)
	}

	tCfg := tracker.DefaultConfig()
	if tCfg.MinTrackDuration != 30*time.Second {
		t.Errorf("expected min duration 30s, got %v", tCfg.MinTrackDuration)
	}
	if tCfg.ScrobblePercent != 50 {
		t.Errorf("expected scrobble percent 50, got %d", tCfg.ScrobblePercent)
	}
	if tCfg.MaxDuration != 240*time.Second {
		t.Errorf("expected max duration 240s, got %v", tCfg.MaxDuration)
	}

	// Verify dry-run client initialization
	lClient, err := lastfm.NewClient(lastfm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("failed to initialize dry-run client: %v", err)
	}
	d := daemon.New(dCfg, lClient)
	if d == nil {
		t.Fatalf("expected non-nil daemon")
	}
}
