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
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T api wget -qO- http://localhost:8080/ready
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
if [ -n "$INSTANCE_DOMAIN" ] && command -v curl >/dev/null 2>&1; then
  curl -fsS --max-time 15 "https://$INSTANCE_DOMAIN/ready" >/dev/null
  echo "Public HTTPS: OK (https://$INSTANCE_DOMAIN/ready)"
  curl -fsS --max-time 15 "https://$INSTANCE_DOMAIN:8443/" >/dev/null
  echo "Group meetings: OK (https://$INSTANCE_DOMAIN:8443/)"
fi

if command -v curl >/dev/null 2>&1; then
  for association_path in assetlinks.json apple-app-site-association; do
    association_url="https://$WEBTUI_APP_LINK_HOST/.well-known/$association_path"
    association_status=$(curl -sS --max-time 15 -o /dev/null -w '%{http_code}' "$association_url")
    if [ "$association_status" != "200" ]; then
      echo "Mobile association endpoint returned HTTP $association_status: $association_url" >&2
      exit 1
    fi
    echo "Mobile association: OK ($association_url)"
  done
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

if [ -n "$PUSH_RELAY_URL$PUSH_RELAY_TOKEN$PUSH_RELAY_INSTANCE_ID" ]; then
  if [ -z "$PUSH_RELAY_URL" ] || [ -z "$PUSH_RELAY_TOKEN" ] || [ -z "$PUSH_RELAY_INSTANCE_ID" ]; then
    echo "Push: invalid partial relay configuration" >&2
    exit 1
  fi
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
    docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" --profile push-relay exec -T push-relay wget -qO- http://localhost:8090/ready >/dev/null
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
