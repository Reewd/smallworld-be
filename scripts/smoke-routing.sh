#!/usr/bin/env bash

set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"

if [[ -z "${AUTH_TOKEN:-}" ]]; then
  echo "AUTH_TOKEN is required"
  echo "Provide a Firebase ID token, ideally from the Firebase Auth emulator during local development."
  exit 1
fi

if [[ -z "${VEHICLE_ID:-}" ]]; then
  echo "VEHICLE_ID is required"
  echo "Pass the backend vehicle ID that belongs to the authenticated user."
  exit 1
fi

echo "== auth/me =="
curl -sS \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  "${API_BASE_URL}/v1/auth/me"
echo
echo

echo "== vehicles =="
curl -sS \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  "${API_BASE_URL}/v1/vehicles"
echo
echo

echo "== create driver session =="
curl -sS \
  -X POST \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"vehicle_id\": \"${VEHICLE_ID}\",
    \"origin\": {\"lat\": 45.4642, \"lng\": 9.1900},
    \"destination\": {\"lat\": 45.4985, \"lng\": 9.2142},
    \"current_location\": {\"lat\": 45.4642, \"lng\": 9.1900},
    \"max_driver_pickup_detour_meters\": 1200,
    \"idempotency_key\": \"route-smoke-1\"
  }" \
  "${API_BASE_URL}/v1/driver-sessions"
echo
