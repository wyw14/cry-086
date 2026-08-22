#!/bin/sh
set -eu

if [ "${REPOSITORY_MODE:-memory}" = "postgres" ]; then
  attempts=0
  until /app/migrate /app/migrations/000001_initial.up.sql; do
    attempts=$((attempts + 1))
    if [ "$attempts" -ge 20 ]; then
      echo "database migration did not become ready" >&2
      exit 1
    fi
    sleep 2
  done
fi

exec /app/server
