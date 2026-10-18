package daemon

import (
	"time"

	"github.com/some-programs/mpd-scrobbler/pkg/lastfm"
	"github.com/some-programs/mpd-scrobbler/pkg/tracker"
)

// Config holds all configuration options for the mpd-scrobbler daemon.
type Config struct {
	// MPD configuration
	MPDNetwork           string        // Network type: "tcp" or "unix"
	MPDAddress           string        // Address: "127.0.0.1:6600" or socket path
	MPDPassword          string        // MPD authentication password
	MPDReconnectInterval time.Duration // Interval between reconnection attempts

	// Last.fm client configuration
	LastFM lastfm.Config

	// Scrobbler rules configuration
	Tracker tracker.Config

	// Behavior flags
	SendNowPlaying bool          // Whether to send 'Now Playing' notifications
	PollInterval   time.Duration // Playback polling interval while playing (default 1s)
	Verbose        bool          // Verbose debug logging
}

// DefaultConfig returns default configuration for the daemon.
func DefaultConfig() Config {
	return Config{
		MPDNetwork:           "tcp",
		MPDAddress:           "127.0.0.1:6600",
		MPDPassword:          "",
		MPDReconnectInterval: 5 * time.Second,
		LastFM: lastfm.Config{
			DryRun:     false,
			Verbose:    false,
			QueueLimit: 500,
		},
		Tracker:        tracker.DefaultConfig(),
		SendNowPlaying: true,
		PollInterval:   3 * time.Second,
		Verbose:        false,
	}
}
