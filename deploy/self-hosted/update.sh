#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)

sh "$SCRIPT_DIR/backup.sh"
cd "$REPO_DIR"
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
  echo "Tracked files have local changes. Commit or revert them before updating." >&2
  exit 1
fi
git pull --ff-only

cd "$SCRIPT_DIR"
docker compose --env-file .env -f compose.yml build --pull
docker compose --env-file .env -f compose.yml run --rm migrate
docker compose --env-file .env -f compose.yml up -d
sh "$SCRIPT_DIR/check.sh"
