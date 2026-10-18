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

	if cfg.MPD.Network != "unix" {
		t.Errorf("unexpected MPD.Network: %v", cfg.MPD.Network)
	}
	if cfg.MPD.Address != "/run/mpd/socket" {
		t.Errorf("unexpected MPD.Address: %v", cfg.MPD.Address)
	}
	if cfg.MPD.Password != "secretpassword" {
		t.Errorf("unexpected MPD.Password: %v", cfg.MPD.Password)
	}
	if cfg.MPD.ReconnectInterval.Duration() != 10*time.Second {
		t.Errorf("unexpected MPD.ReconnectInterval: %v", cfg.MPD.ReconnectInterval)
	}

	if cfg.LastFM.APIKey != "test_api_key" {
		t.Errorf("unexpected LastFM.APIKey: %v", cfg.LastFM.APIKey)
	}
	if cfg.LastFM.SessionKey != "test_session_key" {
		t.Errorf("unexpected LastFM.SessionKey: %v", cfg.LastFM.SessionKey)
	}

	if cfg.Scrobble.MinLength.Duration() != 45*time.Second {
		t.Errorf("unexpected Scrobble.MinLength: %v", cfg.Scrobble.MinLength)
	}
	if cfg.Scrobble.MinPercent != 60 {
		t.Errorf("unexpected Scrobble.MinPercent: %v", cfg.Scrobble.MinPercent)
	}
	if cfg.Scrobble.MaxDuration.Duration() != 5*time.Minute {
		t.Errorf("unexpected Scrobble.MaxDuration: %v", cfg.Scrobble.MaxDuration)
	}
	if cfg.Scrobble.NowPlaying != false {
		t.Errorf("unexpected Scrobble.NowPlaying: %v", cfg.Scrobble.NowPlaying)
	}

	if cfg.General.DryRun != true {
		t.Errorf("unexpected General.DryRun: %v", cfg.General.DryRun)
	}
	if cfg.General.Verbose != true {
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Loading with empty string when default files don't exist should not error
	cfg, _, err := LoadFile("")
	if err != nil {
		t.Fatalf("expected empty non-explicit LoadFile to succeed: %v", err)
	}
	if cfg.MPD.Address != "127.0.0.1:6600" {
		t.Errorf("expected default MPD address, got %s", cfg.MPD.Address)
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

func TestBacklogLimitConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Test scrobble section backlog_limit = 0
	content1 := `
[scrobble]
backlog_limit = 0
`
	f1 := filepath.Join(tmpDir, "c1.toml")
	if err := os.WriteFile(f1, []byte(content1), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	cfg1, _, err := LoadFile(f1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit := cfg1.Scrobble.BacklogLimit; limit != 0 {
		t.Errorf("expected BacklogLimit 0 from [scrobble], got %d", limit)
	}

	// Test default when omitted
	cfg2 := DefaultFileConfig()
	if limit := cfg2.Scrobble.BacklogLimit; limit != 5000 {
		t.Errorf("expected default BacklogLimit 5000, got %d", limit)
	}
}

func TestSaveSessionKey(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Existing session_key in [lastfm]
	configFile1 := filepath.Join(tmpDir, "config1.toml")
	content1 := `[mpd]
address = "localhost:6600"

[lastfm]
api_key = "test_key"
session_key = "old_key"

[general]
verbose = false
`
	if err := os.WriteFile(configFile1, []byte(content1), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if err := SaveSessionKey(configFile1, "new_session_key_1"); err != nil {
		t.Fatalf("SaveSessionKey failed: %v", err)
	}
	cfg1, _, err := LoadFile(configFile1)
	if err != nil {
		t.Fatalf("failed to load modified config: %v", err)
	}
	if cfg1.LastFM.SessionKey != "new_session_key_1" {
		t.Errorf("expected new_session_key_1, got %s", cfg1.LastFM.SessionKey)
	}

	// 2. Commented # session_key in [lastfm]
	configFile2 := filepath.Join(tmpDir, "config2.toml")
	content2 := `[lastfm]
api_key = "test_key"
# session_key = ""
`
	if err := os.WriteFile(configFile2, []byte(content2), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if err := SaveSessionKey(configFile2, "new_session_key_2"); err != nil {
		t.Fatalf("SaveSessionKey failed: %v", err)
	}
	cfg2, _, err := LoadFile(configFile2)
	if err != nil {
		t.Fatalf("failed to load modified config: %v", err)
	}
	if cfg2.LastFM.SessionKey != "new_session_key_2" {
		t.Errorf("expected new_session_key_2, got %s", cfg2.LastFM.SessionKey)
	}

	// 3. No session_key line, but [lastfm] section exists followed by another section
	configFile3 := filepath.Join(tmpDir, "config3.toml")
	content3 := `[lastfm]
api_key = "test_key"

[general]
verbose = true
`
	if err := os.WriteFile(configFile3, []byte(content3), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if err := SaveSessionKey(configFile3, "new_session_key_3"); err != nil {
		t.Fatalf("SaveSessionKey failed: %v", err)
	}
	cfg3, _, err := LoadFile(configFile3)
	if err != nil {
		t.Fatalf("failed to load modified config: %v", err)
	}
	if cfg3.LastFM.SessionKey != "new_session_key_3" {
		t.Errorf("expected new_session_key_3, got %s", cfg3.LastFM.SessionKey)
	}
	if !cfg3.General.Verbose {
		t.Errorf("expected verbose true to be preserved in [general]")
	}

	// 4. No [lastfm] section at all
	configFile4 := filepath.Join(tmpDir, "config4.toml")
	content4 := `[mpd]
address = "127.0.0.1:6600"
`
	if err := os.WriteFile(configFile4, []byte(content4), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if err := SaveSessionKey(configFile4, "new_session_key_4"); err != nil {
		t.Fatalf("SaveSessionKey failed: %v", err)
	}
	cfg4, _, err := LoadFile(configFile4)
	if err != nil {
		t.Fatalf("failed to load modified config: %v", err)
	}
	if cfg4.LastFM.SessionKey != "new_session_key_4" {
		t.Errorf("expected new_session_key_4, got %s", cfg4.LastFM.SessionKey)
	}
}
