#!/usr/bin/env bash

set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
AUTH_TOKEN="${AUTH_TOKEN:-dev:alice}"

echo "== health =="
curl -sS "${API_BASE_URL}/healthz"
echo
echo

echo "== auth/me before profile =="
curl -sS \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  "${API_BASE_URL}/v1/auth/me"
echo
echo

echo "== upsert profile =="
curl -sS \
  -X POST \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "display_name": "Andrea",
    "preferences": {
      "max_walk_to_pickup_meters": 400,
      "max_walk_from_dropoff_meters": 400,
      "max_driver_pickup_detour_meters": 1200
    }
  }' \
  "${API_BASE_URL}/v1/profile"
echo
echo

echo "== auth/me after profile =="
curl -sS \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  "${API_BASE_URL}/v1/auth/me"
echo
