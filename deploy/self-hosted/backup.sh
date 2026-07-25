#!/usr/bin/env sh
set -eu
umask 077

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
BACKUP_DIR="$SCRIPT_DIR/backups/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$BACKUP_DIR"
cd "$SCRIPT_DIR"

docker compose --env-file .env -f compose.yml exec -T postgres sh -c \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
  > "$BACKUP_DIR/database.dump"
docker run --rm \
  -v vpsttt_chat_storage_data:/data:ro \
  -v "$BACKUP_DIR:/backup" \
  alpine:3.22 \
  tar -C /data -czf /backup/storage.tar.gz .
cp .env "$BACKUP_DIR/instance.env"
cp Caddyfile compose.yml "$BACKUP_DIR/"

echo "Backup created: $BACKUP_DIR"
