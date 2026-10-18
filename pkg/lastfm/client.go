package lastfm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/shkh/lastfm-go/lastfm"
	"github.com/some-programs/mpd-scrobbler/pkg/tracker"
)

// Client defines the interface for scrobbling and now-playing notifications.
type Client interface {
	NowPlaying(ctx context.Context, track tracker.Track) error
	Scrobble(ctx context.Context, track tracker.Track, timestamp time.Time) error
	FlushQueue(ctx context.Context) error
	Close() error
}

// Config contains configuration for Last.fm client authentication and behaviour.
type Config struct {
	APIKey     string
	APISecret  string
	SessionKey string
	Username   string
	Password   string
	DryRun     bool
	Verbose    bool
	QueueLimit int
}

type queuedScrobble struct {
	track     tracker.Track
	timestamp time.Time
}

type lastfmClient struct {
	cfg   Config
	api   *lastfm.Api
	mu    sync.Mutex
	queue []queuedScrobble
}

// NewClient initializes a new Last.fm client with authentication.
func NewClient(cfg Config) (Client, error) {
	if cfg.DryRun {
		return &lastfmClient{
			cfg: cfg,
		}, nil
	}

	if cfg.APIKey == "" || cfg.APISecret == "" {
		return nil, errors.New("last.fm api-key and api-secret are required (set via flags or LASTFM_API_KEY / LASTFM_API_SECRET)")
	}

	api := lastfm.New(cfg.APIKey, cfg.APISecret)

	if cfg.SessionKey != "" {
		api.SetSession(cfg.SessionKey)
		if cfg.Verbose {
			log.Println("[lastfm] Authenticated using provided session key")
		}
	} else if cfg.Username != "" && cfg.Password != "" {
		if cfg.Verbose {
			log.Printf("[lastfm] Logging in as user '%s'...", cfg.Username)
		}
		if err := api.Login(cfg.Username, cfg.Password); err != nil {
			return nil, fmt.Errorf("failed to authenticate with Last.fm: %w", err)
		}
		sk := api.GetSessionKey()
		log.Printf("[lastfm] Successfully logged in. Session key: %s", sk)
	} else {
		return nil, errors.New("no Last.fm credentials provided: specify session-key or username and password (or run with -lastfm.auth to authorize)")
	}

	if cfg.QueueLimit <= 0 {
		cfg.QueueLimit = 500
	}

	return &lastfmClient{
		cfg:   cfg,
		api:   api,
		queue: make([]queuedScrobble, 0),
	}, nil
}

// NowPlaying sends a 'track.updateNowPlaying' request.
func (c *lastfmClient) NowPlaying(ctx context.Context, track tracker.Track) error {
	if c.cfg.DryRun {
		log.Printf("[dry-run] Now Playing: %s - %s [%s]", track.Artist, track.Title, track.Album)
		return nil
	}

	params := lastfm.P{
		"artist": track.Artist,
		"track":  track.Title,
	}
	if track.Album != "" {
		params["album"] = track.Album
	}
	if track.AlbumArtist != "" {
		params["albumArtist"] = track.AlbumArtist
	}
	if track.TrackNumber > 0 {
		params["trackNumber"] = track.TrackNumber
	}
	if track.Duration > 0 {
		params["duration"] = int(track.Duration.Seconds())
	}

	if c.cfg.Verbose {
		log.Printf("[lastfm] Sending Now Playing: %s - %s", track.Artist, track.Title)
	}

	_, err := c.api.Track.UpdateNowPlaying(params)
	if err != nil {
		log.Printf("[lastfm] Warning: failed to send Now Playing: %v", err)
		return err
	}

	log.Printf("[lastfm] Now Playing: %s - %s", track.Artist, track.Title)
	return nil
}

// Scrobble sends a 'track.scrobble' request. If it fails, it queues it for retry.
func (c *lastfmClient) Scrobble(ctx context.Context, track tracker.Track, timestamp time.Time) error {
	if c.cfg.DryRun {
		log.Printf("[dry-run] Scrobble: %s - %s [%s] (started: %s)",
			track.Artist, track.Title, track.Album, timestamp.Format(time.RFC3339))
		return nil
	}

	// Try submitting any previously queued scrobbles first
	_ = c.FlushQueue(ctx)

	params := lastfm.P{
		"artist":    track.Artist,
		"track":     track.Title,
		"timestamp": timestamp.Unix(),
	}
	if track.Album != "" {
		params["album"] = track.Album
	}
	if track.AlbumArtist != "" {
		params["albumArtist"] = track.AlbumArtist
	}
	if track.TrackNumber > 0 {
		params["trackNumber"] = track.TrackNumber
	}
	if track.Duration > 0 {
		params["duration"] = int(track.Duration.Seconds())
	}

	_, err := c.api.Track.Scrobble(params)
	if err != nil {
		log.Printf("[lastfm] Error scrobbling %s - %s: %v. Adding to retry queue.", track.Artist, track.Title, err)
		c.enqueue(track, timestamp)
		return err
	}

	log.Printf("[lastfm] Scrobble submitted: %s - %s [%s]", track.Artist, track.Title, track.Album)
	return nil
}

func (c *lastfmClient) enqueue(track tracker.Track, timestamp time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.queue) >= c.cfg.QueueLimit {
		// Drop oldest
		c.queue = c.queue[1:]
	}
	c.queue = append(c.queue, queuedScrobble{track: track, timestamp: timestamp})
	log.Printf("[lastfm] Scrobble queue contains %d pending track(s)", len(c.queue))
}

// FlushQueue attempts to submit all queued scrobbles.
func (c *lastfmClient) FlushQueue(ctx context.Context) error {
	if c.cfg.DryRun {
		return nil
	}

	c.mu.Lock()
	if len(c.queue) == 0 {
		c.mu.Unlock()
		return nil
	}
	pending := make([]queuedScrobble, len(c.queue))
	copy(pending, c.queue)
	c.queue = c.queue[:0]
	c.mu.Unlock()

	var remaining []queuedScrobble
	for _, item := range pending {
		params := lastfm.P{
			"artist":    item.track.Artist,
			"track":     item.track.Title,
			"timestamp": item.timestamp.Unix(),
		}
		if item.track.Album != "" {
			params["album"] = item.track.Album
		}
		if item.track.AlbumArtist != "" {
			params["albumArtist"] = item.track.AlbumArtist
		}
		if item.track.TrackNumber > 0 {
			params["trackNumber"] = item.track.TrackNumber
		}
		if item.track.Duration > 0 {
			params["duration"] = int(item.track.Duration.Seconds())
		}

		_, err := c.api.Track.Scrobble(params)
		if err != nil {
			log.Printf("[lastfm] Retry failed for %s - %s: %v", item.track.Artist, item.track.Title, err)
			remaining = append(remaining, item)
		} else {
			log.Printf("[lastfm] Queued scrobble submitted: %s - %s", item.track.Artist, item.track.Title)
		}
	}

	if len(remaining) > 0 {
		c.mu.Lock()
		c.queue = append(remaining, c.queue...)
		if len(c.queue) > c.cfg.QueueLimit {
			c.queue = c.queue[len(c.queue)-c.cfg.QueueLimit:]
		}
		c.mu.Unlock()
	}

	return nil
}

// Close flushes any pending scrobbles upon shutdown.
func (c *lastfmClient) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.FlushQueue(ctx)
}
