#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"

compose() {
  case "${OFFSITE_BACKUP_INCLUDE_INSTANCE_ENV:-false}" in
    true)
      docker compose --env-file .env -f compose.yml -f compose.include-instance-env.yml "$@"
      ;;
    false|"")
      docker compose --env-file .env -f compose.yml "$@"
      ;;
    *)
      echo "OFFSITE_BACKUP_INCLUDE_INSTANCE_ENV must be true or false" >&2
      return 2
      ;;
  esac
}

run_backup_cli() {
  compose --profile backup run --rm offsite-backup "$@"
}

usage() {
  cat <<'EOF'
Usage:
  ./backup.sh plan
  ./backup.sh init
  ./backup.sh backup [--dry-run] [--maintenance]
  ./backup.sh list [--json]
  ./backup.sh verify [SNAPSHOT_ID]
  ./backup.sh prune [--dry-run]
  ./backup.sh schedule-start|schedule-stop|schedule-logs

The normal stack and quickstart do not start off-site backup. Set
OFFSITE_BACKUP_ENABLED=true and OFFSITE_S3_*/OFFSITE_RESTIC_* in the dedicated
offsite-backup.env file first (copy offsite-backup.env.example if needed).
To include .env in the encrypted snapshot, explicitly export
OFFSITE_BACKUP_INCLUDE_INSTANCE_ENV=true for that command; it is never mounted
by the default Compose profile and restore never applies it.
Use --maintenance when the database and file storage must be captured without
application writes between their snapshots.
EOF
}

running_services=""
restore_stopped_services() {
  trap - EXIT INT TERM
  if [ -n "$running_services" ]; then
    # Service names are selected from a fixed allowlist below, not user input.
    compose up -d $running_services
  fi
}

maintenance_stop_writers() {
  current=$(compose ps --services --status running)
  running_services=""
  for service in api worker; do
    if printf '%s\n' "$current" | grep -qx "$service"; then
      running_services="$running_services $service"
    fi
  done
  if [ -n "$running_services" ]; then
    compose stop $running_services
    trap restore_stopped_services EXIT INT TERM
  fi
}

command=${1:-}
case "$command" in
  plan|init)
    shift
    [ "$#" -eq 0 ] || { usage >&2; exit 2; }
    run_backup_cli "$command"
    ;;
  backup)
    shift
    dry_run=false
    maintenance=false
    while [ "$#" -gt 0 ]; do
      case "$1" in
        --dry-run) dry_run=true ;;
        --maintenance) maintenance=true ;;
        *) usage >&2; exit 2 ;;
      esac
      shift
    done
    if [ "$maintenance" = true ] && [ "$dry_run" = false ]; then
      maintenance_stop_writers
    fi
    if [ "$dry_run" = true ]; then
      run_backup_cli backup --dry-run
    else
      run_backup_cli backup
    fi
    if [ "$maintenance" = true ]; then
      restore_stopped_services
    fi
    ;;
  list)
    shift
    case "${1:-}" in
      "") run_backup_cli list ;;
      --json) [ "$#" -eq 1 ] || { usage >&2; exit 2; }; run_backup_cli list --json ;;
      *) usage >&2; exit 2 ;;
    esac
    ;;
  verify)
    shift
    case "$#" in
      0) run_backup_cli verify ;;
      1) run_backup_cli verify "$1" ;;
      *) usage >&2; exit 2 ;;
    esac
    ;;
  prune)
    shift
    case "${1:-}" in
      "") run_backup_cli prune ;;
      --dry-run) [ "$#" -eq 1 ] || { usage >&2; exit 2; }; run_backup_cli prune --dry-run ;;
      *) usage >&2; exit 2 ;;
    esac
    ;;
  schedule-start)
    [ "$#" -eq 1 ] || { usage >&2; exit 2; }
    compose --profile backup up -d offsite-backup
    ;;
  schedule-stop)
    [ "$#" -eq 1 ] || { usage >&2; exit 2; }
    compose --profile backup stop offsite-backup
    ;;
  schedule-logs)
    [ "$#" -eq 1 ] || { usage >&2; exit 2; }
    compose --profile backup logs -f offsite-backup
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
