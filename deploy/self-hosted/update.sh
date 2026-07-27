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
if ! docker compose --env-file .env -f compose.yml run --rm migrate; then
  echo "" >&2
  echo "Update stopped before replacing containers because database migration failed." >&2
  echo "No newly built web/API image has been activated." >&2
  echo "If the error mentions BOT_AI_SECRET_KEY, set a strong key in deploy/self-hosted/.env, then run this update again." >&2
  echo "For an instance that has never stored Bot AI credentials, generate one with:" >&2
  echo "  BOT_KEY=\$(openssl rand -hex 48)" >&2
  echo "  sed -i \"s|^BOT_AI_SECRET_KEY=.*|BOT_AI_SECRET_KEY=\$BOT_KEY|\" deploy/self-hosted/.env" >&2
  echo "  unset BOT_KEY" >&2
  exit 1
fi
docker compose --env-file .env -f compose.yml up -d --no-deps --force-recreate api worker web admin
sh "$SCRIPT_DIR/check.sh"
