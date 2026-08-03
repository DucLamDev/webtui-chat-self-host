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

backup_cli() {
  compose --profile backup run --rm offsite-backup "$@"
}

restore_cli() {
  compose --profile restore run --rm offsite-restore "$@"
}

usage() {
  cat <<'EOF'
Usage:
  ./restore.sh SNAPSHOT_ID --apply --confirm RESTORE:SNAPSHOT_ID [--keep-staging]

Safety properties:
  - SNAPSHOT_ID must be explicit hexadecimal; "latest" is rejected.
  - The target is downloaded to staging and fully verified before maintenance.
  - A fresh safety snapshot is made with application writers stopped.
  - PostgreSQL and storage require separate generated internal confirmations.
  - Local storage retains the old tree until migrations and health checks pass.
  - .env/compose/Caddy files in the encrypted bundle are never auto-applied.
EOF
}

snapshot=${1:-}
[ -n "$snapshot" ] || { usage >&2; exit 2; }
shift
snapshot=$(printf '%s' "$snapshot" | tr 'A-F' 'a-f')
printf '%s' "$snapshot" | grep -Eq '^[0-9a-f]{8,64}$' || {
  echo "SNAPSHOT_ID must contain 8 to 64 hexadecimal characters; latest is not accepted." >&2
  exit 2
}

apply=false
confirmation=""
keep_staging=false
while [ "$#" -gt 0 ]; do
  case "$1" in
    --apply)
      apply=true
      ;;
    --confirm)
      [ "$#" -ge 2 ] || { usage >&2; exit 2; }
      confirmation=$2
      shift
      ;;
    --keep-staging)
      keep_staging=true
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
  shift
done

[ "$apply" = true ] || {
  echo "Restore is destructive. Re-run with --apply after reviewing the runbook." >&2
  exit 2
}
expected_confirmation="RESTORE:$snapshot"
[ "$confirmation" = "$expected_confirmation" ] || {
  echo "Confirmation mismatch. Expected exactly: $expected_confirmation" >&2
  exit 2
}

timestamp=$(date -u +%Y%m%dT%H%M%SZ)
target_stage="target-$snapshot-$timestamp"
safety_stage=""
safety_snapshot=""
running_services=""
scheduler_was_running=false
database_touched=false
storage_touched=false
restore_succeeded=false
provider=""

service_is_running() {
  compose ps --services --status running | grep -qx "$1"
}

remember_and_stop_writers() {
  current=$(compose ps --services --status running)
  running_services=""
  for service in api worker web admin caddy; do
    if printf '%s\n' "$current" | grep -qx "$service"; then
      running_services="$running_services $service"
    fi
  done
  if printf '%s\n' "$current" | grep -qx offsite-backup; then
    scheduler_was_running=true
    compose --profile backup stop offsite-backup
  fi
  if [ -n "$running_services" ]; then
    # The values come from the fixed service allowlist above.
    compose stop $running_services
  fi
}

restart_previous_services() {
  if [ -n "$running_services" ]; then
    compose up -d $running_services
  fi
  if [ "$scheduler_was_running" = true ]; then
    compose --profile backup up -d offsite-backup
  fi
}

wait_for_api() {
  if ! printf '%s\n' "$running_services" | grep -qw api; then
    return 0
  fi
  attempt=0
  while [ "$attempt" -lt 30 ]; do
    if compose exec -T api wget -qO- http://localhost:8080/ready >/dev/null 2>&1; then
      return 0
    fi
    attempt=$((attempt + 1))
    sleep 2
  done
  echo "API did not become ready after restore." >&2
  return 1
}

internal_confirmation() {
  backup_cli confirmation "$1" "$2" | tail -n 1
}

remove_stage() {
  stage_name=$1
  token=$(internal_confirmation "$stage_name" remove-stage)
  backup_cli remove-stage "$stage_name" --confirmation "$token"
}

rollback() {
  status=$?
  trap - EXIT INT TERM
  [ "$restore_succeeded" = true ] && exit "$status"
  echo "Restore failed; attempting rollback to safety snapshot $safety_snapshot." >&2
  compose stop api worker web admin caddy >/dev/null 2>&1 || true
  rollback_ok=true

  if [ "$storage_touched" = true ] && [ -n "$safety_stage" ]; then
    if [ "$provider" = local ]; then
      if token=$(internal_confirmation "$target_stage" rollback-storage); then
        restore_cli rollback-storage "$target_stage" --confirmation "$token" || rollback_ok=false
      else
        rollback_ok=false
      fi
    else
      if token=$(internal_confirmation "$safety_stage" restore-storage); then
        restore_cli apply-storage "$safety_stage" --confirmation "$token" || rollback_ok=false
      else
        rollback_ok=false
      fi
    fi
  fi

  if [ "$database_touched" = true ] && [ -n "$safety_stage" ]; then
    if token=$(internal_confirmation "$safety_stage" restore-db); then
      restore_cli restore-db "$safety_stage" --confirmation "$token" || rollback_ok=false
    else
      rollback_ok=false
    fi
  fi

  if [ "$rollback_ok" = true ]; then
    compose run --rm migrate || rollback_ok=false
  fi
  if [ "$rollback_ok" = true ]; then
    restart_previous_services || rollback_ok=false
  fi
  if [ "$rollback_ok" = true ]; then
    echo "Rollback completed. The failed target and safety stages were retained for investigation." >&2
  else
    echo "AUTOMATIC ROLLBACK INCOMPLETE. Services remain stopped; follow the recovery runbook." >&2
  fi
  exit "$status"
}

echo "Staging and verifying target snapshot $snapshot before maintenance..."
backup_cli stage "$snapshot" --stage-name "$target_stage"
provider=$(backup_cli storage-provider | tail -n 1)
case "$provider" in
  local|minio|s3) ;;
  *) echo "Unsupported primary storage provider: $provider" >&2; exit 1 ;;
esac

trap rollback EXIT INT TERM
remember_and_stop_writers

echo "Application writers stopped. Creating a fresh off-site safety snapshot..."
safety_snapshot=$(backup_cli backup | tail -n 1)
printf '%s' "$safety_snapshot" | grep -Eq '^[0-9a-f]{8,64}$' || {
  echo "Safety backup did not return a valid snapshot ID." >&2
  exit 1
}
safety_stage="safety-$safety_snapshot-$timestamp"
backup_cli stage "$safety_snapshot" --stage-name "$safety_stage"

echo "Replacing PostgreSQL from verified target snapshot..."
database_touched=true
database_confirmation=$(internal_confirmation "$target_stage" restore-db)
restore_cli restore-db "$target_stage" --confirmation "$database_confirmation"

echo "Replacing primary file/object storage from verified target snapshot..."
storage_confirmation=$(internal_confirmation "$target_stage" restore-storage)
if [ "$provider" != local ]; then
  # rclone may have uploaded/replaced objects before a remote error; make sure
  # rollback re-syncs the maintenance-mode safety snapshot in that case.
  storage_touched=true
fi
restore_cli apply-storage "$target_stage" --confirmation "$storage_confirmation"
storage_touched=true

echo "Applying forward-only migrations for the currently installed application version..."
compose run --rm migrate
restart_previous_services
wait_for_api

restore_succeeded=true
trap - EXIT INT TERM

if [ "$provider" = local ]; then
  finalize_confirmation=$(internal_confirmation "$target_stage" finalize-storage)
  if ! restore_cli finalize-storage "$target_stage" --confirmation "$finalize_confirmation"; then
    echo "Restore is healthy, but old local storage cleanup failed; retain staging and finalize manually." >&2
    keep_staging=true
  fi
fi

if [ "$keep_staging" = false ]; then
  remove_stage "$target_stage"
  remove_stage "$safety_stage"
fi

echo "Restore completed from snapshot $snapshot. Safety snapshot: $safety_snapshot"
echo "Bundled configuration was not applied; compare it manually if rebuilding another host."
