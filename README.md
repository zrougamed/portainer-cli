# ⚓ portainer-tui

A beautiful terminal UI for managing **Portainer Open Source** — built with [BubbleTea](https://github.com/charmbracelet/bubbletea).

```
  ____            _        _
 |  _ \ ___  _ __| |_ __ _(_)_ __   ___ _ __
 | |_) / _ \| '__| __/ _' | | '_ \ / _ \ '__|
 |  __/ (_) | |  | || (_| | | | | |  __/ |
 |_|   \___/|_|   \__\__,_|_|_| |_|\___|_|  TUI
```

## Features

- 🌐 **Environments** — browse and switch between Portainer endpoints
- 📦 **Containers** — list, start, stop, restart, view logs
- 📚 **Stacks** — list, deploy, start, stop, delete compose stacks
- 🖼  **Images** — view Docker images and sizes
- 💾 **Volumes** — view Docker volumes
- 📋 **Logs viewer** — scrollable log viewer with refresh and tail expansion
- 🔑 **Auth** — supports JWT token, username/password, and community X-API-Key

## Install

```bash
git clone https://github.com/zrougamed/portainer-cli
cd portainer-tui
make tidy
make build
./portainer-tui
```

Or install to `$GOPATH/bin`:
```bash
make install
```

## Configuration

Config lives at `~/.config/portainer-tui/config.yaml`:

```yaml
url: http://localhost:9000
token: ""        # JWT token
api_key: ""      # X-API-Key (community edition)
username: ""     # or use username/password
password: ""
```

You can also use environment variables:
```bash
export PORTAINER_URL=http://localhost:9000
export PORTAINER_TOKEN=your-jwt-token
export PORTAINER_API_KEY=your-api-key
```

Or pass flags:
```bash
portainer-tui --url http://portainer.example.com --token mytoken
```

## Usage

### Interactive TUI
```bash
portainer-tui
```

### Login (saves token)
```bash
portainer-tui login
```

### Open Portainer in browser
```bash
portainer-tui open
```

### Show config
```bash
portainer-tui config
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑↓` | Navigate list |
| `enter` | Select / view logs |
| `esc` | Go back |
| `r` | Refresh |
| `q` | Quit (from dashboard) |
| `a` | Toggle show all containers |
| `S` | Start container/stack |
| `s` | Stop container/stack |
| `R` | Restart container |
| `l` | View container logs |
| `n` | New stack (deploy) |
| `d` | Delete stack |
| `+` | Load more log lines |
| `ctrl+s` | Submit/deploy stack |
| `y/n` | Confirm dialog yes/no |

## Architecture

```
portainer-tui/
├── cmd/
│   ├── main.go       # Cobra CLI + entry point
│   └── browser.go    # Cross-platform browser opener
├── internal/
│   ├── api/
│   │   └── client.go # Portainer REST API client
│   ├── config/
│   │   └── config.go # Viper config loading
│   └── tui/
│       ├── app.go        # Root BubbleTea model + navigation
│       ├── styles.go     # Lipgloss styles
│       ├── dashboard.go  # Main menu
│       ├── endpoints.go  # Environments view
│       ├── containers.go # Containers view
│       ├── stacks.go     # Stacks view + deploy editor
│       ├── images.go     # Images view
│       ├── volumes.go    # Volumes view
│       ├── logs.go       # Log viewer (viewport)
│       └── confirm.go    # Confirmation dialog
└── config.example.yaml
```

## API Support

This tool uses the [Portainer REST API](https://app.swaggerhub.com/apis/portainer/portainer-ce).

For the community X-API-Key, create one in Portainer:  
**Settings → Users → Access Tokens → Add access token**

Then add to your config:
```yaml
api_key: ptr_xxxxxxxxxxxxxxxxxxxx
```
