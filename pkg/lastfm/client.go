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

// Config contains configuration for Last.fm client authentication, backlog, and behaviour.
type Config struct {
	APIKey       string
	APISecret    string
	SessionKey   string
	Username     string
	Password     string
	DryRun       bool
	Verbose      bool
	BacklogFile  string
	BacklogLimit int
}

type lastfmClient struct {
	cfg   Config
	api   *lastfm.Api
	mu    sync.Mutex
	queue []BacklogItem
}

// NewClient initializes a new Last.fm client with authentication and offline backlog persistence.
func NewClient(cfg Config) (Client, error) {
	if cfg.BacklogLimit < 0 {
		cfg.BacklogLimit = 5000
	}
	if cfg.BacklogFile == "" && cfg.BacklogLimit > 0 {
		cfg.BacklogFile = DefaultBacklogPath()
	}

	if cfg.DryRun {
		return &lastfmClient{
			cfg:   cfg,
			queue: make([]BacklogItem, 0),
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

	// Load existing persistent backlog from disk if enabled
	var queue []BacklogItem
	if cfg.BacklogLimit > 0 && cfg.BacklogFile != "" {
		loaded, err := LoadBacklog(cfg.BacklogFile)
		if err != nil {
			if cfg.Verbose {
				log.Printf("[lastfm] Warning loading backlog: %v", err)
			}
		} else if len(loaded) > 0 {
			log.Printf("[lastfm] Loaded %d pending offline backlog track(s) from %s", len(loaded), cfg.BacklogFile)
			queue = loaded
		}
	} else if cfg.Verbose {
		log.Println("[lastfm] Offline backlog feature is disabled (limit set to 0)")
	}

	return &lastfmClient{
		cfg:   cfg,
		api:   api,
		queue: queue,
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
		if c.cfg.Verbose {
			log.Printf("[lastfm] Warning: failed to send Now Playing: %v", err)
		}
		return err
	}

	log.Printf("[lastfm] Now Playing: %s - %s", track.Artist, track.Title)
	return nil
}

// Scrobble sends a 'track.scrobble' request. If offline or it fails, it saves to persistent backlog.
func (c *lastfmClient) Scrobble(ctx context.Context, track tracker.Track, timestamp time.Time) error {
	if c.cfg.DryRun {
		log.Printf("[dry-run] Scrobble: %s - %s [%s] (started: %s)",
			track.Artist, track.Title, track.Album, timestamp.Format(time.RFC3339))
		return nil
	}

	// If backlog is enabled and there's an existing backlog, try flushing it first
	if c.cfg.BacklogLimit > 0 {
		_ = c.FlushQueue(ctx)

		// If backlog couldn't be flushed (still offline), add current track directly to backlog
		c.mu.Lock()
		backlogPending := len(c.queue) > 0
		c.mu.Unlock()

		if backlogPending {
			c.enqueue(track, timestamp)
			return errors.New("offline backlog pending, queued scrobble")
		}
	}

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
		if c.cfg.BacklogLimit > 0 {
			log.Printf("[lastfm] Error scrobbling %s - %s: %v. Saving to offline backlog.", track.Artist, track.Title, err)
			c.enqueue(track, timestamp)
		} else {
			log.Printf("[lastfm] Error scrobbling %s - %s: %v. Backlog is disabled, dropping.", track.Artist, track.Title, err)
		}
		return err
	}

	log.Printf("[lastfm] Scrobble submitted: %s - %s [%s]", track.Artist, track.Title, track.Album)
	return nil
}

func (c *lastfmClient) enqueue(track tracker.Track, timestamp time.Time) {
	if c.cfg.BacklogLimit <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	item := BacklogItem{Track: track, Timestamp: timestamp}
	if len(c.queue) >= c.cfg.BacklogLimit {
		// Drop oldest beyond limit
		c.queue = c.queue[1:]
	}
	c.queue = append(c.queue, item)

	if c.cfg.BacklogFile != "" {
		if err := AppendBacklog(c.cfg.BacklogFile, item); err != nil && c.cfg.Verbose {
			log.Printf("[lastfm] Warning: failed to append to backlog file: %v", err)
		}
	}
	log.Printf("[lastfm] Track appended to offline backlog (%d pending in %s)", len(c.queue), c.cfg.BacklogFile)
}

// FlushQueue attempts to submit all queued backlog scrobbles.
func (c *lastfmClient) FlushQueue(ctx context.Context) error {
	if c.cfg.DryRun || c.cfg.BacklogLimit <= 0 {
		return nil
	}

	c.mu.Lock()
	if len(c.queue) == 0 {
		c.mu.Unlock()
		return nil
	}
	pending := make([]BacklogItem, len(c.queue))
	copy(pending, c.queue)
	c.mu.Unlock()

	var remaining []BacklogItem
	submittedCount := 0

	for i, item := range pending {
		select {
		case <-ctx.Done():
			remaining = append(remaining, pending[i:]...)
			break
		default:
		}
		if len(remaining) > 0 {
			break
		}

		params := lastfm.P{
			"artist":    item.Track.Artist,
			"track":     item.Track.Title,
			"timestamp": item.Timestamp.Unix(),
		}
		if item.Track.Album != "" {
			params["album"] = item.Track.Album
		}
		if item.Track.AlbumArtist != "" {
			params["albumArtist"] = item.Track.AlbumArtist
		}
		if item.Track.TrackNumber > 0 {
			params["trackNumber"] = item.Track.TrackNumber
		}
		if item.Track.Duration > 0 {
			params["duration"] = int(item.Track.Duration.Seconds())
		}

		_, err := c.api.Track.Scrobble(params)
		if err != nil {
			// Network error or offline: stop flushing and keep remaining tracks on disk
			if c.cfg.Verbose {
				log.Printf("[lastfm] Backlog submission paused at %s - %s: %v", item.Track.Artist, item.Track.Title, err)
			}
			remaining = append(remaining, pending[i:]...)
			break
		}

		submittedCount++
	}

	c.mu.Lock()
	c.queue = remaining
	if c.cfg.BacklogFile != "" {
		_ = RewriteBacklog(c.cfg.BacklogFile, c.queue)
	}
	c.mu.Unlock()

	if submittedCount > 0 {
		log.Printf("[lastfm] Backlog flush: submitted %d track(s) to Last.fm (%d still pending)",
			submittedCount, len(remaining))
	}

	return nil
}

// Close flushes any pending scrobbles and persists the backlog upon shutdown.
func (c *lastfmClient) Close() error {
	if c.cfg.BacklogLimit <= 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = c.FlushQueue(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.BacklogFile != "" {
		_ = RewriteBacklog(c.cfg.BacklogFile, c.queue)
	}
	return nil
}
