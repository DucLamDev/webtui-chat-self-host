#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
BACKUP_DIR=${1:-}
CONFIRM=${2:-}
if [ -z "$BACKUP_DIR" ] || [ "$CONFIRM" != "--yes" ]; then
  echo "Usage: $0 /absolute/path/to/backup --yes" >&2
  exit 1
fi
BACKUP_DIR=$(CDPATH= cd -- "$BACKUP_DIR" && pwd)
for file in database.dump storage.tar.gz instance.env; do
  if [ ! -f "$BACKUP_DIR/$file" ]; then
    echo "Missing backup file: $BACKUP_DIR/$file" >&2
    exit 1
  fi
done

cd "$SCRIPT_DIR"
cp .env ".env.before-restore.$(date -u +%Y%m%dT%H%M%SZ)"
cp "$BACKUP_DIR/instance.env" .env
docker compose --env-file .env -f compose.yml stop api worker web admin caddy
docker compose --env-file .env -f compose.yml up -d postgres redis rabbitmq
docker compose --env-file .env -f compose.yml exec -T postgres sh -c \
  'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner' \
  < "$BACKUP_DIR/database.dump"
docker run --rm \
  -v vpsttt_chat_storage_data:/data \
  -v "$BACKUP_DIR:/backup:ro" \
  alpine:3.22 \
  sh -c 'rm -rf /data/* /data/.[!.]* /data/..?* 2>/dev/null || true; tar -C /data -xzf /backup/storage.tar.gz'
docker compose --env-file .env -f compose.yml run --rm migrate
docker compose --env-file .env -f compose.yml up -d

echo "Restore completed from: $BACKUP_DIR"
