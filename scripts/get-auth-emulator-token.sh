#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: ./scripts/get-auth-emulator-token.sh <email> <password>"
  exit 1
fi

if [[ -z "${FIREBASE_AUTH_EMULATOR_HOST:-}" ]]; then
  echo "FIREBASE_AUTH_EMULATOR_HOST is required"
  echo "Example: export FIREBASE_AUTH_EMULATOR_HOST=127.0.0.1:9099"
  exit 1
fi

email="$1"
password="$2"
api_key="local-emulator-key"
base_url="http://${FIREBASE_AUTH_EMULATOR_HOST}/identitytoolkit.googleapis.com/v1"
payload="$(printf '{"email":"%s","password":"%s","returnSecureToken":true}' "${email}" "${password}")"

extract_id_token() {
  sed -n 's/.*"idToken":"\([^"]*\)".*/\1/p'
}

response="$(curl -sS \
  -H 'Content-Type: application/json' \
  -d "${payload}" \
  "${base_url}/accounts:signInWithPassword?key=${api_key}" || true)"

id_token="$(printf '%s' "${response}" | extract_id_token)"
if [[ -n "${id_token}" ]]; then
  printf '%s\n' "${id_token}"
  exit 0
fi

response="$(curl -sS \
  -H 'Content-Type: application/json' \
  -d "${payload}" \
  "${base_url}/accounts:signUp?key=${api_key}")"

id_token="$(printf '%s' "${response}" | extract_id_token)"
if [[ -z "${id_token}" ]]; then
  echo "Failed to obtain emulator ID token"
  echo "${response}"
  exit 1
fi

printf '%s\n' "${id_token}"
