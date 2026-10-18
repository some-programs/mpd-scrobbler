package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFile_Valid(t *testing.T) {
	content := `
[mpd]
network = "unix"
address = "/run/mpd/socket"
password = "secretpassword"
reconnect_interval = "10s"

[lastfm]
api_key = "test_api_key"
api_secret = "test_api_secret"
session_key = "test_session_key"
username = "test_user"
password = "test_password"

[scrobble]
min_length = "45s"
min_percent = 60
max_duration = "5m"
now_playing = false

[general]
dry_run = true
verbose = true
`
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, path, err := LoadFile(configFile)
	if err != nil {
		t.Fatalf("expected LoadFile to succeed, got: %v", err)
	}
	if path != configFile {
		t.Errorf("expected path %s, got %s", configFile, path)
	}

	if cfg.MPD.Network == nil || *cfg.MPD.Network != "unix" {
		t.Errorf("unexpected MPD.Network: %v", cfg.MPD.Network)
	}
	if cfg.MPD.Address == nil || *cfg.MPD.Address != "/run/mpd/socket" {
		t.Errorf("unexpected MPD.Address: %v", cfg.MPD.Address)
	}
	if cfg.MPD.Password == nil || *cfg.MPD.Password != "secretpassword" {
		t.Errorf("unexpected MPD.Password: %v", cfg.MPD.Password)
	}
	if cfg.MPD.ReconnectInterval == nil || cfg.MPD.ReconnectInterval.Duration() != 10*time.Second {
		t.Errorf("unexpected MPD.ReconnectInterval: %v", cfg.MPD.ReconnectInterval)
	}

	if cfg.LastFM.APIKey == nil || *cfg.LastFM.APIKey != "test_api_key" {
		t.Errorf("unexpected LastFM.APIKey: %v", cfg.LastFM.APIKey)
	}
	if cfg.LastFM.SessionKey == nil || *cfg.LastFM.SessionKey != "test_session_key" {
		t.Errorf("unexpected LastFM.SessionKey: %v", cfg.LastFM.SessionKey)
	}

	if cfg.Scrobble.MinLength == nil || cfg.Scrobble.MinLength.Duration() != 45*time.Second {
		t.Errorf("unexpected Scrobble.MinLength: %v", cfg.Scrobble.MinLength)
	}
	if cfg.Scrobble.MinPercent == nil || *cfg.Scrobble.MinPercent != 60 {
		t.Errorf("unexpected Scrobble.MinPercent: %v", cfg.Scrobble.MinPercent)
	}
	if cfg.Scrobble.MaxDuration == nil || cfg.Scrobble.MaxDuration.Duration() != 5*time.Minute {
		t.Errorf("unexpected Scrobble.MaxDuration: %v", cfg.Scrobble.MaxDuration)
	}
	if cfg.Scrobble.NowPlaying == nil || *cfg.Scrobble.NowPlaying != false {
		t.Errorf("unexpected Scrobble.NowPlaying: %v", cfg.Scrobble.NowPlaying)
	}

	if cfg.General.DryRun == nil || *cfg.General.DryRun != true {
		t.Errorf("unexpected General.DryRun: %v", cfg.General.DryRun)
	}
	if cfg.General.Verbose == nil || *cfg.General.Verbose != true {
		t.Errorf("unexpected General.Verbose: %v", cfg.General.Verbose)
	}
}

func TestLoadFile_ExplicitNotFound(t *testing.T) {
	_, _, err := LoadFile("/nonexistent/file/path/config.toml")
	if err == nil {
		t.Fatalf("expected error when explicit config file does not exist")
	}
}

func TestLoadFile_EmptyNonExplicit(t *testing.T) {
	// Loading with empty string when default files don't exist should not error
	cfg, _, err := LoadFile("")
	if err != nil {
		t.Fatalf("expected empty non-explicit LoadFile to succeed: %v", err)
	}
	if cfg == nil {
		t.Fatalf("expected non-nil config struct")
	}
}

func TestLoadFile_Malformed(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "bad.toml")
	if err := os.WriteFile(configFile, []byte("this is not valid toml = ="), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, _, err := LoadFile(configFile)
	if err == nil {
		t.Fatalf("expected error for malformed TOML")
	}
}
