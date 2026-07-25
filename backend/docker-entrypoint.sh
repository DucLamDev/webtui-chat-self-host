#!/usr/bin/env sh
set -eu

case "${1:-api}" in
  api)
    exec webtui-api
    ;;
  worker)
    exec webtui-worker
    ;;
  migrate)
    shift
    exec webtui-migrate "$@"
    ;;
  *)
    exec "$@"
    ;;
esac
