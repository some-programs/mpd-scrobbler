package tracker

import (
	"strconv"
	"testing"
	"time"
)

func TestParseTrack(t *testing.T) {
	attrs := map[string]string{
		"Id":          "42",
		"file":        "Radiohead/OK Computer/01 - Airbag.flac",
		"Artist":      "Radiohead",
		"Title":       "Airbag",
		"Album":       "OK Computer",
		"AlbumArtist": "Radiohead",
		"Track":       "1/12",
		"duration":    "284.133",
	}

	track, valid := ParseTrack(attrs)
	if !valid {
		t.Fatalf("expected track to be valid")
	}
	if track.ID != "42" || track.Artist != "Radiohead" || track.Title != "Airbag" {
		t.Errorf("unexpected track metadata: %+v", track)
	}
	if track.TrackNumber != 1 {
		t.Errorf("expected track number 1, got %d", track.TrackNumber)
	}
	if track.Duration < 284*time.Second || track.Duration > 285*time.Second {
		t.Errorf("expected duration ~284s, got %v", track.Duration)
	}
}

func TestParseTrack_FallbackAlbumArtist(t *testing.T) {
	attrs := map[string]string{
		"Title":       "Track Without Artist",
		"AlbumArtist": "Various Artists",
		"Time":        "120",
	}

	track, valid := ParseTrack(attrs)
	if !valid {
		t.Fatalf("expected track with AlbumArtist fallback to be valid")
	}
	if track.Artist != "Various Artists" {
		t.Errorf("expected artist 'Various Artists', got '%s'", track.Artist)
	}
	if track.Duration != 120*time.Second {
		t.Errorf("expected duration 120s, got %v", track.Duration)
	}
}

func TestParseTrack_Invalid(t *testing.T) {
	_, valid := ParseTrack(map[string]string{"Artist": "Only Artist"})
	if valid {
		t.Errorf("expected track without Title to be invalid")
	}

	_, valid = ParseTrack(map[string]string{"Title": "Only Title"})
	if valid {
		t.Errorf("expected track without Artist to be invalid")
	}

	_, valid = ParseTrack(nil)
	if valid {
		t.Errorf("expected empty track to be invalid")
	}
}

func TestScrobbleThreshold(t *testing.T) {
	tr := NewTracker(DefaultConfig(), nil, nil)

	// 100s song -> 50% = 50s
	if th := tr.ScrobbleThreshold(100 * time.Second); th != 50*time.Second {
		t.Errorf("expected 50s, got %v", th)
	}

	// 600s song (10 min) -> 50% = 300s, but max is 240s
	if th := tr.ScrobbleThreshold(600 * time.Second); th != 240*time.Second {
		t.Errorf("expected 240s, got %v", th)
	}
}

func TestTracker_NormalPlayAndScrobble(t *testing.T) {
	var nowPlayingTrack *Track
	var scrobbledTrack *Track

	tr := NewTracker(DefaultConfig(),
		func(track Track) { nowPlayingTrack = &track },
		func(track Track, startedAt time.Time) { scrobbledTrack = &track },
	)

	songAttrs := map[string]string{
		"Id":       "1",
		"Artist":   "Daft Punk",
		"Title":    "Around The World",
		"duration": "240.0",
	}

	// Track starts playing at elapsed 0
	tr.UpdateSong(songAttrs, map[string]string{"state": "play", "elapsed": "0.0"})

	if nowPlayingTrack == nil || nowPlayingTrack.Title != "Around The World" {
		t.Fatalf("expected NowPlaying to be sent")
	}
	if scrobbledTrack != nil {
		t.Fatalf("track should not be scrobbled immediately")
	}

	// Simulate ticking for 60 seconds (half of 240s is 120s)
	for i := 1; i <= 60; i++ {
		tr.OnTick(map[string]string{"state": "play", "elapsed": strconv.Itoa(i)})
	}
	if scrobbledTrack != nil {
		t.Fatalf("track should not be scrobbled at 60s")
	}

	// Tick to 120 seconds (threshold reached)
	for i := 61; i <= 120; i++ {
		tr.OnTick(map[string]string{"state": "play", "elapsed": strconv.Itoa(i)})
	}
	if scrobbledTrack == nil || scrobbledTrack.Title != "Around The World" {
		t.Fatalf("expected track to be scrobbled at 120s")
	}

	// Tick further to 240s - should not scrobble a second time
	scrobbledTrack = nil
	for i := 121; i <= 240; i++ {
		tr.OnTick(map[string]string{"state": "play", "elapsed": strconv.Itoa(i)})
	}
	if scrobbledTrack != nil {
		t.Fatalf("track should not be double scrobbled")
	}
}

func TestTracker_PauseHandling(t *testing.T) {
	var scrobbled bool
	tr := NewTracker(DefaultConfig(), nil, func(track Track, startedAt time.Time) {
		scrobbled = true
	})

	songAttrs := map[string]string{
		"Id":       "2",
		"Artist":   "Pink Floyd",
		"Title":    "Time",
		"duration": "100.0", // 50% = 50s
	}

	tr.UpdateSong(songAttrs, map[string]string{"state": "play", "elapsed": "0.0"})

	// Play 30 seconds
	for i := 1; i <= 30; i++ {
		tr.OnTick(map[string]string{"state": "play", "elapsed": strconv.Itoa(i)})
	}
	if scrobbled {
		t.Fatalf("should not scrobble at 30s")
	}

	// Paused for 10 ticks at elapsed 30
	for i := 0; i < 10; i++ {
		tr.OnTick(map[string]string{"state": "pause", "elapsed": "30.0"})
	}
	if scrobbled {
		t.Fatalf("should not scrobble while paused")
	}

	// Resume play from 30 to 49
	for i := 31; i <= 49; i++ {
		tr.OnTick(map[string]string{"state": "play", "elapsed": strconv.Itoa(i)})
	}
	if scrobbled {
		t.Fatalf("should not scrobble at 49s")
	}

	// At 50s: threshold reached
	tr.OnTick(map[string]string{"state": "play", "elapsed": "50.0"})
	if !scrobbled {
		t.Fatalf("should have scrobbled at 50s")
	}
}

func TestTracker_SeekDoesNotArtificiallyScrobble(t *testing.T) {
	var scrobbled bool
	tr := NewTracker(DefaultConfig(), nil, func(track Track, startedAt time.Time) {
		scrobbled = true
	})

	songAttrs := map[string]string{
		"Id":       "3",
		"Artist":   "Queen",
		"Title":    "Bohemian Rhapsody",
		"duration": "360.0", // threshold = 180s
	}

	tr.UpdateSong(songAttrs, map[string]string{"state": "play", "elapsed": "0.0"})

	// Play 5 seconds
	for i := 1; i <= 5; i++ {
		tr.OnTick(map[string]string{"state": "play", "elapsed": strconv.Itoa(i)})
	}

	// User seeks directly from 5s to 200s!
	tr.OnTick(map[string]string{"state": "play", "elapsed": "200.0"})

	// Only 5 seconds of actual audio was listened to, should NOT scrobble
	if scrobbled {
		t.Fatalf("scrobble triggered after seek without actual playback time")
	}
}

func TestTracker_ShortTrackNotEligible(t *testing.T) {
	var scrobbled bool
	tr := NewTracker(DefaultConfig(), nil, func(track Track, startedAt time.Time) {
		scrobbled = true
	})

	songAttrs := map[string]string{
		"Id":       "4",
		"Artist":   "Intro",
		"Title":    "Short Intro",
		"duration": "20.0", // Less than 30s minimum
	}

	tr.UpdateSong(songAttrs, map[string]string{"state": "play", "elapsed": "0.0"})

	for i := 1; i <= 20; i++ {
		tr.OnTick(map[string]string{"state": "play", "elapsed": strconv.Itoa(i)})
	}
	tr.UpdateSong(nil, map[string]string{"state": "stop"})

	if scrobbled {
		t.Fatalf("track under 30s must not be scrobbled")
	}
}

func TestTracker_TrackSkipBeforeThreshold(t *testing.T) {
	var scrobbled bool
	tr := NewTracker(DefaultConfig(), nil, func(track Track, startedAt time.Time) {
		scrobbled = true
	})

	song1 := map[string]string{"Id": "5", "Artist": "Band", "Title": "Song 1", "duration": "200.0"}
	song2 := map[string]string{"Id": "6", "Artist": "Band", "Title": "Song 2", "duration": "200.0"}

	tr.UpdateSong(song1, map[string]string{"state": "play", "elapsed": "0.0"})

	// Play for 40s (threshold is 100s)
	for i := 1; i <= 40; i++ {
		tr.OnTick(map[string]string{"state": "play", "elapsed": strconv.Itoa(i)})
	}

	// User skips to song2
	tr.UpdateSong(song2, map[string]string{"state": "play", "elapsed": "0.0"})

	if scrobbled {
		t.Fatalf("skipped track should not have scrobbled")
	}
}

func TestTracker_DurationFallbackFromStatus(t *testing.T) {
	var scrobbledTrack *Track
	tr := NewTracker(DefaultConfig(), nil, func(track Track, startedAt time.Time) {
		scrobbledTrack = &track
	})

	// Song attrs without duration (like SID/chiptunes)
	songWithoutDuration := map[string]string{
		"Id":     "7",
		"Artist": "Chiptune Artist",
		"Title":  "Chip Track",
	}

	// Status contains duration = 100s
	status := map[string]string{
		"state":    "play",
		"elapsed":  "0.0",
		"duration": "100.0",
	}

	tr.UpdateSong(songWithoutDuration, status)

	sess := tr.CurrentSession()
	if sess == nil {
		t.Fatalf("expected active session")
	}
	if sess.Track.Duration != 100*time.Second {
		t.Fatalf("expected track duration 100s from status, got %v", sess.Track.Duration)
	}

	// Play to 50s (50% threshold)
	for i := 1; i <= 50; i++ {
		tr.OnTick(map[string]string{"state": "play", "elapsed": strconv.Itoa(i), "duration": "100.0"})
	}

	if scrobbledTrack == nil {
		t.Fatalf("expected track to be scrobbled at 50s with fallback duration")
	}
}
