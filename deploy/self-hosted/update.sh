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
# The customer VPS may only contain this self-host repository. Build the
# deployable services explicitly so an optional sibling portal source does not
# block updates to the chat application.
docker compose --env-file .env -f compose.yml build --pull api worker web admin
docker compose --env-file .env -f compose.yml run --rm migrate
docker compose --env-file .env -f compose.yml up -d --no-deps --force-recreate api worker web admin
sh "$SCRIPT_DIR/check.sh"
