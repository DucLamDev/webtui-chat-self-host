#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"
ENV_FILE="$SCRIPT_DIR/.env"
COMPOSE_FILE="$SCRIPT_DIR/compose.yml"

if [ ! -f "$ENV_FILE" ]; then
  echo "Missing $ENV_FILE. Run install.sh first." >&2
  exit 1
fi

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" config >/dev/null
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps
if ! READY_BODY=$(docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T api wget -qO- http://localhost:8080/ready 2>&1); then
  echo "API /ready failed:" >&2
  printf '%s\n' "$READY_BODY" >&2
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" logs --tail=200 api postgres redis rabbitmq >&2 || true
  exit 1
fi
printf '%s\n' "$READY_BODY"
echo

RUNNING_SERVICES=$(docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps --status running --services)
for service in jitsi-prosody jitsi-jicofo jitsi-jvb jitsi-web; do
  if ! printf '%s\n' "$RUNNING_SERVICES" | grep -qx "$service"; then
    echo "Meeting service is not running: $service" >&2
    exit 1
  fi
done
echo "Meeting containers: running"

INSTANCE_DOMAIN=$(sed -n 's/^INSTANCE_DOMAIN=//p' "$ENV_FILE" | tail -n 1)
PORTAL_DOMAIN=$(sed -n 's/^PORTAL_DOMAIN=//p' "$ENV_FILE" | tail -n 1)
PORTAL_ORIGIN=$(sed -n 's/^PORTAL_ORIGIN=//p' "$ENV_FILE" | tail -n 1)
WEBTUI_APP_LINK_HOST=$(sed -n 's/^WEBTUI_APP_LINK_HOST=//p' "$ENV_FILE" | tail -n 1)
ENABLE_IOS_ASSOCIATION=$(sed -n 's/^ENABLE_IOS_ASSOCIATION=//p' "$ENV_FILE" | tail -n 1)
TERMS_VERSION=$(sed -n 's/^TERMS_VERSION=//p' "$ENV_FILE" | tail -n 1)
PRIVACY_POLICY_VERSION=$(sed -n 's/^PRIVACY_POLICY_VERSION=//p' "$ENV_FILE" | tail -n 1)
NEXT_PUBLIC_TERMS_URL=$(sed -n 's/^NEXT_PUBLIC_TERMS_URL=//p' "$ENV_FILE" | tail -n 1)
NEXT_PUBLIC_PRIVACY_URL=$(sed -n 's/^NEXT_PUBLIC_PRIVACY_URL=//p' "$ENV_FILE" | tail -n 1)
NEXT_PUBLIC_TERMS_VERSION=$(sed -n 's/^NEXT_PUBLIC_TERMS_VERSION=//p' "$ENV_FILE" | tail -n 1)
NEXT_PUBLIC_PRIVACY_VERSION=$(sed -n 's/^NEXT_PUBLIC_PRIVACY_VERSION=//p' "$ENV_FILE" | tail -n 1)
if [ -z "$PORTAL_DOMAIN" ] || [ -z "$PORTAL_ORIGIN" ] || [ -z "$WEBTUI_APP_LINK_HOST" ] || [ -z "$TERMS_VERSION" ] || [ -z "$PRIVACY_POLICY_VERSION" ]; then
  echo "PORTAL_DOMAIN, PORTAL_ORIGIN, WEBTUI_APP_LINK_HOST, TERMS_VERSION and PRIVACY_POLICY_VERSION are required." >&2
  exit 1
fi
if [ "$NEXT_PUBLIC_TERMS_URL" != "$PORTAL_ORIGIN/terms" ] || [ "$NEXT_PUBLIC_PRIVACY_URL" != "$PORTAL_ORIGIN/privacy" ] || [ "$NEXT_PUBLIC_TERMS_VERSION" != "$TERMS_VERSION" ] || [ "$NEXT_PUBLIC_PRIVACY_VERSION" != "$PRIVACY_POLICY_VERSION" ]; then
  echo "Frontend legal URLs/versions must match PORTAL_ORIGIN and backend policy versions." >&2
  exit 1
fi
ENABLE_IOS_ASSOCIATION=$(printf '%s' "${ENABLE_IOS_ASSOCIATION:-false}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | tr '[:upper:]' '[:lower:]')
case "$ENABLE_IOS_ASSOCIATION" in
  1|true|yes|on) ENABLE_IOS_ASSOCIATION=true ;;
  0|false|no|off) ENABLE_IOS_ASSOCIATION=false ;;
  *)
    echo "ENABLE_IOS_ASSOCIATION must be true or false." >&2
    exit 1
    ;;
esac
if [ -n "$INSTANCE_DOMAIN" ] && command -v curl >/dev/null 2>&1; then
  curl -fsS --max-time 15 "https://$INSTANCE_DOMAIN/ready" >/dev/null
  echo "Public HTTPS: OK (https://$INSTANCE_DOMAIN/ready)"
  curl -fsS --max-time 15 "https://$INSTANCE_DOMAIN:8443/" >/dev/null
  echo "Group meetings: OK (https://$INSTANCE_DOMAIN:8443/)"
fi

if [ "$INSTANCE_DOMAIN" = "$WEBTUI_APP_LINK_HOST" ] && command -v curl >/dev/null 2>&1; then
  association_url="https://$WEBTUI_APP_LINK_HOST/.well-known/assetlinks.json"
  association_status=$(curl -sS --max-time 15 -o /dev/null -w '%{http_code}' "$association_url")
  if [ "$association_status" != "200" ]; then
    echo "Publisher Android association endpoint returned HTTP $association_status: $association_url" >&2
    exit 1
  fi
  echo "Publisher Android association: OK ($association_url)"

  association_url="https://$WEBTUI_APP_LINK_HOST/.well-known/apple-app-site-association"
  association_status=$(curl -sS --max-time 15 -o /dev/null -w '%{http_code}' "$association_url")
  if [ "$ENABLE_IOS_ASSOCIATION" = "true" ]; then
    if [ "$association_status" != "200" ]; then
      echo "Publisher iOS association must return 200 when enabled: $association_url (HTTP $association_status)" >&2
      exit 1
    fi
    echo "Publisher iOS association: OK ($association_url)"
  else
    case "$association_status" in
      404|410) echo "Publisher iOS association: disabled fail-closed (HTTP $association_status)" ;;
      *)
        echo "Publisher iOS association must return 404/410 while disabled: $association_url (HTTP $association_status)" >&2
        exit 1
        ;;
    esac
  fi
elif [ "$INSTANCE_DOMAIN" != "$WEBTUI_APP_LINK_HOST" ]; then
  echo "Publisher mobile association: external (customer domain uses manual server entry)"
fi

PUSH_RELAY_URL=$(sed -n 's/^PUSH_RELAY_URL=//p' "$ENV_FILE" | tail -n 1)
PUSH_RELAY_TOKEN=$(sed -n 's/^PUSH_RELAY_TOKEN=//p' "$ENV_FILE" | tail -n 1)
PUSH_RELAY_INSTANCE_ID=$(sed -n 's/^PUSH_RELAY_INSTANCE_ID=//p' "$ENV_FILE" | tail -n 1)
FIREBASE_PROJECT_ID=$(sed -n 's/^FIREBASE_PROJECT_ID=//p' "$ENV_FILE" | tail -n 1)
APNS_KEY_ID=$(sed -n 's/^APNS_KEY_ID=//p' "$ENV_FILE" | tail -n 1)
WEB_PUSH_ENABLED=$(sed -n 's/^WEB_PUSH_ENABLED=//p' "$ENV_FILE" | tail -n 1)
WEB_PUSH_PUBLIC_KEY=$(sed -n 's/^WEB_PUSH_VAPID_PUBLIC_KEY=//p' "$ENV_FILE" | tail -n 1)
WEB_PUSH_PRIVATE_KEY=$(sed -n 's/^WEB_PUSH_VAPID_PRIVATE_KEY=//p' "$ENV_FILE" | tail -n 1)
WEB_PUSH_SUBJECT=$(sed -n 's/^WEB_PUSH_VAPID_SUBJECT=//p' "$ENV_FILE" | tail -n 1)
PUSH_RELAY_SERVER_ENABLED=$(sed -n 's/^PUSH_RELAY_SERVER_ENABLED=//p' "$ENV_FILE" | tail -n 1)
PUSH_RELAY_PUBLISHERS=$(sed -n 's/^PUSH_RELAY_PUBLISHERS=//p' "$ENV_FILE" | tail -n 1)
PUSH_RELAY_HTTP_PORT=$(sed -n 's/^PUSH_RELAY_HTTP_PORT=//p' "$ENV_FILE" | tail -n 1)
PUSH_RELAY_HTTP_PORT=${PUSH_RELAY_HTTP_PORT:-8090}
case "$PUSH_RELAY_HTTP_PORT" in
  *[!0-9]*|"")
    echo "PUSH_RELAY_HTTP_PORT must be an integer from 1 to 65535." >&2
    exit 1
    ;;
esac
if [ "$PUSH_RELAY_HTTP_PORT" -lt 1 ] || [ "$PUSH_RELAY_HTTP_PORT" -gt 65535 ]; then
  echo "PUSH_RELAY_HTTP_PORT must be an integer from 1 to 65535." >&2
  exit 1
fi
if [ "$PUSH_RELAY_HTTP_PORT" -ne 8090 ]; then
  echo "Bundled self-host Caddy/Prometheus require PUSH_RELAY_HTTP_PORT=8090; use a dedicated relay deployment for another port." >&2
  exit 1
fi
PUSH_RELAY_SERVER_ENABLED=$(printf '%s' "$PUSH_RELAY_SERVER_ENABLED" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | tr '[:upper:]' '[:lower:]')
case "$PUSH_RELAY_SERVER_ENABLED" in
  1|true|yes|on) PUSH_RELAY_SERVER_ENABLED=true ;;
  ""|0|false|no|off) PUSH_RELAY_SERVER_ENABLED=false ;;
  *)
    echo "PUSH_RELAY_SERVER_ENABLED must be true or false." >&2
    exit 1
    ;;
esac

if [ -n "$PUSH_RELAY_URL$PUSH_RELAY_TOKEN$PUSH_RELAY_INSTANCE_ID" ]; then
  if [ -z "$PUSH_RELAY_URL" ] || [ -z "$PUSH_RELAY_TOKEN" ] || [ -z "$PUSH_RELAY_INSTANCE_ID" ]; then
    echo "Push: invalid partial relay configuration" >&2
    exit 1
  fi
  if ! printf '%s\n' "$PUSH_RELAY_INSTANCE_ID" | grep -Eiq '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'; then
    echo "Push: PUSH_RELAY_INSTANCE_ID must equal discovery zone.id (UUID), not a customer slug" >&2
    exit 1
  fi
  case "$PUSH_RELAY_URL" in
    https://*/push-relay/v1/deliveries) ;;
    *)
      echo "Push: bundled Caddy relay URL must be HTTPS and end in /push-relay/v1/deliveries" >&2
      exit 1
      ;;
  esac
  echo "Push: publisher relay configured"
elif [ -n "$FIREBASE_PROJECT_ID" ] || [ -n "$APNS_KEY_ID" ]; then
  echo "Push: direct FCM/APNs configuration detected"
else
  echo "Push: disabled (foreground WebSocket and in-app notification center still work)"
fi

if [ "$WEB_PUSH_ENABLED" = "true" ]; then
  if [ -z "$WEB_PUSH_PUBLIC_KEY" ] || [ -z "$WEB_PUSH_PRIVATE_KEY" ] || [ -z "$WEB_PUSH_SUBJECT" ]; then
    echo "Web Push: enabled but VAPID configuration is incomplete" >&2
    exit 1
  fi
  echo "Web Push: enabled with per-instance VAPID"
else
  echo "Web Push: disabled"
fi

if [ "$PUSH_RELAY_SERVER_ENABLED" = "true" ]; then
  if [ -z "$PUSH_RELAY_PUBLISHERS" ]; then
    echo "Push relay server: enabled but publisher credentials are missing" >&2
    exit 1
  fi
  if docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" --profile push-relay ps --status running --services | grep -qx push-relay; then
    docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" --profile push-relay exec -T push-relay wget -qO- "http://localhost:$PUSH_RELAY_HTTP_PORT/ready" >/dev/null
    echo "Push relay server: ready"
  else
    echo "Push relay server: enabled but container is not running; start the push-relay profile" >&2
    exit 1
  fi
else
  echo "Push relay server: disabled"
fi

QUEUE_SUMMARY=$(docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T postgres sh -c \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "SELECT status || E'\''='\'' || count(*) FROM notification_jobs GROUP BY status ORDER BY status"' \
  2>/dev/null || true)
if [ -n "$QUEUE_SUMMARY" ]; then
  echo "Notification jobs:"
  printf '%s\n' "$QUEUE_SUMMARY"
fi
