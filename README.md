```
  ____            _        _
 |  _ \ ___  _ __| |_ __ _(_)_ __   ___ _ __
 | |_) / _ \| '__| __/ _' | | '_ \ / _ \ '__|
 |  __/ (_) | |  | || (_| | | | | |  __/ |
 |_|   \___/|_|   \__\__,_|_|_| |_|\___|_|  TUI by zrougamed
```

# portainer-tui

Operational control for Portainer — directly from your terminal — built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)..

portainer-tui provides a fast, keyboard-driven interface for managing Docker environments through Portainer Open Source.  
Containers, stacks, images, volumes, and networks — accessible without a browser.

Designed for engineers managing real infrastructure in real environments.

![video](resources/portainer-tui.gif)

---

## Features

### Environments

Browse all Portainer endpoints and switch between them. The active environment is shown on every screen, and any view that requires an endpoint (containers, images, volumes, networks) will prompt you to select one first.

### Containers

List all containers on the active endpoint with their ID, name, image, state, and status. Filter between running-only and all containers with a single key. Supported operations:

- Start, stop, and restart containers
- Pause and unpause containers
- Recreate a container (pulls latest image, removes the old container, and starts a fresh one)
- Delete a container (with optional force-delete to remove even running containers)
- View real-time logs in a scrollable viewport with tail expansion

### Stacks

List all Compose stacks across all endpoints with their type (standalone/swarm), status (active/inactive), and associated endpoint. Supported operations:

- Deploy a new stack by pasting a `docker-compose.yml` directly into the built-in editor
- Start and stop existing stacks
- Delete stacks
- Drill into a stack to see its individual containers and their state

### Images

List all Docker images on the active endpoint with repository/tag, size, and container count. Supported operations:

- Pull a new image by reference (e.g. `nginx:latest`, `ubuntu:22.04`)
- Delete an image with confirmation
- Force-delete an image even if containers are using it
- Prune all unused (dangling) images with a space-reclaimed report

### Volumes

List all Docker volumes on the active endpoint with their driver, scope, and mountpoint. Supported operations:

- Create a new volume with a custom name and driver
- Delete a volume with confirmation
- Force-delete a volume even if in use
- Prune all unused volumes with a space-reclaimed report

### Networks

List all Docker networks on the active endpoint with their driver, scope, and subnet configuration. Supported operations:

- Create a new network with a custom name, driver (bridge, overlay, etc.), internal flag, and attachable flag
- Delete a network

### Log Viewer

A dedicated full-screen log viewer for any container. Logs are fetched from the Portainer API with configurable tail size, displayed in a scrollable viewport, and can be refreshed on demand. The tail window expands with the `+` key to load more history.

### Authentication

Supports three authentication methods, tried in priority order:

- **X-API-Key** — Portainer Community Edition permanent API key. Recommended for long-lived usage.
- **JWT token** — Short-lived token from the Portainer API or login command. Automatically used if set.
- **Username/password** — Credentials are used to fetch a JWT token on startup. Useful for scripted workflows.

When a session expires mid-use, the application automatically redirects to the login screen so you can re-authenticate without restarting.

### Error Handling

API errors are displayed in a scrollable modal overlay rather than crashing the application. Error messages can be copied to the clipboard for easy sharing or debugging.

### Session Management

The application maintains your active endpoint selection across view switches. Authentication failures are caught globally and redirect to the login screen, preserving the rest of the application state.

---

## Install

```bash
git clone https://github.com/zrougamed/portainer-cli
cd portainer-tui
make tidy
make build
./portainer-tui
```

To install into `$GOPATH/bin`:

```bash
make install
```

---

## Configuration

Config is loaded from `~/.config/portainer-tui/config.yaml`:

```yaml
url: http://localhost:9000

# Use ONE of the following authentication options:

# Option 1: Permanent API key (recommended)
api_key: ""

# Option 2: Short-lived JWT token
token: ""

# Option 3: Username and password
username: ""
password: ""
```

Environment variables are also supported and override the config file:

```bash
export PORTAINER_URL=http://localhost:9000
export PORTAINER_TOKEN=your-jwt-token
export PORTAINER_API_KEY=your-api-key
```

CLI flags override everything:

```bash
portainer-tui --url http://portainer.example.com --api-key ptr_xxxx
```

---

## Usage

Launch the interactive TUI:

```bash
portainer-tui
```

Authenticate with Portainer and save the token to config:

```bash
portainer-tui login
```

Open the Portainer web UI in your default browser:

```bash
portainer-tui open
```

Show the current configuration (token is truncated for safety):

```bash
portainer-tui config
```

Print the version:

```bash
portainer-tui version
```

---

## Keyboard Shortcuts

### Global

| Key | Action |
|-----|--------|
| `↑ / ↓` | Navigate list |
| `esc` | Go back to previous view |
| `r` | Refresh current view |
| `q` | Quit (from dashboard) |

### Containers

| Key | Action |
|-----|--------|
| `l` or `enter` | View container logs |
| `S` | Start container |
| `s` | Stop container |
| `R` | Restart container |
| `p` | Pause / unpause container |
| `e` | Recreate container (pull + restart) |
| `D` | Delete container |
| `ctrl+d` | Force delete container |
| `a` | Toggle show all containers |

### Stacks

| Key | Action |
|-----|--------|
| `n` | New stack (open compose editor) |
| `S` | Start stack |
| `s` | Stop stack |
| `d` | Delete stack |
| `c` | View containers belonging to stack |
| `ctrl+s` | Submit / deploy stack (in editor) |

### Images

| Key | Action |
|-----|--------|
| `p` | Pull image |
| `d` | Delete image |
| `D` | Force delete image |
| `P` | Prune unused images |

### Volumes

| Key | Action |
|-----|--------|
| `n` | Create new volume |
| `d` | Delete volume |
| `D` | Force delete volume |
| `P` | Prune unused volumes |

### Logs

| Key | Action |
|-----|--------|
| `+` | Load more log lines |
| `r` | Refresh logs |
| `↑ / ↓` | Scroll |

### Dialogs

| Key | Action |
|-----|--------|
| `y` | Confirm action |
| `n` | Cancel action |
| `ctrl+c` | Copy error to clipboard (error modal) |

---

## Development Environment

A Docker Compose setup for Portainer CE is included under `deploy/` to get a local instance running quickly:

```bash
cd deploy

# Start Portainer CE
make up

# Fetch JWT and API key, write to .portainer-tui.env
make token

# Source the env file and launch the TUI
source .portainer-tui.env
portainer-tui
```

Default credentials are `admin` / `adminpassword` at `http://localhost:9000`. Change the password in `deploy/secrets/portainer_admin_password` before first boot.

---

## API

This project uses the [Portainer CE REST API](https://app.swaggerhub.com/apis/portainer/portainer-ce).

One important constraint: Portainer rejects requests that contain both a `Bearer` token and an `X-API-Key` header simultaneously. The client ensures only one authentication header is sent per request based on which credential is configured.

---

## Testing

The project includes unit tests across all modules, using `httptest` mock servers to test the API client without a live Portainer instance:

```bash
make test
```


