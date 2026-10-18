package daemon

import (
	"context"
	"log"
	"time"

	"github.com/fhs/gompd/v2/mpd"
	"github.com/some-programs/mpd-scrobbler/pkg/lastfm"
	"github.com/some-programs/mpd-scrobbler/pkg/tracker"
)

// Daemon coordinates MPD watching, playback tracking, and scrobbling.
type Daemon struct {
	cfg       Config
	tracker   *tracker.Tracker
	scrobbler lastfm.Client
	client    *mpd.Client
	watcher   *mpd.Watcher
}

// New creates a new Daemon instance.
func New(cfg Config, scrobbler lastfm.Client) *Daemon {
	if cfg.MPDReconnectInterval <= 0 {
		cfg.MPDReconnectInterval = 5 * time.Second
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 1 * time.Second
	}

	d := &Daemon{
		cfg:       cfg,
		scrobbler: scrobbler,
	}

	tr := tracker.NewTracker(cfg.Tracker,
		func(track tracker.Track) {
			if cfg.SendNowPlaying {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := scrobbler.NowPlaying(ctx, track); err != nil && cfg.Verbose {
					log.Printf("[daemon] NowPlaying error: %v", err)
				}
			}
		},
		func(track tracker.Track, startedAt time.Time) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := scrobbler.Scrobble(ctx, track, startedAt); err != nil && cfg.Verbose {
				log.Printf("[daemon] Scrobble error: %v", err)
			}
		},
	)

	d.tracker = tr
	return d
}

// Run starts the daemon loop and blocks until the provided context is canceled.
func (d *Daemon) Run(ctx context.Context) error {
	defer func() {
		d.tracker.Shutdown()
		d.closeConnections()
		_ = d.scrobbler.Close()
	}()

	log.Printf("[daemon] Starting mpd-scrobbler connecting to MPD at %s (%s)...",
		d.cfg.MPDAddress, d.cfg.MPDNetwork)

	if err := d.connectMPD(ctx); err != nil {
		return err
	}

	pollTicker := time.NewTicker(d.cfg.PollInterval)
	defer pollTicker.Stop()

	queueFlushTicker := time.NewTicker(30 * time.Second)
	defer queueFlushTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[daemon] Shutdown requested, cleaning up...")
			return nil

		case subsystem, ok := <-d.watcher.Event:
			if !ok {
				log.Println("[daemon] MPD watcher channel closed, attempting reconnection...")
				if err := d.reconnect(ctx); err != nil {
					return err
				}
				continue
			}

			if subsystem == "player" {
				d.syncState(ctx)
			}

		case err, ok := <-d.watcher.Error:
			if !ok || err != nil {
				log.Printf("[daemon] MPD watcher error: %v, attempting reconnection...", err)
				if err := d.reconnect(ctx); err != nil {
					return err
				}
			}

		case <-pollTicker.C:
			d.tick(ctx)

		case <-queueFlushTicker.C:
			flushCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			_ = d.scrobbler.FlushQueue(flushCtx)
			cancel()
		}
	}
}

func (d *Daemon) connectMPD(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var client *mpd.Client
		var watcher *mpd.Watcher
		var err error

		if d.cfg.MPDPassword != "" {
			client, err = mpd.DialAuthenticated(d.cfg.MPDNetwork, d.cfg.MPDAddress, d.cfg.MPDPassword)
		} else {
			client, err = mpd.Dial(d.cfg.MPDNetwork, d.cfg.MPDAddress)
		}

		if err != nil {
			log.Printf("[daemon] Failed to connect MPD client: %v. Retrying in %v...", err, d.cfg.MPDReconnectInterval)
			if err := d.sleepOrDone(ctx, d.cfg.MPDReconnectInterval); err != nil {
				return err
			}
			continue
		}

		watcher, err = mpd.NewWatcher(d.cfg.MPDNetwork, d.cfg.MPDAddress, d.cfg.MPDPassword, "player")
		if err != nil {
			_ = client.Close()
			log.Printf("[daemon] Failed to connect MPD watcher: %v. Retrying in %v...", err, d.cfg.MPDReconnectInterval)
			if err := d.sleepOrDone(ctx, d.cfg.MPDReconnectInterval); err != nil {
				return err
			}
			continue
		}

		d.client = client
		d.watcher = watcher
		log.Printf("[daemon] Connected to MPD at %s (%s)", d.cfg.MPDAddress, d.cfg.MPDNetwork)

		// Sync initial player state
		d.syncState(ctx)
		return nil
	}
}

func (d *Daemon) syncState(ctx context.Context) {
	if d.client == nil {
		return
	}

	status, err := d.client.Status()
	if err != nil {
		log.Printf("[daemon] MPD status query error: %v, attempting reconnection...", err)
		_ = d.reconnect(ctx)
		return
	}

	song, err := d.client.CurrentSong()
	if err != nil {
		log.Printf("[daemon] MPD current song query error: %v, attempting reconnection...", err)
		_ = d.reconnect(ctx)
		return
	}

	d.tracker.UpdateSong(song, status)
}

func (d *Daemon) tick(ctx context.Context) {
	if d.client == nil {
		return
	}

	status, err := d.client.Status()
	if err != nil {
		log.Printf("[daemon] MPD tick status query error: %v, attempting reconnection...", err)
		_ = d.reconnect(ctx)
		return
	}

	d.tracker.OnTick(status)
}

func (d *Daemon) reconnect(ctx context.Context) error {
	d.closeConnections()
	return d.connectMPD(ctx)
}

func (d *Daemon) closeConnections() {
	if d.watcher != nil {
		_ = d.watcher.Close()
		d.watcher = nil
	}
	if d.client != nil {
		_ = d.client.Close()
		d.client = nil
	}
}

func (d *Daemon) sleepOrDone(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
