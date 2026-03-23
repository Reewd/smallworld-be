#!/usr/bin/env bash

set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
FIREBASE_AUTH_EMULATOR_HOST="${FIREBASE_AUTH_EMULATOR_HOST:-127.0.0.1:9099}"
SMOKE_EMAIL="${SMOKE_EMAIL:-android-prototype@example.com}"
SMOKE_PASSWORD="${SMOKE_PASSWORD:-password123}"
SMOKE_DISPLAY_NAME="${SMOKE_DISPLAY_NAME:-Android Prototype}"
SMOKE_VERIFIED_GENDER="${SMOKE_VERIFIED_GENDER:-female}"
SMOKE_VEHICLE_MAKE="${SMOKE_VEHICLE_MAKE:-Toyota}"
SMOKE_VEHICLE_MODEL="${SMOKE_VEHICLE_MODEL:-Yaris}"
SMOKE_VEHICLE_COLOR="${SMOKE_VEHICLE_COLOR:-Blue}"
SMOKE_LICENSE_PLATE="${SMOKE_LICENSE_PLATE:-DEV-001}"
SMOKE_VEHICLE_CAPACITY="${SMOKE_VEHICLE_CAPACITY:-3}"
SMOKE_IDEMPOTENCY_KEY="${SMOKE_IDEMPOTENCY_KEY:-android-prototype-route-1}"
SMOKE_ORIGIN_LAT="${SMOKE_ORIGIN_LAT:-45.4642}"
SMOKE_ORIGIN_LNG="${SMOKE_ORIGIN_LNG:-9.1900}"
SMOKE_DESTINATION_LAT="${SMOKE_DESTINATION_LAT:-45.4985}"
SMOKE_DESTINATION_LNG="${SMOKE_DESTINATION_LNG:-9.2142}"
SMOKE_MAX_DRIVER_PICKUP_DETOUR_METERS="${SMOKE_MAX_DRIVER_PICKUP_DETOUR_METERS:-1200}"

cleanup_files=()
HTTP_JSON_CODE=""

cleanup() {
  for file in "${cleanup_files[@]:-}"; do
    [[ -f "${file}" ]] && rm -f "${file}"
  done
}
trap cleanup EXIT

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Required command not found: $1"
    exit 1
  fi
}

json_field() {
  local file="$1"
  local expr="$2"
  python3 - "$file" "$expr" <<'PY'
import json
import sys

path, expr = sys.argv[1], sys.argv[2]
with open(path, "r", encoding="utf-8") as fh:
    data = json.load(fh)

parts = [p for p in expr.split(".") if p]
value = data
for part in parts:
    if isinstance(value, list):
        value = value[int(part)]
    else:
        value = value.get(part)

if value is None:
    sys.exit(1)
if isinstance(value, (dict, list)):
    print(json.dumps(value))
else:
    print(value)
PY
}

json_has_field() {
  local file="$1"
  local expr="$2"
  python3 - "$file" "$expr" <<'PY'
import json
import sys

path, expr = sys.argv[1], sys.argv[2]
with open(path, "r", encoding="utf-8") as fh:
    data = json.load(fh)

value = data
for part in [p for p in expr.split(".") if p]:
    if isinstance(value, list):
        try:
            value = value[int(part)]
        except Exception:
            sys.exit(1)
    elif isinstance(value, dict) and part in value:
        value = value[part]
    else:
        sys.exit(1)
sys.exit(0)
PY
}

json_assert_eq() {
  local file="$1"
  local expr="$2"
  local expected="$3"
  local actual
  actual="$(json_field "$file" "$expr")" || {
    echo "Expected field '$expr' to exist in $file"
    cat "$file"
    exit 1
  }
  if [[ "$actual" != "$expected" ]]; then
    echo "Expected $expr=$expected but got $actual"
    cat "$file"
    exit 1
  fi
}

http_json() {
  local method="$1"
  local path="$2"
  local output_file="$3"
  local data="${4:-}"
  local auth_token="${5:-}"
  cleanup_files+=("$output_file")

  local -a curl_args=(
    -sS
    -o "$output_file"
    -w "%{http_code}"
    -X "$method"
  )

  if [[ -n "$auth_token" ]]; then
    curl_args+=(-H "Authorization: Bearer ${auth_token}")
  fi
  if [[ -n "$data" ]]; then
    curl_args+=(-H "Content-Type: application/json" -d "$data")
  fi
  curl_args+=("${API_BASE_URL}${path}")

  HTTP_JSON_CODE="$(curl "${curl_args[@]}")"
}

print_step() {
  echo "== $1 =="
}

require_command curl
require_command go
require_command python3

export FIREBASE_AUTH_EMULATOR_HOST

print_step "mint emulator auth token"
AUTH_TOKEN="$(./scripts/get-auth-emulator-token.sh "${SMOKE_EMAIL}" "${SMOKE_PASSWORD}")"
echo "token acquired"
echo

health_file="$(mktemp)"
http_json GET /healthz "$health_file"
health_code="$HTTP_JSON_CODE"
print_step "health"
echo "$(cat "$health_file")"
echo
if [[ "$health_code" != "200" ]]; then
  echo "Expected 200 from /healthz, got $health_code"
  exit 1
fi

before_auth_file="$(mktemp)"
http_json GET /v1/auth/me "$before_auth_file" "" "$AUTH_TOKEN"
before_auth_code="$HTTP_JSON_CODE"
print_step "auth/me before bootstrap"
cat "$before_auth_file"
echo
if [[ "$before_auth_code" != "200" ]]; then
  echo "Expected 200 from /v1/auth/me before bootstrap, got $before_auth_code"
  exit 1
fi
json_assert_eq "$before_auth_file" "auth.provider" "firebase"
json_has_field "$before_auth_file" "auth.subject" || {
  echo "Expected auth.subject in /v1/auth/me response"
  cat "$before_auth_file"
  exit 1
}
if json_has_field "$before_auth_file" "user.id"; then
  echo "Expected no backend user before bootstrap"
  cat "$before_auth_file"
  exit 1
fi

bootstrap_body="$(cat <<JSON
{
  "display_name": "${SMOKE_DISPLAY_NAME}",
  "verified_gender": "${SMOKE_VERIFIED_GENDER}",
  "vehicle": {
    "make": "${SMOKE_VEHICLE_MAKE}",
    "model": "${SMOKE_VEHICLE_MODEL}",
    "color": "${SMOKE_VEHICLE_COLOR}",
    "license_plate": "${SMOKE_LICENSE_PLATE}",
    "capacity": ${SMOKE_VEHICLE_CAPACITY}
  }
}
JSON
)"

bootstrap_file="$(mktemp)"
http_json POST /v1/dev/me/bootstrap "$bootstrap_file" "$bootstrap_body" "$AUTH_TOKEN"
bootstrap_code="$HTTP_JSON_CODE"
print_step "dev bootstrap"
cat "$bootstrap_file"
echo
if [[ "$bootstrap_code" != "200" ]]; then
  echo "Expected 200 from /v1/dev/me/bootstrap, got $bootstrap_code"
  exit 1
fi
json_assert_eq "$bootstrap_file" "verification.provider" "dev_emulator"
json_assert_eq "$bootstrap_file" "verification.status" "verified"
json_assert_eq "$bootstrap_file" "verification.verified_gender" "${SMOKE_VERIFIED_GENDER}"
json_has_field "$bootstrap_file" "user.id" || {
  echo "Expected bootstrapped user.id"
  cat "$bootstrap_file"
  exit 1
}
json_has_field "$bootstrap_file" "vehicle.id" || {
  echo "Expected bootstrapped vehicle.id"
  cat "$bootstrap_file"
  exit 1
}

after_auth_file="$(mktemp)"
http_json GET /v1/auth/me "$after_auth_file" "" "$AUTH_TOKEN"
after_auth_code="$HTTP_JSON_CODE"
print_step "auth/me after bootstrap"
cat "$after_auth_file"
echo
if [[ "$after_auth_code" != "200" ]]; then
  echo "Expected 200 from /v1/auth/me after bootstrap, got $after_auth_code"
  exit 1
fi
json_has_field "$after_auth_file" "user.id" || {
  echo "Expected backend user after bootstrap"
  cat "$after_auth_file"
  exit 1
}

print_step "current-state endpoints before activity"
for path in /v1/me/driver-session /v1/me/trip-demand /v1/me/ride-offers /v1/me/bookings; do
  body_file="$(mktemp)"
  http_json GET "$path" "$body_file" "" "$AUTH_TOKEN"
  code="$HTTP_JSON_CODE"
  echo "-- ${path} (${code})"
  cat "$body_file"
  echo
  if [[ "$code" != "404" ]]; then
    echo "Expected 404 from ${path} before activity, got ${code}"
    exit 1
  fi
done

print_step "websocket without token is rejected"
ws_unauth_body="$(mktemp)"
ws_unauth_headers="$(mktemp)"
cleanup_files+=("$ws_unauth_body" "$ws_unauth_headers")
curl -sS -D "$ws_unauth_headers" -o "$ws_unauth_body" "${API_BASE_URL}/v1/ws" >/dev/null || true
if ! grep -q "401" "$ws_unauth_headers"; then
  echo "Expected unauthenticated websocket HTTP request to be rejected with 401"
  cat "$ws_unauth_headers"
  cat "$ws_unauth_body"
  exit 1
fi

print_step "websocket with auth connects"
go run ./scripts/ws-smoke.go -url "${API_BASE_URL}/v1/ws" -token "${AUTH_TOKEN}"
echo

vehicles_file="$(mktemp)"
http_json GET /v1/vehicles "$vehicles_file" "" "$AUTH_TOKEN"
vehicles_code="$HTTP_JSON_CODE"
print_step "vehicles"
cat "$vehicles_file"
echo
if [[ "$vehicles_code" != "200" ]]; then
  echo "Expected 200 from /v1/vehicles, got $vehicles_code"
  exit 1
fi
VEHICLE_ID="$(json_field "$vehicles_file" "0.id")" || {
  echo "Expected at least one vehicle after bootstrap"
  cat "$vehicles_file"
  exit 1
}

session_body="$(cat <<JSON
{
  "vehicle_id": "${VEHICLE_ID}",
  "origin": {"lat": ${SMOKE_ORIGIN_LAT}, "lng": ${SMOKE_ORIGIN_LNG}},
  "destination": {"lat": ${SMOKE_DESTINATION_LAT}, "lng": ${SMOKE_DESTINATION_LNG}},
  "current_location": {"lat": ${SMOKE_ORIGIN_LAT}, "lng": ${SMOKE_ORIGIN_LNG}},
  "max_driver_pickup_detour_meters": ${SMOKE_MAX_DRIVER_PICKUP_DETOUR_METERS},
  "idempotency_key": "${SMOKE_IDEMPOTENCY_KEY}"
}
JSON
)"

session_file="$(mktemp)"
http_json POST /v1/driver-sessions "$session_file" "$session_body" "$AUTH_TOKEN"
session_code="$HTTP_JSON_CODE"
print_step "create driver session"
cat "$session_file"
echo
if [[ "$session_code" != "201" && "$session_code" != "200" ]]; then
  echo "Expected 201 or 200 from /v1/driver-sessions, got $session_code"
  exit 1
fi
json_assert_eq "$session_file" "state" "active"
json_has_field "$session_file" "route_distance_meters" || {
  echo "Expected route_distance_meters on driver session"
  cat "$session_file"
  exit 1
}
json_has_field "$session_file" "route_duration_seconds" || {
  echo "Expected route_duration_seconds on driver session"
  cat "$session_file"
  exit 1
}
json_has_field "$session_file" "route_polyline" || {
  echo "Expected route_polyline on driver session"
  cat "$session_file"
  exit 1
}

SESSION_ID="$(json_field "$session_file" "id")"
ROUTE_POLYLINE="$(json_field "$session_file" "route_polyline")"
if [[ -z "$ROUTE_POLYLINE" ]]; then
  echo "Expected non-empty route_polyline"
  cat "$session_file"
  exit 1
fi

current_session_file="$(mktemp)"
http_json GET /v1/me/driver-session "$current_session_file" "" "$AUTH_TOKEN"
current_session_code="$HTTP_JSON_CODE"
print_step "current driver session"
cat "$current_session_file"
echo
if [[ "$current_session_code" != "200" ]]; then
  echo "Expected 200 from /v1/me/driver-session after session creation, got $current_session_code"
  exit 1
fi
json_assert_eq "$current_session_file" "id" "$SESSION_ID"

heartbeat_before="$(json_field "$current_session_file" "last_heartbeat_at")"
heartbeat_body="$(cat <<JSON
{
  "current_location": {"lat": ${SMOKE_ORIGIN_LAT}, "lng": ${SMOKE_ORIGIN_LNG}}
}
JSON
)"
heartbeat_file="$(mktemp)"
http_json POST "/v1/driver-sessions/${SESSION_ID}/heartbeat" "$heartbeat_file" "$heartbeat_body" "$AUTH_TOKEN"
heartbeat_code="$HTTP_JSON_CODE"
print_step "heartbeat"
cat "$heartbeat_file"
echo
if [[ "$heartbeat_code" != "200" ]]; then
  echo "Expected 200 from heartbeat, got $heartbeat_code"
  exit 1
fi

after_heartbeat_file="$(mktemp)"
http_json GET /v1/me/driver-session "$after_heartbeat_file" "" "$AUTH_TOKEN"
after_heartbeat_code="$HTTP_JSON_CODE"
print_step "current driver session after heartbeat"
cat "$after_heartbeat_file"
echo
if [[ "$after_heartbeat_code" != "200" ]]; then
  echo "Expected 200 from /v1/me/driver-session after heartbeat, got $after_heartbeat_code"
  exit 1
fi
heartbeat_after="$(json_field "$after_heartbeat_file" "last_heartbeat_at")"
if [[ "$heartbeat_before" == "$heartbeat_after" ]]; then
  echo "Expected last_heartbeat_at to change after heartbeat"
  exit 1
fi

echo "Smoke test completed successfully."
