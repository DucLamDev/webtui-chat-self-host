#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
ENV_FILE="$SCRIPT_DIR/.env"
OFFSITE_ENV_FILE="$SCRIPT_DIR/offsite-backup.env"

read_file_env_value() {
  file=$1
  key=$2
  if [ ! -f "$file" ]; then
    return 0
  fi
  sed -n "s/^$key=//p" "$file" | tail -n 1 | tr -d '\r'
}

read_env_value() {
  read_file_env_value "$ENV_FILE" "$1"
}

write_env_value() {
  key=$1
  value=$2
  escaped=$(printf '%s' "$value" | sed 's/[&|]/\\&/g')
  if grep -q "^$key=" "$ENV_FILE"; then
    sed -i "s|^$key=.*|$key=$escaped|" "$ENV_FILE"
  else
    printf '%s=%s\n' "$key" "$value" >> "$ENV_FILE"
  fi
}

ensure_secret() {
  key=$1
  current=$(read_env_value "$key")
  case "$current" in
    ""|CHANGE_ME*) write_env_value "$key" "$(openssl rand -hex 32)" ;;
  esac
}

cd "$REPO_DIR"
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
  echo "Tracked files have local changes. Commit or revert them before updating." >&2
  exit 1
fi
if [ ! -f "$ENV_FILE" ]; then
  echo "Missing $ENV_FILE. Run install.sh first." >&2
  exit 1
fi

BACKUP_ENABLED=$(read_file_env_value "$OFFSITE_ENV_FILE" OFFSITE_BACKUP_ENABLED)
BACKUP_ENABLED=$(printf '%s' "$BACKUP_ENABLED" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | tr '[:upper:]' '[:lower:]')
case "$BACKUP_ENABLED" in
  1|true|yes|on)
    echo "Creating the configured off-site backup before updating..."
    sh "$SCRIPT_DIR/backup.sh" backup --maintenance
    ;;
  ""|0|false|no|off)
    echo "Off-site backup is not enabled; continuing without an automatic backup."
    ;;
  *)
    echo "OFFSITE_BACKUP_ENABLED in $OFFSITE_ENV_FILE must be true or false." >&2
    exit 1
    ;;
esac

git pull --ff-only

cd "$SCRIPT_DIR"
if ! command -v openssl >/dev/null 2>&1; then
  echo "OpenSSL is required to create missing self-hosted service credentials." >&2
  exit 1
fi

# Upgrade older installations in place. Existing custom Jitsi URLs and strong
# credentials are preserved; missing values are generated automatically.
INSTANCE_DOMAIN=$(read_env_value INSTANCE_DOMAIN)
if [ -z "$INSTANCE_DOMAIN" ]; then
  echo "INSTANCE_DOMAIN is missing from $ENV_FILE." >&2
  exit 1
fi
OFFICIAL_APP_LINK_HOST=chat.vpsttt.com
CURRENT_APP_LINK_HOST=$(read_env_value WEBTUI_APP_LINK_HOST)
PRESERVE_CUSTOM_APP_LINK_HOST=$(read_env_value PRESERVE_CUSTOM_APP_LINK_HOST)
PRESERVE_CUSTOM_APP_LINK_HOST=$(printf '%s' "$PRESERVE_CUSTOM_APP_LINK_HOST" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | tr '[:upper:]' '[:lower:]')
case "$PRESERVE_CUSTOM_APP_LINK_HOST" in
  1|true|yes|on)
    if [ -z "$CURRENT_APP_LINK_HOST" ]; then
      echo "PRESERVE_CUSTOM_APP_LINK_HOST=true requires WEBTUI_APP_LINK_HOST." >&2
      exit 1
    fi
    echo "Preserving custom-branded WEBTUI_APP_LINK_HOST=$CURRENT_APP_LINK_HOST."
    ;;
  ""|0|false|no|off)
    case "$CURRENT_APP_LINK_HOST" in
      ""|"$INSTANCE_DOMAIN")
        if [ "$CURRENT_APP_LINK_HOST" != "$OFFICIAL_APP_LINK_HOST" ]; then
          echo "Migrating WEBTUI_APP_LINK_HOST to publisher host $OFFICIAL_APP_LINK_HOST for the official universal app."
          write_env_value WEBTUI_APP_LINK_HOST "$OFFICIAL_APP_LINK_HOST"
        fi
        ;;
      *)
        echo "Preserving custom WEBTUI_APP_LINK_HOST=$CURRENT_APP_LINK_HOST (custom-branded app override)."
        ;;
    esac
    ;;
  *)
    echo "PRESERVE_CUSTOM_APP_LINK_HOST in $ENV_FILE must be true or false." >&2
    exit 1
    ;;
esac
if [ -z "$(read_env_value MOBILE_MIN_VERSION)" ]; then
  # Compatibility contract added after the first self-host releases. Backfill
  # it before recreating API containers while preserving operator overrides.
  write_env_value MOBILE_MIN_VERSION "1.0.0"
fi
PORTAL_ORIGIN=$(read_env_value PORTAL_ORIGIN)
if [ -z "$PORTAL_ORIGIN" ]; then
  PORTAL_ORIGIN="https://download.vpsttt.com"
  write_env_value PORTAL_ORIGIN "$PORTAL_ORIGIN"
fi
TERMS_VERSION=$(read_env_value TERMS_VERSION)
if [ -z "$TERMS_VERSION" ]; then
  TERMS_VERSION="2026-08-07"
  write_env_value TERMS_VERSION "$TERMS_VERSION"
fi
PRIVACY_POLICY_VERSION=$(read_env_value PRIVACY_POLICY_VERSION)
if [ -z "$PRIVACY_POLICY_VERSION" ]; then
  PRIVACY_POLICY_VERSION="2026-08-07"
  write_env_value PRIVACY_POLICY_VERSION "$PRIVACY_POLICY_VERSION"
fi
if [ -z "$(read_env_value NEXT_PUBLIC_TERMS_URL)" ]; then
  write_env_value NEXT_PUBLIC_TERMS_URL "$PORTAL_ORIGIN/terms"
fi
if [ -z "$(read_env_value NEXT_PUBLIC_PRIVACY_URL)" ]; then
  write_env_value NEXT_PUBLIC_PRIVACY_URL "$PORTAL_ORIGIN/privacy"
fi
if [ -z "$(read_env_value NEXT_PUBLIC_TERMS_VERSION)" ]; then
  write_env_value NEXT_PUBLIC_TERMS_VERSION "$TERMS_VERSION"
fi
if [ -z "$(read_env_value NEXT_PUBLIC_PRIVACY_VERSION)" ]; then
  write_env_value NEXT_PUBLIC_PRIVACY_VERSION "$PRIVACY_POLICY_VERSION"
fi
if [ -z "$(read_env_value ENABLE_IOS_ASSOCIATION)" ]; then
  # Older installs probed AASA unconditionally. The official first release is
  # Play-only, so preserve a fail-closed 404 until iOS is explicitly provisioned.
  write_env_value ENABLE_IOS_ASSOCIATION "false"
fi
PUSH_RELAY_ENABLED=$(read_env_value PUSH_RELAY_SERVER_ENABLED)
PUSH_RELAY_ENABLED=$(printf '%s' "$PUSH_RELAY_ENABLED" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | tr '[:upper:]' '[:lower:]')
case "$PUSH_RELAY_ENABLED" in
  1|true|yes|on) PUSH_RELAY_ENABLED=true ;;
  ""|0|false|no|off) PUSH_RELAY_ENABLED=false ;;
  *)
    echo "PUSH_RELAY_SERVER_ENABLED in $ENV_FILE must be true or false." >&2
    exit 1
    ;;
esac
PUSH_RELAY_PORT=$(read_env_value PUSH_RELAY_HTTP_PORT)
if [ -z "$PUSH_RELAY_PORT" ]; then
  PUSH_RELAY_PORT=8090
fi
if [ "$PUSH_RELAY_PORT" != "8090" ]; then
  echo "Bundled self-host Caddy/Prometheus require PUSH_RELAY_HTTP_PORT=8090; use a dedicated relay deployment for another port." >&2
  exit 1
fi
MEETING_URL=$(read_env_value JITSI_BASE_URL)
if [ -z "$MEETING_URL" ]; then
  MEETING_URL=$(read_env_value NEXT_PUBLIC_JITSI_BASE_URL)
fi
if [ -z "$MEETING_URL" ]; then
  MEETING_URL="https://$INSTANCE_DOMAIN:8443"
fi
if [ -z "$(read_env_value JITSI_BASE_URL)" ]; then
  write_env_value JITSI_BASE_URL "$MEETING_URL"
fi
if [ -z "$(read_env_value NEXT_PUBLIC_JITSI_BASE_URL)" ]; then
  write_env_value NEXT_PUBLIC_JITSI_BASE_URL "$MEETING_URL"
fi
ensure_secret JICOFO_AUTH_PASSWORD
ensure_secret JVB_AUTH_PASSWORD
# Older installations may predate this key or still contain the documented
# placeholder. Generate it before migrations so a hidden, unused Bot module
# cannot prevent the rest of the application from being updated.
ensure_secret BOT_AI_SECRET_KEY
if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q '^Status: active'; then
  if [ "$(id -u)" -eq 0 ]; then
    ufw allow 8443/tcp >/dev/null
    ufw allow 10000/udp >/dev/null
  else
    echo "Warning: allow TCP 8443 and UDP 10000 in the VPS firewall for group video." >&2
  fi
fi

# The customer VPS may only contain this self-host repository. Build the
# deployable services explicitly so an optional sibling portal source does not
# block updates to the chat application.
docker compose --env-file .env -f compose.yml pull caddy jitsi-prosody jitsi-jicofo jitsi-jvb jitsi-web
docker compose --env-file .env -f compose.yml build --pull api worker web admin
if [ "$PUSH_RELAY_ENABLED" = "true" ]; then
  docker compose --env-file .env -f compose.yml --profile push-relay build --pull push-relay
fi
if ! docker compose --env-file .env -f compose.yml run --rm migrate; then
  echo "" >&2
  echo "Update stopped before replacing containers because database migration failed." >&2
  echo "No newly built web/API image has been activated." >&2
  echo "If the error mentions BOT_AI_SECRET_KEY, set a strong key in deploy/self-hosted/.env, then run this update again." >&2
  echo "For an instance that has never stored Bot AI credentials, generate one with:" >&2
  echo "  BOT_KEY=\$(openssl rand -hex 48)" >&2
  echo "  sed -i \"s|^BOT_AI_SECRET_KEY=.*|BOT_AI_SECRET_KEY=\$BOT_KEY|\" deploy/self-hosted/.env" >&2
  echo "  unset BOT_KEY" >&2
  exit 1
fi
docker compose --env-file .env -f compose.yml up -d --no-deps --force-recreate api worker web admin
docker compose --env-file .env -f compose.yml up -d --no-deps jitsi-prosody
docker compose --env-file .env -f compose.yml up -d --no-deps jitsi-jicofo jitsi-jvb
docker compose --env-file .env -f compose.yml up -d --no-deps jitsi-web
docker compose --env-file .env -f compose.yml up -d --no-deps --force-recreate caddy
if [ "$PUSH_RELAY_ENABLED" = "true" ]; then
  docker compose --env-file .env -f compose.yml --profile push-relay up -d --no-deps --force-recreate push-relay
  relay_attempt=0
  until docker compose --env-file .env -f compose.yml --profile push-relay exec -T push-relay \
    wget -qO- "http://localhost:$PUSH_RELAY_PORT/ready" >/dev/null 2>&1; do
    relay_attempt=$((relay_attempt + 1))
    if [ "$relay_attempt" -ge 30 ]; then
      echo "Updated push-relay did not become ready within 60 seconds." >&2
      docker compose --env-file .env -f compose.yml --profile push-relay logs --tail=100 push-relay >&2
      exit 1
    fi
    sleep 2
  done
  echo "Updated push-relay: ready"
fi
sh "$SCRIPT_DIR/check.sh"
