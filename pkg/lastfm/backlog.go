package lastfm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/some-programs/mpd-scrobbler/pkg/tracker"
)

// BacklogItem represents an unsubmitted scrobble persisted to disk.
type BacklogItem struct {
	Track     tracker.Track `json:"track"`
	Timestamp time.Time     `json:"timestamp"`
}

// DefaultBacklogPath returns the default path for persistent append-only backlog storage.
func DefaultBacklogPath() string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			stateDir = filepath.Join(homeDir, ".local", "state")
		}
	}
	if stateDir != "" {
		return filepath.Join(stateDir, "mpd-scrobbler", "backlog.jsonl")
	}

	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configDir = filepath.Join(homeDir, ".config")
		}
	}
	if configDir != "" {
		return filepath.Join(configDir, "mpd-scrobbler", "backlog.jsonl")
	}

	return "backlog.jsonl"
}

// AppendBacklog appends a single scrobble entry as a JSON line to the backlog file.
// This is an O(1) append operation and does not rewrite existing entries.
func AppendBacklog(path string, item BacklogItem) error {
	if path == "" {
		return nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create backlog directory '%s': %w", dir, err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open backlog file '%s' for append: %w", path, err)
	}
	defer f.Close()

	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal backlog item: %w", err)
	}

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write to backlog file '%s': %w", path, err)
	}

	return nil
}

// LoadBacklog loads pending scrobbles from an append-only JSONL file (or legacy JSON array).
// Entries older than 14 days are automatically discarded (Last.fm API cutoff).
func LoadBacklog(path string) ([]BacklogItem, error) {
	if path == "" {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Check legacy .json fallback if path ends with .jsonl
			if filepath.Ext(path) == ".jsonl" {
				legacyPath := path[:len(path)-1] // .json
				return loadLegacyJSONBacklog(legacyPath)
			}
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open backlog file '%s': %w", path, err)
	}
	defer f.Close()

	var items []BacklogItem
	cutoff := time.Now().Add(-14 * 24 * time.Hour)

	scanner := bufio.NewScanner(f)
	// Support larger line buffer if track metadata contains large strings
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		// If first character is '[', this is a legacy JSON array
		if line[0] == '[' {
			return parseJSONArray(line, cutoff)
		}

		var item BacklogItem
		if err := json.Unmarshal(line, &item); err == nil {
			if item.Timestamp.After(cutoff) {
				items = append(items, item)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return items, fmt.Errorf("error reading backlog file '%s': %w", path, err)
	}

	return items, nil
}

func loadLegacyJSONBacklog(path string) ([]BacklogItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	cutoff := time.Now().Add(-14 * 24 * time.Hour)
	return parseJSONArray(data, cutoff)
}

func parseJSONArray(data []byte, cutoff time.Time) ([]BacklogItem, error) {
	var raw []BacklogItem
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	var valid []BacklogItem
	for _, item := range raw {
		if item.Timestamp.After(cutoff) {
			valid = append(valid, item)
		}
	}
	return valid, nil
}

// RewriteBacklog updates the backlog file after successfully submitting tracks.
// If items is empty, the file is truncated to 0 bytes.
func RewriteBacklog(path string, items []BacklogItem) error {
	if path == "" {
		return nil
	}

	if len(items) == 0 {
		_ = os.Truncate(path, 0)
		return nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create backlog directory '%s': %w", dir, err)
	}

	tmpFile := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temp backlog file '%s': %w", tmpFile, err)
	}

	writer := bufio.NewWriter(f)
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			continue
		}
		data = append(data, '\n')
		if _, err := writer.Write(data); err != nil {
			_ = f.Close()
			_ = os.Remove(tmpFile)
			return fmt.Errorf("failed to write to temp backlog: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return err
	}

	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to rename temp backlog file: %w", err)
	}

	return nil
}
