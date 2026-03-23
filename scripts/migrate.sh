#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./scripts/migrate.sh up
  ./scripts/migrate.sh down 1
  ./scripts/migrate.sh version
  ./scripts/migrate.sh force <version>
  ./scripts/migrate.sh goto <version>
  ./scripts/migrate.sh create <name>
  ./scripts/migrate.sh reset-local
  ./scripts/migrate.sh --yes reset-local

Environment:
  DATABASE_URL is required for all commands except create and reset-local.

Notes:
  down rolls back one or more migration steps.
  reset-local recreates the local docker-compose Postgres/Redis volumes, then runs up.
  Use --yes to skip confirmation prompts for destructive commands.
EOF
}

assume_yes=false
if [[ "${1:-}" == "--yes" || "${1:-}" == "-y" ]]; then
  assume_yes=true
  shift
fi

if [[ $# -lt 1 ]]; then
  usage
  exit 1
fi

command_name="$1"
shift

if [[ "${command_name}" == "drop" ]]; then
  cat <<'EOF'
The raw 'migrate drop' command is disabled for this project.

Reason:
  This database uses the PostGIS extension, and 'migrate drop' attempts to drop
  extension-owned tables such as spatial_ref_sys, which fails.

Use one of these instead:
  1. Controlled rollback:
     ./scripts/migrate.sh down 1

  2. Local full reset for docker-compose development:
     docker compose down -v
     docker compose up -d
     export DATABASE_URL='postgres://postgres:postgres@localhost:5432/smallworld?sslmode=disable'
     ./scripts/migrate.sh up

  3. Local helper:
     ./scripts/migrate.sh reset-local
EOF
  exit 1
fi

default_local_database_url='postgres://postgres:postgres@localhost:5432/smallworld?sslmode=disable'

confirm_destructive() {
  local action_name="${1}"
  local prompt
  case "${action_name}" in
    down)
      prompt="This will roll back one or more migrations. Continue? [y/N] "
      ;;
    force)
      prompt="This will force the migration version without running migrations. Continue? [y/N] "
      ;;
    reset-local)
      prompt="This will destroy local Postgres and Redis data by running 'docker compose down -v'. Continue? [y/N] "
      ;;
    *)
      return
      ;;
  esac

  if [[ "${assume_yes}" == "true" ]]; then
    return
  fi

  if [[ ! -t 0 ]]; then
    echo "Refusing to run ${action_name} without interactive confirmation. Re-run with --yes if intentional."
    exit 1
  fi

  read -r -p "${prompt}" reply
  case "${reply}" in
    y|Y|yes|YES|Yes)
      ;;
    *)
      echo "Aborted."
      exit 1
      ;;
  esac
}

run_local() {
  local migrate_command="$1"
  shift
  migrate -path db/migrations -database "${DATABASE_URL}" "${migrate_command}" "$@"
}

run_docker() {
  local migrate_command="$1"
  shift
  local image="migrate/migrate"
  local migrations_dir
  local -a docker_args
  migrations_dir="$(pwd)/db/migrations"
  docker_args=(run --rm)

  if [[ -t 0 ]]; then
    docker_args+=(-i)
  fi
  if [[ -t 1 ]]; then
    docker_args+=(-t)
  fi

  if [[ "${migrate_command}" == "create" ]]; then
    docker "${docker_args[@]}" \
      -v "${migrations_dir}:/migrations" \
      "${image}" \
      create -ext sql -dir /migrations -seq "$@"
    return
  fi

  docker "${docker_args[@]}" \
    --network host \
    -v "${migrations_dir}:/migrations" \
    "${image}" \
    -path=/migrations -database "${DATABASE_URL}" "${migrate_command}" "$@"
}

dispatch_migrate() {
  local migrate_command="$1"
  shift

  if [[ "${migrate_command}" != "create" && -z "${DATABASE_URL:-}" ]]; then
    echo "DATABASE_URL is required"
    exit 1
  fi

  if command -v migrate >/dev/null 2>&1; then
    run_local "${migrate_command}" "$@"
    return
  fi

  if ! command -v docker >/dev/null 2>&1; then
    echo "Neither migrate nor docker is available"
    exit 1
  fi

  run_docker "${migrate_command}" "$@"
}

reset_local() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "docker is required for reset-local"
    exit 1
  fi

  confirm_destructive reset-local

  docker compose down -v
  docker compose up -d

  if [[ -z "${DATABASE_URL:-}" ]]; then
    export DATABASE_URL="${default_local_database_url}"
  fi
  sleep 5
  
  dispatch_migrate up

  echo "Local database reset complete."
}

case "${command_name}" in
  reset-local)
    reset_local "$@"
    ;;
  *)
    confirm_destructive "${command_name}"
    dispatch_migrate "${command_name}" "$@"
    ;;
esac
