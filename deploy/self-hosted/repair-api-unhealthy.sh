#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ENV_FILE="$SCRIPT_DIR/.env"
COMPOSE_FILE="$SCRIPT_DIR/compose.yml"

compose() {
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
}

read_env_value() {
  key=$1
  sed -n "s/^$key=//p" "$ENV_FILE" | tail -n 1 | tr -d '\r'
}

wait_for_service() {
  service=$1
  attempt=0
  until compose ps --status running --services | grep -qx "$service"; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 30 ]; then
      echo "Service $service did not reach running state." >&2
      compose ps >&2 || true
      exit 1
    fi
    sleep 2
  done
}

if [ ! -f "$ENV_FILE" ]; then
  echo "Missing $ENV_FILE. Run install.sh first." >&2
  exit 1
fi

cd "$SCRIPT_DIR"
compose config >/dev/null

echo "Ensuring core dependencies are running..."
compose up -d postgres redis rabbitmq
wait_for_service rabbitmq

echo "Repairing local storage permissions..."
compose run --rm --no-deps storage-permissions

RABBITMQ_USER=$(read_env_value RABBITMQ_USER)
RABBITMQ_PASSWORD=$(read_env_value RABBITMQ_PASSWORD)
if [ -n "$RABBITMQ_USER" ] && [ -n "$RABBITMQ_PASSWORD" ]; then
  echo "Reconciling RabbitMQ user from .env..."
  if compose exec -T rabbitmq rabbitmqctl list_users 2>/dev/null | awk '{print $1}' | grep -qx "$RABBITMQ_USER"; then
    compose exec -T rabbitmq rabbitmqctl change_password "$RABBITMQ_USER" "$RABBITMQ_PASSWORD"
  else
    compose exec -T rabbitmq rabbitmqctl add_user "$RABBITMQ_USER" "$RABBITMQ_PASSWORD"
  fi
  compose exec -T rabbitmq rabbitmqctl set_permissions -p / "$RABBITMQ_USER" ".*" ".*" ".*"
fi

echo "Running migrations..."
compose run --rm migrate

echo "Restarting API..."
compose up -d --force-recreate api

attempt=0
until ready_body=$(compose exec -T api wget -qO- http://localhost:8080/ready 2>&1); do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 45 ]; then
    echo "API is still not ready." >&2
    echo "$ready_body" >&2
    echo "" >&2
    compose ps >&2 || true
    compose logs --tail=200 api postgres redis rabbitmq >&2 || true
    exit 1
  fi
  sleep 2
done
printf '%s\n' "$ready_body"

echo "Restarting dependent services..."
compose up -d --force-recreate worker web admin caddy
compose ps

echo "API repair completed."
