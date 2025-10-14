# ⚓ Portainer CE — Docker Compose

Spin up Portainer Community Edition with a pre-configured admin password and API token generation script, ready to connect with **portainer-tui**.

## Quick Start

```bash
# 1. Start Portainer
make up

# 2. Grab your JWT + API token
make token

# 3. Source the generated env file
source .portainer-tui.env

# 4. Launch the TUI (from portainer-tui directory)
portainer-tui
```

## Credentials

| Field    | Value            |
|----------|------------------|
| URL      | http://localhost:9000 |
| HTTPS    | https://localhost:9443 |
| Username | `admin`          |
| Password | `adminpassword`  |

> **Change the password** before exposing this to any network.  
> Edit `secrets/portainer_admin_password` before first `make up`.

## Files

```
portainer-ce-compose/
├── docker-compose.yml                  # Main compose file
├── secrets/
│   └── portainer_admin_password        # Plaintext password (Portainer hashes it)
├── scripts/
│   └── get-token.sh                    # Fetches JWT + API key, writes .portainer-tui.env
└── Makefile                            # Convenience targets
```

## How the password works

Portainer reads the file at `/run/secrets/portainer_admin_password` (mounted via Docker secrets) and bcrypt-hashes it internally on first boot. This avoids the double-`$$` escaping problem with inline bcrypt hashes in `docker-compose.yml`.

```
secrets/portainer_admin_password  ←  just put your plaintext password here
```

## Ports

| Port | Purpose |
|------|---------|
| `9000` | HTTP — Portainer UI + REST API |
| `9443` | HTTPS — Portainer UI + REST API |
| `8000` | Edge Agent tunnel server |

## Connecting portainer-tui

After `make token`, the script writes a `.portainer-tui.env` file:

```bash
source .portainer-tui.env
portainer-tui               # interactive TUI

# Or use the login command to save permanently:
portainer-tui login
# URL: http://localhost:9000
# Username: admin
# Password: adminpassword
```

Or set config manually at `~/.config/portainer-tui/config.yaml`:

```yaml
url: http://localhost:9000
api_key: ptr_xxxxxxxxxxxxxxxxxxxx   # from make token
```

## API Token vs JWT

| Type | Flag | Lifetime | Use case |
|------|------|----------|----------|
| JWT | `PORTAINER_TOKEN` | ~8 hours | Short-lived sessions |
| API Key | `PORTAINER_API_KEY` | Permanent | portainer-tui config |

The `get-token.sh` script fetches both and writes them to `.portainer-tui.env`.

## Make targets

```bash
make up       # Start Portainer
make down     # Stop
make restart  # Restart
make logs     # Tail logs
make token    # Get JWT + API key
make open     # Open in browser
make ps       # Show status
make clean    # Remove containers + volumes (destructive!)
```

## Changing the password

Edit the secret file and recreate the container:

```bash
echo -n "mynewpassword" > secrets/portainer_admin_password
make down
make up
```

Note: the password can only be set on **first boot** (fresh data volume). To change an existing password, use the Portainer reset helper or the UI.