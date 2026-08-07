#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ENV_FILE="$SCRIPT_DIR/.env"

if [ ! -f "$ENV_FILE" ]; then
  echo "Không tìm thấy $ENV_FILE. Hãy chạy install.sh trước." >&2
  exit 1
fi

compose() {
  docker compose --env-file "$ENV_FILE" -f "$SCRIPT_DIR/compose.yml" "$@"
}

command=${1:-list}
case "$command" in
  list)
    [ "$#" -eq 1 ] || { echo "Cách dùng: $0 list" >&2; exit 2; }
    compose exec -T api webtui-admin-account list
    ;;
  create)
    [ "$#" -le 3 ] || { echo "Cách dùng: $0 create [username] [email]" >&2; exit 2; }
    username=${2:-admin}
    domain=$(sed -n 's/^INSTANCE_DOMAIN=//p' "$ENV_FILE" | tail -n 1 | tr -d '\r')
    email=${3:-admin@$domain}
    compose exec -T api webtui-admin-account ensure \
      --username "$username" \
      --email "$email" \
      --display-name "Quản trị viên"
    ;;
  reset)
    [ "$#" -le 2 ] || { echo "Cách dùng: $0 reset [username]" >&2; exit 2; }
    username=${2:-admin}
    compose exec -T api webtui-admin-account reset --username "$username"
    ;;
  *)
    echo "Cách dùng:" >&2
    echo "  $0 list" >&2
    echo "  $0 create [username] [email]" >&2
    echo "  $0 reset [username]" >&2
    exit 2
    ;;
esac
