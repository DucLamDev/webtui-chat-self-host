#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
COMPOSE_FILE="$SCRIPT_DIR/compose.yml"
ENV_FILE="$SCRIPT_DIR/.env"
DOMAIN=""
EMAIL=""
INSTANCE_NAME="VPSTTT Chat"
TURN_EXTERNAL_IP=""
PORTAL_ORIGIN="https://chat.vpsttt.com"
PORTAL_PATH="/portal"
FORCE=0
SKIP_DNS_CHECK=0

usage() {
  echo "Usage: $0 --domain chat.example.com --email admin@example.com [--name 'Example Chat'] [--portal-origin https://chat.vpsttt.com] [--external-ip 203.0.113.10] [--skip-dns-check] [--force]"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --domain) DOMAIN=${2:-}; shift 2 ;;
    --email) EMAIL=${2:-}; shift 2 ;;
    --name) INSTANCE_NAME=${2:-}; shift 2 ;;
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
if ! printf '%s' "$PORTAL_ORIGIN" | grep -Eq '^https://([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}(:[0-9]{1,5})?$'; then
  echo "Portal origin must be an HTTPS origin without a path." >&2
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
OIDC_STATE_SECRET=$(openssl rand -hex 48)
TURN_PASSWORD=$(openssl rand -hex 24)
TURN_USERNAME=vpsttt-turn

umask 077
cat > "$ENV_FILE" <<EOF
DEPLOYMENT_MODE=self_hosted
INSTANCE_DOMAIN=$DOMAIN
INSTANCE_NAME=$INSTANCE_NAME
APP_ENV=production
APP_NAME=$INSTANCE_NAME
APP_URL=https://$DOMAIN
APP_VERSION=self-hosted
LOG_LEVEL=info
LOG_FORMAT=json
API_HTTP_HOST=0.0.0.0
API_HTTP_PORT=8080
TRUSTED_PROXIES=172.16.0.0/12,127.0.0.1
PORTAL_ORIGIN=$PORTAL_ORIGIN
CORS_ALLOWED_ORIGINS=https://$DOMAIN,$PORTAL_ORIGIN,http://tauri.localhost,https://tauri.localhost,tauri://localhost
SECURE_HEADERS_ENABLED=true
RATE_LIMIT_ENABLED=true
RATE_LIMIT_PER_MINUTE=120
RATE_LIMIT_BURST=60
METRICS_ENABLED=true
METRICS_PATH=/metrics
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
OIDC_STATE_SECRET=$OIDC_STATE_SECRET
OIDC_CLIENT_SECRETS=
GOOGLE_CLIENT_ID=
FIREBASE_PROJECT_ID=
FIREBASE_SERVICE_ACCOUNT_JSON_BASE64=
ORDER_API_BASE_URL=
ORDER_INTERNAL_API_KEY=
ORDER_QUICK_ORDER_KEY=
MODULE_RUNNER_SCRIPT_ALLOWLIST=
CALL_RING_TIMEOUT=30s
TURN_EXTERNAL_IP=$TURN_EXTERNAL_IP
TURN_USERNAME=$TURN_USERNAME
TURN_PASSWORD=$TURN_PASSWORD
NEXT_PUBLIC_RTC_ICE_SERVERS=[{"urls":"stun:$DOMAIN:3478"},{"urls":"turn:$DOMAIN:3478?transport=udp","username":"$TURN_USERNAME","credential":"$TURN_PASSWORD"},{"urls":"turn:$DOMAIN:3478?transport=tcp","username":"$TURN_USERNAME","credential":"$TURN_PASSWORD"}]
RTC_ICE_SERVERS=[{"urls":"stun:$DOMAIN:3478"},{"urls":"turn:$DOMAIN:3478?transport=udp","username":"$TURN_USERNAME","credential":"$TURN_PASSWORD"},{"urls":"turn:$DOMAIN:3478?transport=tcp","username":"$TURN_USERNAME","credential":"$TURN_PASSWORD"}]
LETSENCRYPT_EMAIL=$EMAIL
WORKER_CONCURRENCY=4
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

echo "VPSTTT Chat is ready at https://$DOMAIN"
echo "Register or sign in through $PORTAL_ORIGIN$PORTAL_PATH with domain $DOMAIN"
echo "The first account registered on this domain becomes workspace owner."
