#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
COMPOSE_FILE="$SCRIPT_DIR/compose.yml"
ENV_FILE="$SCRIPT_DIR/.env"
OFFSITE_ENV_FILE="$SCRIPT_DIR/offsite-backup.env"
DOMAIN=""
EMAIL=""
INSTANCE_NAME="Team Chat"
INSTANCE_LOGO_URL=""
INSTANCE_REGISTRATION_MODE="open"
TURN_EXTERNAL_IP=""
PORTAL_ORIGIN="https://download.vpsttt.com"
PORTAL_DOMAIN="download.vpsttt.com"
FORCE=0
SKIP_DNS_CHECK=0

usage() {
  echo "Usage: $0 --domain chat.example.com --email admin@example.com [--name 'Example Chat'] [--logo-url https://chat.example.com/logo.png] [--registration-mode open|invite_only|closed] [--portal-origin https://download.vpsttt.com] [--external-ip 203.0.113.10] [--skip-dns-check] [--force]"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --domain) DOMAIN=${2:-}; shift 2 ;;
    --email) EMAIL=${2:-}; shift 2 ;;
    --name) INSTANCE_NAME=${2:-}; shift 2 ;;
    --logo-url) INSTANCE_LOGO_URL=${2:-}; shift 2 ;;
    --registration-mode) INSTANCE_REGISTRATION_MODE=${2:-}; shift 2 ;;
    --portal-origin) PORTAL_ORIGIN=${2:-}; shift 2 ;;
    --external-ip) TURN_EXTERNAL_IP=${2:-}; shift 2 ;;
    --skip-dns-check) SKIP_DNS_CHECK=1; shift ;;
    --force) FORCE=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if [ -z "$DOMAIN" ] || [ -z "$EMAIL" ]; then
  usage
  exit 1
fi
if ! printf '%s' "$DOMAIN" | grep -Eq '^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$'; then
  echo "Invalid public domain: $DOMAIN" >&2
  exit 1
fi
DOMAIN=$(printf '%s' "$DOMAIN" | tr '[:upper:]' '[:lower:]')
if ! printf '%s' "$EMAIL" | grep -Eq '^[^[:space:]@]+@[^[:space:]@]+\.[^[:space:]@]+$'; then
  echo "Invalid email: $EMAIL" >&2
  exit 1
fi
if [ -z "$INSTANCE_NAME" ] ||
  printf '%s' "$INSTANCE_NAME" | grep -Eq '[#=]|[[:cntrl:]]'; then
  echo "Instance name must not be empty or contain #, =, or control characters." >&2
  exit 1
fi
if [ -n "$INSTANCE_LOGO_URL" ] &&
  ! printf '%s' "$INSTANCE_LOGO_URL" | grep -Eq '^https://[^/@[:space:]]+(/[^[:space:]]*)?$'; then
  echo "Logo URL must be a public HTTPS URL without embedded credentials." >&2
  exit 1
fi
case "$INSTANCE_REGISTRATION_MODE" in
  open|invite_only|closed) ;;
  *) echo "Registration mode must be open, invite_only or closed." >&2; exit 1 ;;
esac
if ! printf '%s' "$PORTAL_ORIGIN" | grep -Eq '^https://([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}(:[0-9]{1,5})?$'; then
  echo "Portal origin must be an HTTPS origin without a path." >&2
  exit 1
fi
PORTAL_DOMAIN=${PORTAL_ORIGIN#https://}
if printf '%s' "$PORTAL_DOMAIN" | grep -q ':'; then
  echo "Portal origin must use the standard HTTPS port so its hostname can be used for SNI." >&2
  exit 1
fi
for command_name in docker openssl curl; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "Missing required command: $command_name" >&2
    exit 1
  fi
done
if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose v2 is required." >&2
  exit 1
fi
if [ -f "$ENV_FILE" ] && [ "$FORCE" -ne 1 ]; then
  echo "$ENV_FILE already exists. Use --force only for a new instance that has no data." >&2
  exit 1
fi

if [ -z "$TURN_EXTERNAL_IP" ]; then
  TURN_EXTERNAL_IP=$(curl -4fsS --max-time 10 https://api.ipify.org || true)
fi
if ! printf '%s\n' "$TURN_EXTERNAL_IP" | awk -F. '
  NF != 4 { exit 1 }
  {
    for (i = 1; i <= 4; i++) {
      if ($i !~ /^[0-9]+$/ || $i < 0 || $i > 255) {
        exit 1
      }
    }
  }
'; then
  echo "Unable to determine a public IPv4 address. Pass --external-ip." >&2
  exit 1
fi

if [ "$SKIP_DNS_CHECK" -ne 1 ]; then
  DNS_IPS=""
  if command -v getent >/dev/null 2>&1; then
    DNS_IPS=$(getent ahostsv4 "$DOMAIN" | awk '{print $1}' | sort -u || true)
  fi
  if ! printf '%s\n' "$DNS_IPS" | grep -Fx "$TURN_EXTERNAL_IP" >/dev/null 2>&1; then
    GOOGLE_DNS_IPS=$(curl -4fsS --max-time 10 "https://dns.google/resolve?name=$DOMAIN&type=A" |
      sed 's/[{},]/\
/g' |
      awk -F: '/"data"/ { gsub(/[" ]/, "", $2); print $2 }' |
      sort -u || true)
    DNS_IPS=$(printf '%s\n%s\n' "$DNS_IPS" "$GOOGLE_DNS_IPS" | awk 'NF' | sort -u)
  fi
  if ! printf '%s\n' "$DNS_IPS" | grep -Fx "$TURN_EXTERNAL_IP" >/dev/null 2>&1; then
    echo "DNS for $DOMAIN must contain A record $TURN_EXTERNAL_IP before installation." >&2
    echo "Resolved IPv4 addresses: ${DNS_IPS:-none}" >&2
    echo "If your DNS panel already shows this record, wait for propagation or rerun with --skip-dns-check." >&2
    exit 1
  fi
fi

POSTGRES_PASSWORD=$(openssl rand -hex 24)
REDIS_PASSWORD=$(openssl rand -hex 24)
RABBITMQ_PASSWORD=$(openssl rand -hex 24)
JWT_ACCESS_SECRET=$(openssl rand -hex 48)
JWT_REFRESH_SECRET=$(openssl rand -hex 48)
WEBHOOK_SIGNING_SECRET=$(openssl rand -hex 48)
BOT_AI_SECRET_KEY=$(openssl rand -hex 48)
OIDC_STATE_SECRET=$(openssl rand -hex 48)
TURN_SHARED_SECRET=$(openssl rand -hex 48)
JICOFO_AUTH_PASSWORD=$(openssl rand -hex 32)
JVB_AUTH_PASSWORD=$(openssl rand -hex 32)
GRAFANA_ADMIN_PASSWORD=$(openssl rand -hex 24)

umask 077
cat > "$ENV_FILE" <<EOF
DEPLOYMENT_MODE=self_hosted
INSTANCE_DOMAIN=$DOMAIN
INSTANCE_NAME=$INSTANCE_NAME
INSTANCE_LOGO_URL=$INSTANCE_LOGO_URL
INSTANCE_REGISTRATION_MODE=$INSTANCE_REGISTRATION_MODE
APP_ENV=production
APP_NAME=$INSTANCE_NAME
APP_URL=https://$DOMAIN
APP_VERSION=self-hosted
LOG_LEVEL=info
LOG_FORMAT=json
TERMS_VERSION=2026-08-07
PRIVACY_POLICY_VERSION=2026-08-07
NEXT_PUBLIC_TERMS_URL=$PORTAL_ORIGIN/terms
NEXT_PUBLIC_PRIVACY_URL=$PORTAL_ORIGIN/privacy
NEXT_PUBLIC_TERMS_VERSION=2026-08-07
NEXT_PUBLIC_PRIVACY_VERSION=2026-08-07
API_HTTP_HOST=0.0.0.0
API_HTTP_PORT=8080
TRUSTED_PROXIES=172.16.0.0/12,127.0.0.1
PORTAL_ORIGIN=$PORTAL_ORIGIN
PORTAL_DOMAIN=$PORTAL_DOMAIN
WEBTUI_APP_LINK_HOST=$DOMAIN
DESKTOP_DOWNLOAD_URL=$PORTAL_ORIGIN/download/
MOBILE_DOWNLOAD_URL=$PORTAL_ORIGIN/download/
DOCUMENTATION_URL=$PORTAL_ORIGIN/#self-host
CORS_ALLOWED_ORIGINS=https://$DOMAIN,$PORTAL_ORIGIN,http://tauri.localhost,https://tauri.localhost,tauri://localhost
SECURE_HEADERS_ENABLED=true
RATE_LIMIT_ENABLED=true
RATE_LIMIT_PER_MINUTE=120
RATE_LIMIT_BURST=60
METRICS_ENABLED=true
METRICS_PATH=/metrics
OTEL_ENABLED=false
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
OTEL_TRACE_SAMPLE_RATIO=0.10
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=$GRAFANA_ADMIN_PASSWORD
GRAFANA_BIND_ADDRESS=127.0.0.1
GRAFANA_PORT=3300
DATABASE_ENABLED=true
POSTGRES_DB=vpsttt_chat
POSTGRES_USER=vpsttt
POSTGRES_PASSWORD=$POSTGRES_PASSWORD
DATABASE_URL=postgres://vpsttt:$POSTGRES_PASSWORD@postgres:5432/vpsttt_chat?sslmode=disable
REDIS_ENABLED=true
REDIS_PASSWORD=$REDIS_PASSWORD
REDIS_URL=redis://:$REDIS_PASSWORD@redis:6379/0
RABBITMQ_ENABLED=true
RABBITMQ_USER=vpsttt
RABBITMQ_PASSWORD=$RABBITMQ_PASSWORD
RABBITMQ_URL=amqp://vpsttt:$RABBITMQ_PASSWORD@rabbitmq:5672/
STORAGE_PROVIDER=local
LOCAL_STORAGE_PATH=/var/lib/vpsttt-chat/storage
BACKUP_PG_DUMP_PATH=pg_dump
BACKUP_TIMEOUT=30m
JWT_ACCESS_SECRET=$JWT_ACCESS_SECRET
JWT_REFRESH_SECRET=$JWT_REFRESH_SECRET
WEBHOOK_SIGNING_SECRET=$WEBHOOK_SIGNING_SECRET
BOT_AI_SECRET_KEY=$BOT_AI_SECRET_KEY
OIDC_STATE_SECRET=$OIDC_STATE_SECRET
OIDC_CLIENT_SECRETS=
GOOGLE_CLIENT_ID=
FIREBASE_PROJECT_ID=
FIREBASE_SERVICE_ACCOUNT_FILE=
FIREBASE_SERVICE_ACCOUNT_JSON_BASE64=
APNS_KEY_ID=
APNS_TEAM_ID=
APNS_BUNDLE_ID=com.vpsttt.webtuiChat
APNS_PRIVATE_KEY_FILE=
APNS_PRIVATE_KEY_BASE64=
APNS_SANDBOX=false
PUSH_RELAY_URL=
PUSH_RELAY_TOKEN=
PUSH_RELAY_INSTANCE_ID=
WEB_PUSH_ENABLED=false
WEB_PUSH_VAPID_PUBLIC_KEY=
WEB_PUSH_VAPID_PRIVATE_KEY=
WEB_PUSH_VAPID_SUBJECT=mailto:$EMAIL
WEB_PUSH_TTL_SECONDS=300
WEB_PUSH_MAX_SUBSCRIPTIONS_PER_USER=10
PUSH_RELAY_SERVER_ENABLED=false
PUSH_RELAY_HTTP_HOST=0.0.0.0
PUSH_RELAY_HTTP_PORT=8090
PUSH_RELAY_PUBLISHERS=
PUSH_RELAY_MAX_BODY_BYTES=32768
PUSH_RELAY_RATE_LIMIT_PER_MINUTE=240
PUSH_RELAY_RATE_LIMIT_BURST=60
PUSH_RELAY_WORKER_CONCURRENCY=4
PUSH_RELAY_POLL_INTERVAL=1s
ORDER_API_BASE_URL=
ORDER_INTERNAL_API_KEY=
ORDER_QUICK_ORDER_KEY=
MODULE_RUNNER_SCRIPT_ALLOWLIST=
TALK_AI_ALLOWED_HOSTS=ollama,local-ai
BOT_AI_ALLOWED_HOSTS=ollama,local-ai
CALL_RING_TIMEOUT=30s
TURN_EXTERNAL_IP=$TURN_EXTERNAL_IP
TURN_SHARED_SECRET=$TURN_SHARED_SECRET
TURN_URLS=turn:$DOMAIN:3478?transport=udp,turn:$DOMAIN:3478?transport=tcp
TURN_CREDENTIAL_TTL=10m
NEXT_PUBLIC_RTC_ICE_SERVERS=[{"urls":"stun:$DOMAIN:3478"}]
RTC_ICE_SERVERS=[{"urls":"stun:$DOMAIN:3478"}]
NEXT_PUBLIC_JITSI_BASE_URL=https://$DOMAIN:8443
JITSI_BASE_URL=https://$DOMAIN:8443
JICOFO_AUTH_PASSWORD=$JICOFO_AUTH_PASSWORD
JVB_AUTH_PASSWORD=$JVB_AUTH_PASSWORD
LETSENCRYPT_EMAIL=$EMAIL
WORKER_CONCURRENCY=4
MODERATION_EVIDENCE_RETENTION_DAYS=365
EOF

# Dedicated credentials are loaded only by the opt-in backup/restore services.
# The API, worker, web and admin containers never receive these values.
cat > "$OFFSITE_ENV_FILE" <<'EOF'
OFFSITE_BACKUP_ENABLED=false
OFFSITE_S3_ENDPOINT=
OFFSITE_S3_REGION=us-east-1
OFFSITE_S3_BUCKET=
OFFSITE_S3_PREFIX=webtui-chat
OFFSITE_S3_BUCKET_LOOKUP=auto
OFFSITE_S3_ACCESS_KEY_ID=
OFFSITE_S3_SECRET_ACCESS_KEY=
OFFSITE_S3_SESSION_TOKEN=
OFFSITE_S3_STORAGE_CLASS=
OFFSITE_S3_CONNECTIONS=8
OFFSITE_RESTIC_PASSWORD_FILE=
OFFSITE_RESTIC_PASSWORD=
OFFSITE_BACKUP_INTERVAL_SECONDS=86400
OFFSITE_BACKUP_RUN_ON_START=false
OFFSITE_BACKUP_TIMEOUT_SECONDS=7200
OFFSITE_BACKUP_MIN_FREE_BYTES=1073741824
OFFSITE_BACKUP_STAGING_HEADROOM_PERCENT=20
OFFSITE_BACKUP_COMPRESSION=auto
OFFSITE_BACKUP_RETENTION_ENABLED=true
OFFSITE_BACKUP_KEEP_DAILY=7
OFFSITE_BACKUP_KEEP_WEEKLY=4
OFFSITE_BACKUP_KEEP_MONTHLY=12
OFFSITE_BACKUP_KEEP_YEARLY=3
OFFSITE_BACKUP_VERIFY_AFTER_BACKUP=false
OFFSITE_BACKUP_VERIFY_READ_DATA_SUBSET=5%
OFFSITE_BACKUP_SERIALIZABLE_DEFERRABLE=false
OFFSITE_SOURCE_S3_PREFIX=
EOF

cd "$SCRIPT_DIR"
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" config >/dev/null
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" build --pull
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d

echo "Waiting for https://$DOMAIN/ready ..."
attempt=0
until curl -fsS --max-time 10 "https://$DOMAIN/ready" >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 36 ]; then
    echo "Instance did not become ready. Inspect: docker compose -f $COMPOSE_FILE logs" >&2
    exit 1
  fi
  sleep 5
done

echo "Waiting for https://$DOMAIN:8443/ ..."
attempt=0
until curl -fsS --max-time 10 "https://$DOMAIN:8443/" >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 36 ]; then
    echo "Meeting service did not become ready. Inspect Jitsi logs in $COMPOSE_FILE." >&2
    exit 1
  fi
  sleep 5
done

echo "VPSTTT Chat is ready at https://$DOMAIN"
echo "The bundled meeting service is ready at https://$DOMAIN:8443"
echo "Make sure TCP 8443 and UDP 10000 are allowed by the VPS firewall for group video."
echo "Download portal is ready at $PORTAL_ORIGIN"
echo "Register or sign in through $PORTAL_ORIGIN with domain $DOMAIN"
echo "The first account registered on this domain becomes workspace owner."
