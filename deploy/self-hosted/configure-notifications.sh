#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ENV_FILE="$SCRIPT_DIR/.env"
COMPOSE_FILE="$SCRIPT_DIR/compose.yml"

PUSH_MODE=""
FIREBASE_PROJECT_ID_INPUT=""
FIREBASE_SERVICE_ACCOUNT_JSON_FILE=""
FIREBASE_SERVICE_ACCOUNT_JSON_BASE64_INPUT=""
PUSH_RELAY_URL_INPUT=""
PUSH_RELAY_TOKEN_INPUT=""
PUSH_RELAY_INSTANCE_ID_INPUT=""
PUSH_RELAY_PUBLISHERS_INPUT=""
ENABLE_WEB_PUSH=""
RESTART=0

usage() {
  cat <<'EOF'
Usage: configure-notifications.sh [options]

Push modes:
  --push-mode none
  --push-mode direct-fcm --firebase-service-account-json /path/adminsdk.json
  --push-mode relay-client --push-relay-url https://relay.example/push-relay/v1/deliveries [--push-relay-token TOKEN]
  --push-mode relay-server --firebase-service-account-json /path/adminsdk.json [--push-relay-publishers UUID=TOKEN;UUID=TOKEN]

Options:
  --firebase-project-id PROJECT_ID               Defaults to webtui-chat for direct/relay-server FCM.
  --firebase-service-account-json FILE           Firebase Admin SDK service-account JSON file.
  --firebase-service-account-base64 BASE64       Firebase Admin SDK service-account JSON as base64.
  --push-relay-url URL                           Relay client URL ending in /push-relay/v1/deliveries.
  --push-relay-token TOKEN                       Relay client token. Generated when omitted in relay-client mode.
  --push-relay-instance-id UUID                  Usually auto-read from /api/v1/discovery.
  --push-relay-publishers MAP                    Semicolon separated UUID=TOKEN relay publisher map.
  --enable-web-push                              Generate per-instance VAPID keys when missing.
  --disable-web-push                             Disable browser Web Push.
  --restart                                     Recreate affected services after updating .env.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --push-mode) PUSH_MODE=${2:-}; shift 2 ;;
    --firebase-project-id) FIREBASE_PROJECT_ID_INPUT=${2:-}; shift 2 ;;
    --firebase-service-account-json) FIREBASE_SERVICE_ACCOUNT_JSON_FILE=${2:-}; shift 2 ;;
    --firebase-service-account-base64) FIREBASE_SERVICE_ACCOUNT_JSON_BASE64_INPUT=${2:-}; shift 2 ;;
    --push-relay-url) PUSH_RELAY_URL_INPUT=${2:-}; shift 2 ;;
    --push-relay-token) PUSH_RELAY_TOKEN_INPUT=${2:-}; shift 2 ;;
    --push-relay-instance-id) PUSH_RELAY_INSTANCE_ID_INPUT=${2:-}; shift 2 ;;
    --push-relay-publishers) PUSH_RELAY_PUBLISHERS_INPUT=${2:-}; shift 2 ;;
    --enable-web-push) ENABLE_WEB_PUSH=true; shift ;;
    --disable-web-push) ENABLE_WEB_PUSH=false; shift ;;
    --restart) RESTART=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if [ ! -f "$ENV_FILE" ]; then
  echo "Missing $ENV_FILE. Run install.sh first." >&2
  exit 1
fi

read_env() {
  sed -n "s/^$1=//p" "$ENV_FILE" | tail -n 1
}

write_env() {
  key=$1
  value=$2
  tmp_file=$(mktemp)
  awk -v key="$key" -v value="$value" '
    BEGIN { replaced = 0 }
    index($0, key "=") == 1 {
      print key "=" value
      replaced = 1
      next
    }
    { print }
    END {
      if (replaced == 0) {
        print key "=" value
      }
    }
  ' "$ENV_FILE" > "$tmp_file"
  cat "$tmp_file" > "$ENV_FILE"
  rm -f "$tmp_file"
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

is_uuid() {
  printf '%s\n' "$1" | grep -Eq '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
}

random_token() {
  openssl rand -hex 32
}

normalized_bool_env() {
  value=$(printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | tr '[:upper:]' '[:lower:]')
  case "$value" in
    1|true|yes|on) printf '%s\n' true ;;
    ""|0|false|no|off) printf '%s\n' false ;;
    *) echo "$2 must be true or false." >&2; exit 1 ;;
  esac
}

base64_file() {
  if [ ! -r "$1" ]; then
    echo "Cannot read Firebase service account JSON: $1" >&2
    exit 1
  fi
  if ! grep -q '"service_account"' "$1" || ! grep -q '"private_key"' "$1" || ! grep -q '"client_email"' "$1"; then
    echo "Firebase credential must be an Admin SDK service-account JSON, not google-services.json." >&2
    exit 1
  fi
  base64 < "$1" | tr -d '\n\r'
}

public_instance_id() {
  if [ -n "$PUSH_RELAY_INSTANCE_ID_INPUT" ]; then
    instance_id=$(printf '%s' "$PUSH_RELAY_INSTANCE_ID_INPUT" | tr '[:upper:]' '[:lower:]')
    if ! is_uuid "$instance_id"; then
      echo "PUSH_RELAY_INSTANCE_ID must be a canonical lowercase UUID." >&2
      exit 1
    fi
    printf '%s\n' "$instance_id"
    return
  fi

  domain=$(read_env INSTANCE_DOMAIN)
  if [ -z "$domain" ]; then
    echo "INSTANCE_DOMAIN is required to auto-detect discovery instance_id." >&2
    exit 1
  fi
  require_command curl
  body=$(curl -fsS --max-time 15 "https://$domain/api/v1/discovery?domain=$domain")
  instance_id=$(printf '%s\n' "$body" |
    sed -n 's/.*"instance_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
    head -n 1 |
    tr '[:upper:]' '[:lower:]')
  if ! is_uuid "$instance_id"; then
    echo "Could not read a canonical instance_id from https://$domain/api/v1/discovery?domain=$domain" >&2
    exit 1
  fi
  printf '%s\n' "$instance_id"
}

service_account_base64() {
  if [ -n "$FIREBASE_SERVICE_ACCOUNT_JSON_FILE" ]; then
    base64_file "$FIREBASE_SERVICE_ACCOUNT_JSON_FILE"
    return
  fi
  if [ -n "$FIREBASE_SERVICE_ACCOUNT_JSON_BASE64_INPUT" ]; then
    printf '%s\n' "$FIREBASE_SERVICE_ACCOUNT_JSON_BASE64_INPUT"
    return
  fi
  read_env FIREBASE_SERVICE_ACCOUNT_JSON_BASE64
}

firebase_project_id() {
  if [ -n "$FIREBASE_PROJECT_ID_INPUT" ]; then
    printf '%s\n' "$FIREBASE_PROJECT_ID_INPUT"
    return
  fi
  existing=$(read_env FIREBASE_PROJECT_ID)
  if [ -n "$existing" ]; then
    printf '%s\n' "$existing"
    return
  fi
  printf '%s\n' "webtui-chat"
}

clear_native_push() {
  write_env FIREBASE_PROJECT_ID ""
  write_env FIREBASE_SERVICE_ACCOUNT_FILE ""
  write_env FIREBASE_SERVICE_ACCOUNT_JSON_BASE64 ""
  write_env APNS_KEY_ID ""
  write_env APNS_TEAM_ID ""
  write_env APNS_PRIVATE_KEY_FILE ""
  write_env APNS_PRIVATE_KEY_BASE64 ""
  write_env APNS_SANDBOX "false"
}

clear_relay_client() {
  write_env PUSH_RELAY_URL ""
  write_env PUSH_RELAY_TOKEN ""
  write_env PUSH_RELAY_INSTANCE_ID ""
}

configure_none() {
  clear_native_push
  clear_relay_client
  write_env PUSH_RELAY_SERVER_ENABLED "false"
  write_env PUSH_RELAY_PUBLISHERS ""
}

configure_direct_fcm() {
  project_id=$(firebase_project_id)
  credential_base64=$(service_account_base64)
  if [ -z "$project_id" ] || [ -z "$credential_base64" ]; then
    echo "direct-fcm requires FIREBASE_PROJECT_ID and Firebase Admin SDK service-account JSON/base64." >&2
    exit 1
  fi
  write_env FIREBASE_PROJECT_ID "$project_id"
  write_env FIREBASE_SERVICE_ACCOUNT_FILE ""
  write_env FIREBASE_SERVICE_ACCOUNT_JSON_BASE64 "$credential_base64"
  write_env APNS_KEY_ID ""
  write_env APNS_TEAM_ID ""
  write_env APNS_PRIVATE_KEY_FILE ""
  write_env APNS_PRIVATE_KEY_BASE64 ""
  write_env APNS_SANDBOX "false"
  clear_relay_client
  write_env PUSH_RELAY_SERVER_ENABLED "false"
  write_env PUSH_RELAY_PUBLISHERS ""
}

configure_relay_client() {
  relay_url=${PUSH_RELAY_URL_INPUT:-$(read_env PUSH_RELAY_URL)}
  if [ -z "$relay_url" ]; then
    echo "relay-client requires --push-relay-url." >&2
    exit 1
  fi
  case "$relay_url" in
    https://*/push-relay/v1/deliveries) ;;
    *) echo "PUSH_RELAY_URL must be HTTPS and end in /push-relay/v1/deliveries." >&2; exit 1 ;;
  esac
  relay_token=${PUSH_RELAY_TOKEN_INPUT:-$(read_env PUSH_RELAY_TOKEN)}
  if [ -z "$relay_token" ]; then
    relay_token=$(random_token)
  fi
  if [ "${#relay_token}" -lt 32 ]; then
    echo "PUSH_RELAY_TOKEN must be at least 32 characters." >&2
    exit 1
  fi
  instance_id=$(public_instance_id)

  clear_native_push
  write_env PUSH_RELAY_URL "$relay_url"
  write_env PUSH_RELAY_TOKEN "$relay_token"
  write_env PUSH_RELAY_INSTANCE_ID "$instance_id"
  write_env PUSH_RELAY_SERVER_ENABLED "false"
  write_env PUSH_RELAY_PUBLISHERS ""
}

configure_relay_server() {
  project_id=$(firebase_project_id)
  credential_base64=$(service_account_base64)
  if [ -z "$project_id" ] || [ -z "$credential_base64" ]; then
    echo "relay-server requires FIREBASE_PROJECT_ID and Firebase Admin SDK service-account JSON/base64." >&2
    exit 1
  fi
  publishers=$PUSH_RELAY_PUBLISHERS_INPUT
  if [ -z "$publishers" ]; then
    instance_id=$(public_instance_id)
    relay_token=${PUSH_RELAY_TOKEN_INPUT:-$(read_env PUSH_RELAY_TOKEN)}
    if [ -z "$relay_token" ]; then
      relay_token=$(random_token)
    fi
    publishers="$instance_id=$relay_token"
  fi

  write_env FIREBASE_PROJECT_ID "$project_id"
  write_env FIREBASE_SERVICE_ACCOUNT_FILE ""
  write_env FIREBASE_SERVICE_ACCOUNT_JSON_BASE64 "$credential_base64"
  clear_relay_client
  write_env PUSH_RELAY_SERVER_ENABLED "true"
  write_env PUSH_RELAY_PUBLISHERS "$publishers"
}

repair_existing_relay_server() {
  relay_server_enabled=$(normalized_bool_env "$(read_env PUSH_RELAY_SERVER_ENABLED)" "PUSH_RELAY_SERVER_ENABLED")
  if [ "$relay_server_enabled" != "true" ]; then
    return
  fi
  if [ -n "$(read_env PUSH_RELAY_PUBLISHERS)" ]; then
    return
  fi
  if [ -z "$(read_env FIREBASE_SERVICE_ACCOUNT_FILE)$(read_env FIREBASE_SERVICE_ACCOUNT_JSON_BASE64)$(read_env APNS_PRIVATE_KEY_FILE)$(read_env APNS_PRIVATE_KEY_BASE64)" ]; then
    echo "PUSH_RELAY_SERVER_ENABLED=true but provider credentials are missing. Pass --push-mode relay-server with Firebase/APNs credentials, or set --push-mode none." >&2
    exit 1
  fi

  instance_id=$(public_instance_id)
  relay_token=${PUSH_RELAY_TOKEN_INPUT:-$(read_env PUSH_RELAY_TOKEN)}
  if [ -z "$relay_token" ]; then
    relay_token=$(random_token)
  fi
  if [ "${#relay_token}" -lt 32 ]; then
    echo "PUSH_RELAY_TOKEN must be at least 32 characters when reused as a relay publisher token." >&2
    exit 1
  fi
  clear_relay_client
  write_env PUSH_RELAY_PUBLISHERS "$instance_id=$relay_token"
  echo "Generated PUSH_RELAY_PUBLISHERS for relay server publisher $instance_id."
}

generate_vapid() {
  require_command docker
  if ! docker compose version >/dev/null 2>&1; then
    echo "Docker Compose v2 is required to generate VAPID keys." >&2
    exit 1
  fi
  output=$(cd "$SCRIPT_DIR" && docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" run --rm api vapid-keygen)
  public_key=$(printf '%s\n' "$output" | sed -n 's/^WEB_PUSH_VAPID_PUBLIC_KEY=//p' | tail -n 1)
  private_key=$(printf '%s\n' "$output" | sed -n 's/^WEB_PUSH_VAPID_PRIVATE_KEY=//p' | tail -n 1)
  if [ -z "$public_key" ] || [ -z "$private_key" ]; then
    echo "Could not generate Web Push VAPID keys." >&2
    exit 1
  fi
  write_env WEB_PUSH_VAPID_PUBLIC_KEY "$public_key"
  write_env WEB_PUSH_VAPID_PRIVATE_KEY "$private_key"
}

configure_web_push() {
  if [ "$ENABLE_WEB_PUSH" = "true" ]; then
    if [ -z "$(read_env WEB_PUSH_VAPID_PUBLIC_KEY)" ] || [ -z "$(read_env WEB_PUSH_VAPID_PRIVATE_KEY)" ]; then
      generate_vapid
    fi
    subject=$(read_env WEB_PUSH_VAPID_SUBJECT)
    if [ -z "$subject" ]; then
      email=$(read_env LETSENCRYPT_EMAIL)
      if [ -z "$email" ]; then
        email="admin@example.com"
      fi
      subject="mailto:$email"
    fi
    write_env WEB_PUSH_ENABLED "true"
    write_env WEB_PUSH_VAPID_SUBJECT "$subject"
  elif [ "$ENABLE_WEB_PUSH" = "false" ]; then
    write_env WEB_PUSH_ENABLED "false"
  fi
}

require_command openssl

case "$PUSH_MODE" in
  "") repair_existing_relay_server ;;
  none) configure_none ;;
  direct-fcm) configure_direct_fcm ;;
  relay-client) configure_relay_client ;;
  relay-server) configure_relay_server ;;
  *) echo "Unsupported --push-mode: $PUSH_MODE" >&2; usage; exit 1 ;;
esac

if [ -n "$ENABLE_WEB_PUSH" ]; then
  configure_web_push
fi

if [ "$RESTART" -eq 1 ]; then
  cd "$SCRIPT_DIR"
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" config >/dev/null
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --no-deps --force-recreate api worker web
  if [ "$(read_env PUSH_RELAY_SERVER_ENABLED)" = "true" ]; then
    docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" --profile push-relay build push-relay
    docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" --profile push-relay up -d --no-deps --force-recreate push-relay
  fi
fi

echo "Notification environment updated."
