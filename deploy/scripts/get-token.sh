#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# get-token.sh
# Waits for Portainer to be healthy, then fetches a JWT token and
# creates a named API access token (X-API-Key) for portainer-tui.
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

PORTAINER_URL="${PORTAINER_URL:-http://localhost:9000}"
ADMIN_USER="${PORTAINER_USER:-admin}"
ADMIN_PASS="${PORTAINER_PASS:-adminpassword}"
TOKEN_NAME="${TOKEN_NAME:-portainer-tui}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m'

log()  { echo -e "${CYAN}[portainer]${NC} $*"; }
ok()   { echo -e "${GREEN}[✓]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
fail() { echo -e "${RED}[✗]${NC} $*"; exit 1; }

# ─── Wait for Portainer to be ready ─────────────────────────────────────────
log "Waiting for Portainer at ${PORTAINER_URL} ..."
for i in $(seq 1 30); do
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${PORTAINER_URL}/api/system/status" 2>/dev/null || true)
  if [ "$STATUS" = "200" ]; then
    ok "Portainer is up!"
    break
  fi
  if [ "$i" -eq 30 ]; then
    fail "Portainer did not become ready after 30 attempts. Is it running?"
  fi
  echo -n "."
  sleep 2
done
echo

# ─── Authenticate and get JWT ────────────────────────────────────────────────
log "Authenticating as '${ADMIN_USER}' ..."
AUTH_RESPONSE=$(curl -s -X POST \
  "${PORTAINER_URL}/api/auth" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PASS}\"}")

JWT=$(echo "$AUTH_RESPONSE" | grep -o '"jwt":"[^"]*"' | cut -d'"' -f4)

if [ -z "$JWT" ]; then
  fail "Authentication failed. Response: ${AUTH_RESPONSE}"
fi
ok "JWT token obtained."

# ─── Create an API access token (X-API-Key) ──────────────────────────────────
log "Creating API access token '${TOKEN_NAME}' ..."
TOKEN_RESPONSE=$(curl -s -X POST \
  "${PORTAINER_URL}/api/users/me/tokens" \
  -H "Authorization: Bearer ${JWT}" \
  -H "Content-Type: application/json" \
  -d "{\"password\":\"${ADMIN_PASS}\",\"description\":\"${TOKEN_NAME}\"}")

API_KEY=$(echo "$TOKEN_RESPONSE" | grep -o '"rawAPIKey":"[^"]*"' | cut -d'"' -f4)

if [ -z "$API_KEY" ]; then
  warn "Could not create API key (may already exist). Using JWT instead."
  API_KEY=""
fi

# ─── Output ──────────────────────────────────────────────────────────────────
echo
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  Portainer CE — Credentials & Tokens${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  UI URL   : ${CYAN}${PORTAINER_URL}${NC}"
echo -e "  Username : ${CYAN}${ADMIN_USER}${NC}"
echo -e "  Password : ${CYAN}${ADMIN_PASS}${NC}"
echo
echo -e "  JWT Token (short-lived):"
echo -e "  ${YELLOW}${JWT}${NC}"
echo
if [ -n "$API_KEY" ]; then
  echo -e "  API Key (persistent, use with portainer-tui):"
  echo -e "  ${GREEN}${API_KEY}${NC}"
  echo
fi
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo
echo -e "  Configure portainer-tui:"
echo -e "  ${CYAN}export PORTAINER_URL=${PORTAINER_URL}${NC}"
if [ -n "$API_KEY" ]; then
  echo -e "  ${CYAN}export PORTAINER_API_KEY=${API_KEY}${NC}"
else
  echo -e "  ${CYAN}export PORTAINER_TOKEN=${JWT}${NC}"
fi
echo
echo -e "  Or run: ${CYAN}portainer-tui login${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# ─── Write to .env for portainer-tui ─────────────────────────────────────────
ENV_FILE=".portainer-tui.env"
{
  echo "PORTAINER_URL=${PORTAINER_URL}"
  echo "PORTAINER_TOKEN=${JWT}"
  if [ -n "$API_KEY" ]; then
    echo "PORTAINER_API_KEY=${API_KEY}"
  fi
} > "${ENV_FILE}"

ok "Credentials written to ${ENV_FILE}"
echo -e "  Source it with: ${CYAN}source .portainer-tui.env${NC}"