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

// FileConfig defines the TOML file structure supporting both root-level keys and section tables.
type FileConfig struct {
	// Root-level fields (convenience for flat config files)
	APIKey             *string `toml:"api_key"`
	APIKeyHyphen       *string `toml:"api-key"`
	APIKeyPlain        *string `toml:"apikey"`
	APISecret          *string `toml:"api_secret"`
	APISecretHyphen    *string `toml:"api-secret"`
	APISecretPlain     *string `toml:"apisecret"`
	SessionKey         *string `toml:"session_key"`
	SessionKeyHyphen   *string `toml:"session-key"`
	SessionKeyPlain    *string `toml:"sessionkey"`
	Username           *string `toml:"username"`
	Password           *string `toml:"password"`
	Address            *string `toml:"address"`
	MPDAddress         *string `toml:"mpd_address"`
	MPDAddressHyphen   *string `toml:"mpd-address"`
	Network            *string `toml:"network"`
	MPDNetwork         *string `toml:"mpd_network"`
	MPDNetworkHyphen   *string `toml:"mpd-network"`
	BacklogFile        *string `toml:"backlog_file"`
	BacklogFileHyphen  *string `toml:"backlog-file"`
	BacklogLimit       *int    `toml:"backlog_limit"`
	BacklogLimitHyphen *int    `toml:"backlog-limit"`
	DryRun             *bool   `toml:"dry_run"`
	Verbose            *bool   `toml:"verbose"`

	MPD struct {
		Network                 *string   `toml:"network"`
		Address                 *string   `toml:"address"`
		Password                *string   `toml:"password"`
		ReconnectInterval       *Duration `toml:"reconnect_interval"`
		ReconnectIntervalHyphen *Duration `toml:"reconnect-interval"`
	} `toml:"mpd"`

	LastFM struct {
		APIKey           *string `toml:"api_key"`
		APIKeyHyphen     *string `toml:"api-key"`
		APIKeyPlain      *string `toml:"apikey"`
		APISecret        *string `toml:"api_secret"`
		APISecretHyphen  *string `toml:"api-secret"`
		APISecretPlain   *string `toml:"apisecret"`
		SessionKey       *string `toml:"session_key"`
		SessionKeyHyphen *string `toml:"session-key"`
		SessionKeyPlain  *string `toml:"sessionkey"`
		Username         *string `toml:"username"`
		Password         *string `toml:"password"`
	} `toml:"lastfm"`

	Scrobble struct {
		MinLength          *Duration `toml:"min_length"`
		MinLengthHyphen    *Duration `toml:"min-length"`
		MinPercent         *int      `toml:"min_percent"`
		MinPercentHyphen   *int      `toml:"min-percent"`
		MaxDuration        *Duration `toml:"max_duration"`
		MaxDurationHyphen  *Duration `toml:"max-duration"`
		NowPlaying         *bool     `toml:"now_playing"`
		NowPlayingHyphen   *bool     `toml:"now-playing"`
		BacklogFile        *string   `toml:"backlog_file"`
		BacklogFileHyphen  *string   `toml:"backlog-file"`
		BacklogLimit       *int      `toml:"backlog_limit"`
		BacklogLimitHyphen *int      `toml:"backlog-limit"`
	} `toml:"scrobble"`

	General struct {
		DryRun       *bool `toml:"dry_run"`
		DryRunHyphen *bool `toml:"dry-run"`
		Verbose      *bool `toml:"verbose"`
	} `toml:"general"`
}

func (c *FileConfig) GetAPIKey() string {
	for _, val := range []*string{
		c.LastFM.APIKey, c.LastFM.APIKeyHyphen, c.LastFM.APIKeyPlain,
		c.APIKey, c.APIKeyHyphen, c.APIKeyPlain,
	} {
		if val != nil && *val != "" {
			return *val
		}
	}
	return ""
}

func (c *FileConfig) GetAPISecret() string {
	for _, val := range []*string{
		c.LastFM.APISecret, c.LastFM.APISecretHyphen, c.LastFM.APISecretPlain,
		c.APISecret, c.APISecretHyphen, c.APISecretPlain,
	} {
		if val != nil && *val != "" {
			return *val
		}
	}
	return ""
}

func (c *FileConfig) GetSessionKey() string {
	for _, val := range []*string{
		c.LastFM.SessionKey, c.LastFM.SessionKeyHyphen, c.LastFM.SessionKeyPlain,
		c.SessionKey, c.SessionKeyHyphen, c.SessionKeyPlain,
	} {
		if val != nil && *val != "" {
			return *val
		}
	}
	return ""
}

func (c *FileConfig) GetUsername() string {
	for _, val := range []*string{c.LastFM.Username, c.Username} {
		if val != nil && *val != "" {
			return *val
		}
	}
	return ""
}

func (c *FileConfig) GetPassword() string {
	for _, val := range []*string{c.LastFM.Password, c.Password} {
		if val != nil && *val != "" {
			return *val
		}
	}
	return ""
}

func (c *FileConfig) GetMPDNetwork() string {
	for _, val := range []*string{c.MPD.Network, c.Network, c.MPDNetwork, c.MPDNetworkHyphen} {
		if val != nil && *val != "" {
			return *val
		}
	}
	return "tcp"
}

func (c *FileConfig) GetMPDAddress() string {
	for _, val := range []*string{c.MPD.Address, c.Address, c.MPDAddress, c.MPDAddressHyphen} {
		if val != nil && *val != "" {
			return *val
		}
	}
	return "127.0.0.1:6600"
}

func (c *FileConfig) GetMPDPassword() string {
	if c.MPD.Password != nil {
		return *c.MPD.Password
	}
	return ""
}

func (c *FileConfig) GetMPDReconnectInterval() time.Duration {
	for _, val := range []*Duration{c.MPD.ReconnectInterval, c.MPD.ReconnectIntervalHyphen} {
		if val != nil && val.Duration() > 0 {
			return val.Duration()
		}
	}
	return 5 * time.Second
}

func (c *FileConfig) GetScrobbleMinLength() time.Duration {
	for _, val := range []*Duration{c.Scrobble.MinLength, c.Scrobble.MinLengthHyphen} {
		if val != nil && val.Duration() > 0 {
			return val.Duration()
		}
	}
	return 30 * time.Second
}

func (c *FileConfig) GetScrobbleMinPercent() int {
	for _, val := range []*int{c.Scrobble.MinPercent, c.Scrobble.MinPercentHyphen} {
		if val != nil && *val > 0 {
			return *val
		}
	}
	return 50
}

func (c *FileConfig) GetScrobbleMaxDuration() time.Duration {
	for _, val := range []*Duration{c.Scrobble.MaxDuration, c.Scrobble.MaxDurationHyphen} {
		if val != nil && val.Duration() > 0 {
			return val.Duration()
		}
	}
	return 240 * time.Second
}

func (c *FileConfig) GetScrobbleNowPlaying() bool {
	for _, val := range []*bool{c.Scrobble.NowPlaying, c.Scrobble.NowPlayingHyphen} {
		if val != nil {
			return *val
		}
	}
	return true
}

func (c *FileConfig) GetDryRun() bool {
	for _, val := range []*bool{c.General.DryRun, c.General.DryRunHyphen, c.DryRun} {
		if val != nil {
			return *val
		}
	}
	return false
}

func (c *FileConfig) GetVerbose() bool {
	for _, val := range []*bool{c.General.Verbose, c.Verbose} {
		if val != nil {
			return *val
		}
	}
	return false
}

func (c *FileConfig) GetBacklogFile() string {
	for _, val := range []*string{c.Scrobble.BacklogFile, c.Scrobble.BacklogFileHyphen, c.BacklogFile, c.BacklogFileHyphen} {
		if val != nil && *val != "" {
			return *val
		}
	}
	return ""
}

func (c *FileConfig) GetBacklogLimit() int {
	for _, val := range []*int{c.Scrobble.BacklogLimit, c.Scrobble.BacklogLimitHyphen, c.BacklogLimit, c.BacklogLimitHyphen} {
		if val != nil && *val >= 0 {
			return *val
		}
	}
	return 5000
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
func LoadFile(path string) (*FileConfig, string, error) {
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

	if path == "" {
		return &FileConfig{}, "", nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if explicit {
			return nil, "", fmt.Errorf("failed to read config file '%s': %w", path, err)
		}
		if errors.Is(err, os.ErrNotExist) {
			return &FileConfig{}, "", nil
		}
		return nil, "", fmt.Errorf("failed to read config file '%s': %w", path, err)
	}

	var cfg FileConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, path, fmt.Errorf("failed to parse TOML config file '%s': %w", path, err)
	}

	return &cfg, path, nil
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
		lines = append(lines, fmt.Sprintf("\nsession_key = \"%s\"", sessionKey))
	}
	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0600)
}
