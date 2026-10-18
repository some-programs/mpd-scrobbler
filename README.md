# mpd-scrobbler

A lightweight, reliable Go daemon that submits tracks played on Music Player Daemon (MPD) to Last.fm / Audioscrobbler.

## Linux Quick Install (User Systemd Service)

To build, install the binary to `~/.local/bin`, set up `~/.config/mpd-scrobbler/config.toml`, and enable the user-level systemd service:

```bash
./install.sh
```

To remove the service and binary:

```bash
./uninstall.sh
```

---

## Configuration

Configuration values are evaluated with the following precedence:
1. **Command-line flags** (highest priority)
2. **Environment variables** (via `envflag`)
3. **TOML configuration file** (`~/.config/mpd-scrobbler/config.toml` or path passed to `-config` / `CONFIG`)
4. **Built-in defaults** (lowest priority)

### 1. TOML Configuration File (`config.toml`)

Copy `config.example.toml` to `~/.config/mpd-scrobbler/config.toml`:

```toml
[mpd]
network = "tcp"
address = "127.0.0.1:6600"
password = ""
reconnect_interval = "5s"

[lastfm]
api_key = "YOUR_API_KEY"
api_secret = "YOUR_API_SECRET"
session_key = "YOUR_SESSION_KEY"
# Alternatively for username/password:
# username = "user"
# password = "password"

[scrobble]
min_length = "30s"
min_percent = 50
max_duration = "4m"
now_playing = true

[general]
dry_run = false
verbose = false
```

### 2. Environment Variables & Command-Line Flags

| Flag                      | Environment Variable     | TOML Key                 | Default                               | Description                                      |
|---------------------------|--------------------------|--------------------------|---------------------------------------|--------------------------------------------------|
| `-config`                 | `CONFIG`                 | -                        | `~/.config/mpd-scrobbler/config.toml` | Path to TOML config file                         |
| `-mpd.network`            | `MPD_NETWORK`            | `mpd.network`            | `tcp`                                 | MPD network type (`tcp` or `unix`)               |
| `-mpd.address`            | `MPD_ADDRESS`            | `mpd.address`            | `127.0.0.1:6600`                      | MPD host:port or unix socket path                |
| `-mpd.password`           | `MPD_PASSWORD`           | `mpd.password`           | `""`                                  | MPD password (optional)                          |
| `-mpd.reconnect-interval` | `MPD_RECONNECT_INTERVAL` | `mpd.reconnect_interval` | `5s`                                  | Delay between MPD reconnect attempts             |
| `-lastfm.api-key`         | `LASTFM_API_KEY`         | `lastfm.api_key`         | `""`                                  | Last.fm API Key                                  |
| `-lastfm.api-secret`      | `LASTFM_API_SECRET`      | `lastfm.api_secret`      | `""`                                  | Last.fm API Secret                               |
| `-lastfm.session-key`     | `LASTFM_SESSION_KEY`     | `lastfm.session_key`     | `""`                                  | Last.fm Session Key                              |
| `-lastfm.username`        | `LASTFM_USERNAME`        | `lastfm.username`        | `""`                                  | Last.fm Username (for mobile auth)               |
| `-lastfm.password`        | `LASTFM_PASSWORD`        | `lastfm.password`        | `""`                                  | Last.fm Password (for mobile auth)               |
| `-lastfm.auth`            | `LASTFM_AUTH`            | -                        | `false`                               | Run interactive terminal auth to get session key |
| `-scrobble.min-length`    | `SCROBBLE_MIN_LENGTH`    | `scrobble.min_length`    | `30s`                                 | Minimum song duration to scrobble                |
| `-scrobble.min-percent`   | `SCROBBLE_MIN_PERCENT`   | `scrobble.min_percent`   | `50`                                  | Required percentage of track played (1-100)      |
| `-scrobble.max-duration`  | `SCROBBLE_MAX_DURATION`  | `scrobble.max_duration`  | `4m0s`                                | Maximum listen duration before scrobbling        |
| `-scrobble.now-playing`   | `SCROBBLE_NOW_PLAYING`   | `scrobble.now_playing`   | `true`                                | Send 'Now Playing' updates                       |
| `-dry-run`                | `DRY_RUN`                | `general.dry_run`        | `false`                               | Test scrobbling logic without contacting Last.fm |
| `-verbose`                | `VERBOSE`                | `general.verbose`        | `false`                               | Enable verbose logging                           |

---

## Last.fm Setup & Authentication

### 1. How to Get Your Last.fm API Key & Secret

1. Log in to your Last.fm account and navigate to the API account creation page:  
   **[https://www.last.fm/api/account/create](https://www.last.fm/api/account/create)**

2. Fill out the application form:
   - **Contact email**: Your email address.
   - **Application name**: E.g. `mpd-scrobbler` (or any name you prefer).
   - **Application description**: A short note, e.g. `MPD scrobbler daemon`.
   - **Callback URL**: **Leave this blank / empty!**  
     > **Why leave Callback URL blank?**  
     > As a daemon/command-line tool, `mpd-scrobbler` uses desktop/out-of-band authorization rather than running a web redirect server. When the callback URL is left blank, Last.fm simply displays a confirmation screen (*"Application authorized"*) in your browser after you click "Allow", and the daemon retrieves the session key directly once you confirm in the terminal.
   - **Application homepage**: Can be left empty or set to `https://localhost` or your repository URL.

3. Click **Submit**.

4. Last.fm will generate and display:
   - **API Key** (e.g. `0123456789abcdef0123456789abcdef`)
   - **Shared Secret** (e.g. `fedcba9876543210fedcba9876543210`)

---

### 2. Generating Your Session Key (Recommended)

Once you have your API Key and Secret, run `mpd-scrobbler` with the `-lastfm.auth` flag:

```bash
mpd-scrobbler \
  -lastfm.api-key="YOUR_API_KEY" \
  -lastfm.api-secret="YOUR_API_SECRET" \
  -lastfm.auth
```

Follow the prompt:
1. Open the printed authorization link in your web browser.
2. Click **"Yes, allow access"** on Last.fm.
3. Switch back to your terminal and press **[ENTER]**.
4. The daemon will print your **Session Key** (e.g. `abc123...`).

Add this session key to your `~/.config/mpd-scrobbler/config.toml`:

```toml
[lastfm]
api_key = "YOUR_API_KEY"
api_secret = "YOUR_API_SECRET"
session_key = "YOUR_SESSION_KEY"
```

With the session key saved, the daemon stays authenticated permanently without needing your username or password.

## Systemd Service Management

Once installed via `./install.sh`:

- **Check status**:
  ```bash
  systemctl --user status mpd-scrobbler.service
  ```
- **View live logs**:
  ```bash
  journalctl --user -u mpd-scrobbler -f
  ```
- **Restart service**:
  ```bash
  systemctl --user restart mpd-scrobbler.service
  ```

---

## Testing

Run the test suite:

```bash
go test -v ./...
```
