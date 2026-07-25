#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"
docker compose --env-file .env -f compose.yml ps
docker compose --env-file .env -f compose.yml exec -T api wget -qO- http://localhost:8080/ready
echo
