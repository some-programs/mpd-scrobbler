package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Duration wraps time.Duration to support TOML unmarshaling from strings like "5s", "4m".
type Duration time.Duration

func (d *Duration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// MPDConfig holds MPD connection settings.
type MPDConfig struct {
	Network           string   `toml:"network"`
	Address           string   `toml:"address"`
	Password          string   `toml:"password"`
	ReconnectInterval Duration `toml:"reconnect_interval"`
}

// LastFMConfig holds Last.fm API and authentication settings.
type LastFMConfig struct {
	APIKey     string `toml:"api_key"`
	APISecret  string `toml:"api_secret"`
	SessionKey string `toml:"session_key"`
	Username   string `toml:"username"`
	Password   string `toml:"password"`
}

// ScrobbleConfig holds scrobbler criteria and backlog settings.
type ScrobbleConfig struct {
	MinLength    Duration `toml:"min_length"`
	MinPercent   int      `toml:"min_percent"`
	MaxDuration  Duration `toml:"max_duration"`
	NowPlaying   bool     `toml:"now_playing"`
	BacklogFile  string   `toml:"backlog_file"`
	BacklogLimit int      `toml:"backlog_limit"`
}

// GeneralConfig holds operational settings.
type GeneralConfig struct {
	DryRun  bool `toml:"dry_run"`
	Verbose bool `toml:"verbose"`
}

// FileConfig defines the TOML file structure supporting section tables.
type FileConfig struct {
	MPD      MPDConfig      `toml:"mpd"`
	LastFM   LastFMConfig   `toml:"lastfm"`
	Scrobble ScrobbleConfig `toml:"scrobble"`
	General  GeneralConfig  `toml:"general"`
}

// DefaultFileConfig returns a FileConfig populated with default settings.
func DefaultFileConfig() FileConfig {
	return FileConfig{
		MPD: MPDConfig{
			Network:           "tcp",
			Address:           "127.0.0.1:6600",
			ReconnectInterval: Duration(5 * time.Second),
		},
		Scrobble: ScrobbleConfig{
			MinLength:    Duration(30 * time.Second),
			MinPercent:   50,
			MaxDuration:  Duration(240 * time.Second),
			NowPlaying:   true,
			BacklogLimit: 5000,
		},
		General: GeneralConfig{
			DryRun:  false,
			Verbose: false,
		},
	}
}



// DefaultConfigPath returns the default configuration path in XDG_CONFIG_HOME or ~/.config.
func DefaultConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configDir = filepath.Join(homeDir, ".config")
		}
	}
	if configDir != "" {
		return filepath.Join(configDir, "mpd-scrobbler", "config.toml")
	}
	return "config.toml"
}

// LoadFile reads and parses a TOML configuration file.
func LoadFile(path string) (FileConfig, string, error) {
	explicit := path != ""
	if !explicit {
		candidates := []string{
			DefaultConfigPath(),
			"config.toml",
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}

	cfg := DefaultFileConfig()
	if path == "" {
		return cfg, "", nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if explicit {
			return cfg, "", fmt.Errorf("failed to read config file '%s': %w", path, err)
		}
		if errors.Is(err, os.ErrNotExist) {
			return cfg, "", nil
		}
		return cfg, "", fmt.Errorf("failed to read config file '%s': %w", path, err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, path, fmt.Errorf("failed to parse TOML config file '%s': %w", path, err)
	}

	return cfg, path, nil
}

// SaveSessionKey updates or appends session_key in the specified config file.
func SaveSessionKey(filePath string, sessionKey string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "session_key") ||
			strings.HasPrefix(trimmed, "# session_key") ||
			strings.HasPrefix(trimmed, "#session_key") ||
			strings.HasPrefix(trimmed, "session-key") ||
			strings.HasPrefix(trimmed, "# session-key") {
			lines[i] = fmt.Sprintf("session_key = \"%s\"", sessionKey)
			found = true
			break
		}
	}
	if !found {
		lastfmIdx := -1
		for i, line := range lines {
			if strings.TrimSpace(line) == "[lastfm]" {
				lastfmIdx = i
				break
			}
		}
		if lastfmIdx != -1 {
			newLines := make([]string, 0, len(lines)+1)
			newLines = append(newLines, lines[:lastfmIdx+1]...)
			newLines = append(newLines, fmt.Sprintf("session_key = \"%s\"", sessionKey))
			newLines = append(newLines, lines[lastfmIdx+1:]...)
			lines = newLines
		} else {
			lines = append(lines, "", "[lastfm]", fmt.Sprintf("session_key = \"%s\"", sessionKey))
		}
	}
	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0600)
}
