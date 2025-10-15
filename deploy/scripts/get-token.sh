#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# get-token.sh
# Waits for Portainer to be healthy, fetches a JWT token, then tries to
# create a persistent API access token (X-API-Key) via HTTPS.
#
# Note: Portainer only allows API key creation over HTTPS (port 9443).
#       JWT (port 9000) is always available as a fallback.
# ─────────────────────────────────────────────────────────────────────────────

# NO set -e here — grep returns 1 on no-match and would kill the script.
# We handle errors explicitly instead.
set -uo pipefail

PORTAINER_HTTP="${PORTAINER_URL:-http://localhost:9000}"
PORTAINER_HTTPS="${PORTAINER_HTTPS_URL:-https://localhost:9443}"
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

# Safe json field extractor — returns empty string (never fails) if field missing
json_field() {
  local json="$1" field="$2"
  echo "$json" | grep -o "\"${field}\":\"[^\"]*\"" | head -1 | cut -d'"' -f4 || true
}

# ─── Wait for Portainer HTTP to be ready ─────────────────────────────────────
log "Waiting for Portainer at ${PORTAINER_HTTP} ..."
READY=0
for i in $(seq 1 30); do
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    --connect-timeout 2 \
    "${PORTAINER_HTTP}/api/system/status" 2>/dev/null || echo "000")
  if [ "$STATUS" = "200" ]; then
    ok "Portainer is up!"
    READY=1
    break
  fi
  echo -n "."
  sleep 2
done
echo

if [ "$READY" -eq 0 ]; then
  fail "Portainer did not become ready after 30 attempts. Is it running?\n  Run: docker compose ps"
fi

# ─── Authenticate via HTTP and get JWT ───────────────────────────────────────
log "Authenticating as '${ADMIN_USER}' ..."
AUTH_RESPONSE=$(curl -s -X POST \
  "${PORTAINER_HTTP}/api/auth" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PASS}\"}" || true)

JWT=$(json_field "$AUTH_RESPONSE" "jwt")

if [ -z "$JWT" ]; then
  echo -e "${RED}[✗]${NC} Authentication failed."
  echo "    Response: ${AUTH_RESPONSE}"
  echo "    Check that the password in secrets/portainer_admin_password is correct."
  exit 1
fi
ok "JWT token obtained (valid for 8 hours)."

# ─── Create persistent API key via HTTPS ─────────────────────────────────────
# Portainer requires HTTPS for API key creation (security restriction).
# We use -k (insecure) to accept the self-signed cert it ships with.
log "Creating API access token '${TOKEN_NAME}' via HTTPS (${PORTAINER_HTTPS}) ..."

TOKEN_RESPONSE=$(curl -sk -X POST \
  "${PORTAINER_HTTPS}/api/users/me/tokens" \
  -H "Authorization: Bearer ${JWT}" \
  -H "Content-Type: application/json" \
  -d "{\"password\":\"${ADMIN_PASS}\",\"description\":\"${TOKEN_NAME}\"}" || true)

API_KEY=$(json_field "$TOKEN_RESPONSE" "rawAPIKey")

if [ -z "$API_KEY" ]; then
  warn "Could not create a persistent API key."
  # Show the actual response to help diagnose (strip JWT from display)
  REASON=$(echo "$TOKEN_RESPONSE" | grep -o '"message":"[^"]*"' | cut -d'"' -f4 || true)
  if [ -n "$REASON" ]; then
    warn "Reason: ${REASON}"
  else
    warn "Raw response: ${TOKEN_RESPONSE}"
  fi
  warn "Falling back to JWT token (expires in 8h)."
  warn "To get a persistent API key, visit ${PORTAINER_HTTPS} and go to:"
  warn "  My account → Access tokens → Add access token"
fi

# ─── Print results ────────────────────────────────────────────────────────────
echo
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  Portainer CE — Credentials & Tokens${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  HTTP UI   : ${CYAN}${PORTAINER_HTTP}${NC}"
echo -e "  HTTPS UI  : ${CYAN}${PORTAINER_HTTPS}${NC}"
echo -e "  Username  : ${CYAN}${ADMIN_USER}${NC}"
echo -e "  Password  : ${CYAN}${ADMIN_PASS}${NC}"
echo
echo -e "  JWT Token ${YELLOW}(expires in 8h)${NC}:"
echo -e "  ${YELLOW}${JWT}${NC}"
echo
if [ -n "$API_KEY" ]; then
  ok "API Key created (persistent):"
  echo -e "  ${GREEN}${API_KEY}${NC}"
  echo
fi
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo
echo -e "  To use with portainer-tui:"
if [ -n "$API_KEY" ]; then
  echo -e "  ${CYAN}export PORTAINER_URL=${PORTAINER_HTTP}${NC}"
  echo -e "  ${CYAN}export PORTAINER_API_KEY=${API_KEY}${NC}"
else
  echo -e "  ${CYAN}export PORTAINER_URL=${PORTAINER_HTTP}${NC}"
  echo -e "  ${CYAN}export PORTAINER_TOKEN=${JWT}${NC}"
fi
echo -e "  ${CYAN}portainer-tui${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# ─── Write .portainer-tui.env ────────────────────────────────────────────────
ENV_FILE=".portainer-tui.env"
{
  echo "export PORTAINER_URL=${PORTAINER_HTTP}"
  echo "export PORTAINER_TOKEN=${JWT}"
  if [ -n "$API_KEY" ]; then
    echo "export PORTAINER_API_KEY=${API_KEY}"
  fi
} > "${ENV_FILE}"

echo
ok "Saved to ${ENV_FILE}"
echo -e "  ${CYAN}source ${ENV_FILE} && portainer-tui${NC}"
