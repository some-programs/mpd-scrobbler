package lastfm

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/shkh/lastfm-go/lastfm"
	"github.com/some-programs/mpd-scrobbler/pkg/config"
)

// InteractiveAuth runs a terminal-based token flow to generate and print a Last.fm session key.
// If configPath is provided and the file exists, it automatically updates session_key in that file.
func InteractiveAuth(apiKey, apiSecret string, configPath string) error {
	if apiKey == "" || apiSecret == "" {
		return errors.New("lastfm.api-key and lastfm.api-secret are required for authentication (set in config file, environment, or flags)")
	}

	api := lastfm.New(apiKey, apiSecret)
	token, err := api.GetToken()
	if err != nil {
		return fmt.Errorf("failed to retrieve request token from Last.fm: %w", err)
	}

	authURL := api.GetAuthTokenUrl(token)
	fmt.Println("\n==================================================================")
	fmt.Println("             LAST.FM INTERACTIVE AUTHORIZATION")
	fmt.Println("==================================================================")
	fmt.Printf("1. Please open the following URL in your web browser:\n\n   %s\n\n", authURL)
	fmt.Println("2. Click 'Yes, allow access' on the Last.fm authorization page.")
	fmt.Print("3. Once approved, press [ENTER] here in the terminal to finish: ")

	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')

	if err := api.LoginWithToken(token); err != nil {
		return fmt.Errorf("failed to obtain session from Last.fm token: %w", err)
	}

	sessionKey := api.GetSessionKey()
	fmt.Println("\n==================================================================")
	fmt.Println("                      SUCCESS!")
	fmt.Println("==================================================================")
	fmt.Printf("Your Last.fm Session Key is:\n\n   %s\n\n", sessionKey)

	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			if err := config.SaveSessionKey(configPath, sessionKey); err == nil {
				fmt.Printf("✓ Automatically saved session_key to:\n   %s\n\n", configPath)
			}
		}
	}

	fmt.Println("You can run mpd-scrobbler directly, or as a service:")
	fmt.Println("   systemctl --user restart mpd-scrobbler.service")
	fmt.Println("==================================================================")

	return nil
}

// SanitizeString cleans up token/credential input strings.
func SanitizeString(s string) string {
	return strings.TrimSpace(s)
}
