package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/csmith/envflag/v2"
	"github.com/some-programs/mpd-scrobbler/pkg/config"
	"github.com/some-programs/mpd-scrobbler/pkg/daemon"
	"github.com/some-programs/mpd-scrobbler/pkg/lastfm"
	"github.com/some-programs/mpd-scrobbler/pkg/tracker"
)

func findConfigPath() string {
	if envPath := os.Getenv("CONFIG"); envPath != "" {
		return envPath
	}
	for i, arg := range os.Args {
		if arg == "-config" || arg == "--config" {
			if i+1 < len(os.Args) {
				return os.Args[i+1]
			}
		} else if strings.HasPrefix(arg, "-config=") || strings.HasPrefix(arg, "--config=") {
			return strings.SplitN(arg, "=", 2)[1]
		}
	}
	return ""
}

func main() {
	runtime.GOMAXPROCS(1)
	// Pre-load TOML config file if available (from -config flag, CONFIG env, or default paths)
	specifiedConfigPath := findConfigPath()
	fileCfg, loadedPath, err := config.LoadFile(specifiedConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Set initial defaults using helper getters from FileConfig (handles root keys & sectioned tables)
	defaultMPDNetwork := fileCfg.GetMPDNetwork()
	defaultMPDAddress := fileCfg.GetMPDAddress()
	defaultMPDPassword := fileCfg.GetMPDPassword()
	defaultMPDReconnect := fileCfg.GetMPDReconnectInterval()

	defaultLastfmApiKey := fileCfg.GetAPIKey()
	defaultLastfmApiSecret := fileCfg.GetAPISecret()
	defaultLastfmSessionKey := fileCfg.GetSessionKey()
	defaultLastfmUsername := fileCfg.GetUsername()
	defaultLastfmPassword := fileCfg.GetPassword()

	defaultScrobbleMinLength := fileCfg.GetScrobbleMinLength()
	defaultScrobbleMinPercent := fileCfg.GetScrobbleMinPercent()
	defaultScrobbleMaxDuration := fileCfg.GetScrobbleMaxDuration()
	defaultScrobbleNowPlaying := fileCfg.GetScrobbleNowPlaying()
	defaultBacklogFile := fileCfg.GetBacklogFile()
	if defaultBacklogFile == "" {
		defaultBacklogFile = lastfm.DefaultBacklogPath()
	}
	defaultBacklogLimit := fileCfg.GetBacklogLimit()

	defaultDryRun := fileCfg.GetDryRun()
	defaultVerbose := fileCfg.GetVerbose()

	// Configuration flags (defaults populated from TOML, overridable by env and CLI flags)
	_ = flag.String("config", loadedPath, "Path to TOML configuration file")

	// MPD flags
	mpdNetwork := flag.String("mpd.network", defaultMPDNetwork, "MPD network type ('tcp' or 'unix')")
	mpdAddress := flag.String("mpd.address", defaultMPDAddress, "MPD address (host:port or unix socket path)")
	mpdPassword := flag.String("mpd.password", defaultMPDPassword, "MPD password (if required)")
	mpdReconnect := flag.Duration("mpd.reconnect-interval", defaultMPDReconnect, "Delay between MPD reconnect attempts")

	// Last.fm flags
	lastfmApiKey := flag.String("lastfm.api-key", defaultLastfmApiKey, "Last.fm API key")
	lastfmApiSecret := flag.String("lastfm.api-secret", defaultLastfmApiSecret, "Last.fm API secret")
	lastfmSessionKey := flag.String("lastfm.session-key", defaultLastfmSessionKey, "Last.fm session key")
	lastfmUsername := flag.String("lastfm.username", defaultLastfmUsername, "Last.fm username (for mobile session auth)")
	lastfmPassword := flag.String("lastfm.password", defaultLastfmPassword, "Last.fm password (for mobile session auth)")
	lastfmAuth := flag.Bool("lastfm.auth", false, "Run interactive terminal authorization to generate a Last.fm session key")

	// Scrobbler flags
	scrobbleMinLength := flag.Duration("scrobble.min-length", defaultScrobbleMinLength, "Minimum song duration required to scrobble")
	scrobbleMinPercent := flag.Int("scrobble.min-percent", defaultScrobbleMinPercent, "Percentage of song length required before scrobbling (1-100)")
	scrobbleMaxDuration := flag.Duration("scrobble.max-duration", defaultScrobbleMaxDuration, "Maximum playback time required before scrobbling")
	scrobbleNowPlaying := flag.Bool("scrobble.now-playing", defaultScrobbleNowPlaying, "Send 'Now Playing' notifications to Last.fm")
	scrobbleBacklogFile := flag.String("scrobble.backlog-file", defaultBacklogFile, "Path to persistent offline scrobble backlog file")
	scrobbleBacklogLimit := flag.Int("scrobble.backlog-limit", defaultBacklogLimit, "Maximum number of scrobbles to retain in offline backlog")

	// Operational flags
	dryRun := flag.Bool("dry-run", defaultDryRun, "Simulate tracking and scrobbling without sending requests to Last.fm")
	verbose := flag.Bool("verbose", defaultVerbose, "Enable verbose debug logging")

	// Parse flags and environment variables via envflag
	envflag.Parse()

	// If interactive auth was requested, run it and exit
	if *lastfmAuth {
		if err := lastfm.InteractiveAuth(*lastfmApiKey, *lastfmApiSecret, loadedPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error during Last.fm authorization: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if loadedPath != "" && *verbose {
		log.Printf("[mpd-scrobbler] Loaded configuration from %s", loadedPath)
	}

	lastfmCfg := lastfm.Config{
		APIKey:       *lastfmApiKey,
		APISecret:    *lastfmApiSecret,
		SessionKey:   *lastfmSessionKey,
		Username:     *lastfmUsername,
		Password:     *lastfmPassword,
		DryRun:       *dryRun,
		Verbose:      *verbose,
		BacklogFile:  *scrobbleBacklogFile,
		BacklogLimit: *scrobbleBacklogLimit,
	}

	scrobblerClient, err := lastfm.NewClient(lastfmCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n[mpd-scrobbler] Configuration error: %v\n\n", err)
		fmt.Fprintf(os.Stderr, "Configure via TOML file (~/.config/mpd-scrobbler/config.toml or -config <path>),\n")
		fmt.Fprintf(os.Stderr, "or via environment variables (LASTFM_API_KEY, LASTFM_API_SECRET, LASTFM_SESSION_KEY),\n")
		fmt.Fprintf(os.Stderr, "or generate a session key interactively:\n")
		fmt.Fprintf(os.Stderr, "  mpd-scrobbler -lastfm.api-key=<KEY> -lastfm.api-secret=<SECRET> -lastfm.auth\n\n")
		fmt.Fprintf(os.Stderr, "For dry-run testing with MPD without Last.fm credentials, use -dry-run.\n\n")
		os.Exit(1)
	}

	daemonCfg := daemon.Config{
		MPDNetwork:           *mpdNetwork,
		MPDAddress:           *mpdAddress,
		MPDPassword:          *mpdPassword,
		MPDReconnectInterval: *mpdReconnect,
		LastFM:               lastfmCfg,
		Tracker: tracker.Config{
			MinTrackDuration: *scrobbleMinLength,
			ScrobblePercent:  *scrobbleMinPercent,
			MaxDuration:      *scrobbleMaxDuration,
			Verbose:          *verbose,
		},
		SendNowPlaying: *scrobbleNowPlaying,
		PollInterval:   1 * time.Second,
		Verbose:        *verbose,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	d := daemon.New(daemonCfg, scrobblerClient)
	if err := d.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("[mpd-scrobbler] Fatal daemon error: %v", err)
	}
}
