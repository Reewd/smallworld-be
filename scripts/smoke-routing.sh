#!/usr/bin/env bash

set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
AUTH_TOKEN="${AUTH_TOKEN:-dev:driver_1}"
VEHICLE_ID="${VEHICLE_ID:-veh_1}"

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
