package tracker

import (
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

// Track represents metadata of a track played in MPD.
type Track struct {
	ID          string        // MPD song ID
	File        string        // MPD file URI or path
	Artist      string        // Artist name
	Title       string        // Track title
	Album       string        // Album title
	AlbumArtist string        // Album artist name
	TrackNumber int           // Track number on album
	Duration    time.Duration // Total duration of the track
}

// Config holds configuration parameters for scrobble timing and eligibility.
type Config struct {
	MinTrackDuration time.Duration // Minimum track duration to be eligible (default 30s)
	ScrobblePercent  int           // Percentage of track duration required (default 50%)
	MaxDuration      time.Duration // Max listened duration required before scrobbling (default 240s)
	Verbose          bool          // Verbose logging
}

// DefaultConfig returns the standard Audioscrobbler configuration.
func DefaultConfig() Config {
	return Config{
		MinTrackDuration: 30 * time.Second,
		ScrobblePercent:  50,
		MaxDuration:      240 * time.Second,
		Verbose:          false,
	}
}

// PlaySession records the current playback state for a track.
type PlaySession struct {
	Track            Track
	StartedAt        time.Time
	LastElapsed      time.Duration
	ListenedDuration time.Duration
	NowPlayingSent   bool
	Scrobbled        bool
}

// ScrobbleCallback is invoked when a track is eligible for scrobbling.
type ScrobbleCallback func(track Track, startedAt time.Time)

// NowPlayingCallback is invoked when a track starts playing.
type NowPlayingCallback func(track Track)

// Tracker monitors playback status changes and calculates listened duration.
type Tracker struct {
	config       Config
	session      *PlaySession
	onNowPlaying NowPlayingCallback
	onScrobble   ScrobbleCallback
}

// NewTracker creates a new Tracker instance.
func NewTracker(cfg Config, onNowPlaying NowPlayingCallback, onScrobble ScrobbleCallback) *Tracker {
	if cfg.MinTrackDuration <= 0 {
		cfg.MinTrackDuration = 30 * time.Second
	}
	if cfg.ScrobblePercent <= 0 || cfg.ScrobblePercent > 100 {
		cfg.ScrobblePercent = 50
	}
	if cfg.MaxDuration <= 0 {
		cfg.MaxDuration = 240 * time.Second
	}

	return &Tracker{
		config:       cfg,
		onNowPlaying: onNowPlaying,
		onScrobble:   onScrobble,
	}
}

// ParseTrack extracts Track metadata from MPD CurrentSong attributes.
func ParseTrack(attrs map[string]string) (Track, bool) {
	if len(attrs) == 0 {
		return Track{}, false
	}

	artist := attrs["Artist"]
	if artist == "" {
		artist = attrs["AlbumArtist"]
	}
	title := attrs["Title"]

	// Parse duration: MPD provides 'duration' (float) and 'Time' (int)
	var dur time.Duration
	if dStr, ok := attrs["duration"]; ok && dStr != "" {
		if dSec, err := strconv.ParseFloat(dStr, 64); err == nil && dSec > 0 {
			dur = time.Duration(dSec * float64(time.Second))
		}
	}
	if dur == 0 {
		if tStr, ok := attrs["Time"]; ok && tStr != "" {
			if tSec, err := strconv.Atoi(tStr); err == nil && tSec > 0 {
				dur = time.Duration(tSec) * time.Second
			}
		}
	}

	// Parse track number
	trackNum := 0
	if trkStr, ok := attrs["Track"]; ok && trkStr != "" {
		// "1/12" or "01"
		part := strings.Split(trkStr, "/")[0]
		if n, err := strconv.Atoi(part); err == nil {
			trackNum = n
		}
	}

	track := Track{
		ID:          attrs["Id"],
		File:        attrs["file"],
		Artist:      strings.TrimSpace(artist),
		Title:       strings.TrimSpace(title),
		Album:       strings.TrimSpace(attrs["Album"]),
		AlbumArtist: strings.TrimSpace(attrs["AlbumArtist"]),
		TrackNumber: trackNum,
		Duration:    dur,
	}

	return track, track.Artist != "" && track.Title != ""
}

// ParseDuration extracts duration from attributes (status or song metadata).
func ParseDuration(attrs map[string]string) time.Duration {
	if len(attrs) == 0 {
		return 0
	}
	if dStr, ok := attrs["duration"]; ok && dStr != "" {
		if dSec, err := strconv.ParseFloat(dStr, 64); err == nil && dSec > 0 {
			return time.Duration(dSec * float64(time.Second))
		}
	}
	if tStr, ok := attrs["Time"]; ok && tStr != "" {
		if tSec, err := strconv.Atoi(tStr); err == nil && tSec > 0 {
			return time.Duration(tSec) * time.Second
		}
	}
	if timeStr, ok := attrs["time"]; ok && timeStr != "" {
		parts := strings.Split(timeStr, ":")
		if len(parts) > 1 {
			if totalSec, err := strconv.Atoi(parts[1]); err == nil && totalSec > 0 {
				return time.Duration(totalSec) * time.Second
			}
		}
	}
	return 0
}

// ParseElapsed parses elapsed time from MPD status attributes.
func ParseElapsed(attrs map[string]string) time.Duration {
	if eStr, ok := attrs["elapsed"]; ok && eStr != "" {
		if sec, err := strconv.ParseFloat(eStr, 64); err == nil && sec >= 0 {
			return time.Duration(sec * float64(time.Second))
		}
	}
	if tStr, ok := attrs["time"]; ok && tStr != "" {
		parts := strings.Split(tStr, ":")
		if len(parts) > 0 {
			if sec, err := strconv.Atoi(parts[0]); err == nil && sec >= 0 {
				return time.Duration(sec) * time.Second
			}
		}
	}
	return 0
}

// CurrentSession returns a copy of the active playback session, or nil.
func (t *Tracker) CurrentSession() *PlaySession {
	if t.session == nil {
		return nil
	}
	cpy := *t.session
	return &cpy
}

// UpdateSong updates the tracker with newly queried MPD song and status.
func (t *Tracker) UpdateSong(songAttrs, statusAttrs map[string]string) {
	state := statusAttrs["state"]
	elapsed := ParseElapsed(statusAttrs)
	newTrack, valid := ParseTrack(songAttrs)

	// If MPD is stopped or no valid song is playing
	if state == "stop" || !valid {
		t.finalizeCurrentSession()
		t.session = nil
		return
	}

	// Fallback duration from status if song metadata didn't have duration tag
	if newTrack.Duration == 0 {
		newTrack.Duration = ParseDuration(statusAttrs)
	}

	// Check if this is the same track as currently tracked
	if t.session != nil && t.isSameTrack(t.session.Track, newTrack) {
		// Same track, update playback progress
		t.onPlaybackUpdate(state, elapsed, statusAttrs)
		return
	}

	// Track has changed: finalize previous session
	t.finalizeCurrentSession()

	// Start new session
	t.session = &PlaySession{
		Track:            newTrack,
		StartedAt:        time.Now().Add(-elapsed),
		LastElapsed:      elapsed,
		ListenedDuration: elapsed,
		NowPlayingSent:   false,
		Scrobbled:        false,
	}

	if t.config.Verbose {
		log.Printf("[tracker] New track detected: %s - %s (%v)", newTrack.Artist, newTrack.Title, newTrack.Duration)
	}

	if state == "play" {
		t.sendNowPlaying()
	}

	t.checkAndScrobble()
}

// OnTick is called on periodic timer intervals to track elapsed time while playing.
func (t *Tracker) OnTick(statusAttrs map[string]string) {
	state := statusAttrs["state"]
	elapsed := ParseElapsed(statusAttrs)
	t.onPlaybackUpdate(state, elapsed, statusAttrs)
}

func (t *Tracker) onPlaybackUpdate(state string, elapsed time.Duration, statusAttrs map[string]string) {
	if t.session == nil {
		return
	}

	if t.session.Track.Duration == 0 && len(statusAttrs) > 0 {
		t.session.Track.Duration = ParseDuration(statusAttrs)
	}

	if state != "play" {
		// Paused or stopped; do not accumulate listened duration
		t.session.LastElapsed = elapsed
		return
	}

	// If Now Playing hasn't been sent yet (e.g. unpaused into play), send it
	if !t.session.NowPlayingSent {
		t.sendNowPlaying()
	}

	delta := elapsed - t.session.LastElapsed

	// Detect track replay/loop (user restarted track or player looped to beginning)
	if elapsed < 2*time.Second && t.session.LastElapsed > 5*time.Second {
		if t.config.Verbose {
			log.Printf("[tracker] Track restart detected for %s - %s", t.session.Track.Artist, t.session.Track.Title)
		}
		t.finalizeCurrentSession()
		t.session.StartedAt = time.Now().Add(-elapsed)
		t.session.LastElapsed = elapsed
		t.session.ListenedDuration = elapsed
		t.session.NowPlayingSent = false
		t.session.Scrobbled = false
		t.sendNowPlaying()
		t.checkAndScrobble()
		return
	}

	// Normal forward progression: delta between 0 and 5 seconds
	if delta > 0 && delta <= 5*time.Second {
		t.session.ListenedDuration += delta
	} else if math.Abs(float64(delta)) > float64(5*time.Second) {
		// User sought (forward or backward) - update position without artificially adding skipped time
		if t.config.Verbose {
			log.Printf("[tracker] Seek detected from %v to %v", t.session.LastElapsed, elapsed)
		}
	}

	t.session.LastElapsed = elapsed
	t.checkAndScrobble()
}

func (t *Tracker) isSameTrack(a, b Track) bool {
	if a.ID != "" && b.ID != "" {
		return a.ID == b.ID
	}
	if a.File != "" && b.File != "" {
		return a.File == b.File
	}
	return a.Artist == b.Artist && a.Title == b.Title && a.Album == b.Album
}

func (t *Tracker) sendNowPlaying() {
	if t.session == nil || t.session.NowPlayingSent {
		return
	}
	t.session.NowPlayingSent = true
	if t.onNowPlaying != nil {
		t.onNowPlaying(t.session.Track)
	}
}

// ScrobbleThreshold calculates how much playback time is needed before scrobbling.
func (t *Tracker) ScrobbleThreshold(dur time.Duration) time.Duration {
	if dur <= 0 {
		return t.config.MaxDuration
	}
	percentDur := time.Duration(float64(dur) * float64(t.config.ScrobblePercent) / 100.0)
	if percentDur < t.config.MaxDuration {
		return percentDur
	}
	return t.config.MaxDuration
}

// IsEligible returns true if the track meets minimum length and tag requirements.
func (t *Tracker) IsEligible(track Track) bool {
	if track.Artist == "" || track.Title == "" {
		return false
	}
	if track.Duration > 0 && track.Duration < t.config.MinTrackDuration {
		return false
	}
	return true
}

func (t *Tracker) checkAndScrobble() {
	if t.session == nil || t.session.Scrobbled {
		return
	}

	if !t.IsEligible(t.session.Track) {
		return
	}

	threshold := t.ScrobbleThreshold(t.session.Track.Duration)
	if t.session.ListenedDuration >= threshold {
		t.session.Scrobbled = true
		if t.config.Verbose {
			log.Printf("[tracker] Scrobble condition met: listened %v >= threshold %v (%s - %s)",
				t.session.ListenedDuration, threshold, t.session.Track.Artist, t.session.Track.Title)
		}
		if t.onScrobble != nil {
			t.onScrobble(t.session.Track, t.session.StartedAt)
		}
	}
}

func (t *Tracker) finalizeCurrentSession() {
	if t.session == nil {
		return
	}
	// Check one last time before discarding session
	t.checkAndScrobble()
}

// Shutdown finalizes the tracker when shutting down.
func (t *Tracker) Shutdown() {
	t.finalizeCurrentSession()
	t.session = nil
}
