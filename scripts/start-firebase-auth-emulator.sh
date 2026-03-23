#!/usr/bin/env bash

set -euo pipefail

FIREBASE_PROJECT_ID="${FIREBASE_PROJECT_ID:-smallworld-a0223}"
FIREBASE_EMULATOR_CONFIG="${FIREBASE_EMULATOR_CONFIG:-/tmp/firebase-emulators.json}"
FIREBASE_EMULATOR_PORT="${FIREBASE_EMULATOR_PORT:-9099}"
FIREBASE_EMULATOR_HOME="${FIREBASE_EMULATOR_HOME:-/tmp/firebase-home}"
FIREBASE_EMULATOR_CACHE="${FIREBASE_EMULATOR_CACHE:-/tmp/firebase-cache}"

mkdir -p "${FIREBASE_EMULATOR_HOME}" "${FIREBASE_EMULATOR_CACHE}"

cat > "${FIREBASE_EMULATOR_CONFIG}" <<JSON
{
  "emulators": {
    "auth": {
      "port": ${FIREBASE_EMULATOR_PORT}
    }
  }
}
JSON

export HOME="${FIREBASE_EMULATOR_HOME}"
export XDG_CACHE_HOME="${FIREBASE_EMULATOR_CACHE}"

exec firebase emulators:start \
  --config "${FIREBASE_EMULATOR_CONFIG}" \
  --project "${FIREBASE_PROJECT_ID}" \
  --only auth
