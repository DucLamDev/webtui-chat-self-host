#!/usr/bin/env sh
set -eu

case "${1:-api}" in
  api)
    exec webtui-api
    ;;
  worker)
    exec webtui-worker
    ;;
  push-relay)
    exec webtui-push-relay
    ;;
  vapid-keygen)
    exec webtui-vapid-keygen
    ;;
  migrate)
    shift
    exec webtui-migrate "$@"
    ;;
  *)
    exec "$@"
    ;;
esac
