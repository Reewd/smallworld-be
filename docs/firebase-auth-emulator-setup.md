# Firebase Auth Emulator Setup

This guide covers the recommended local auth workflow for the Smallworld backend.

Use the emulator for normal local development. Do not rely on backend-only dev token bypasses.

For production and staging-style setup, use [firebase-auth-setup.md](./firebase-auth-setup.md).

## Backend Env Vars For Emulator Mode

Set these for the API server:

```bash
export FIREBASE_PROJECT_ID="smallworld-a0223"
export FIREBASE_AUTH_EMULATOR_HOST="127.0.0.1:9099"
```

In emulator mode:

- `FIREBASE_CREDENTIALS_FILE` is not required
- the backend verifies emulator-issued Firebase ID tokens
- `POST /v1/dev/me/bootstrap` is mounted

## Important Naming Warning

The Firebase CLI uses repo-root `firebase.json` as project config.

Do not store backend service-account credentials at the repo root under that filename. Prefer:

- `.secrets/firebase-service-account.json`

## Starting The Auth Emulator

The simplest repo-local path is:

```bash
./scripts/start-firebase-auth-emulator.sh
```

That helper uses a temporary emulator config and avoids collisions with backend credential filenames.

If you prefer the Firebase CLI directly:

```bash
firebase login
firebase init emulators
firebase emulators:start --only auth
```

The default auth emulator port is usually `9099`.

## Local Development Flow

1. Start Postgres and Redis.
2. Apply migrations.
3. Start the Firebase Auth emulator.
4. Start the backend with `FIREBASE_AUTH_EMULATOR_HOST` set.
5. Sign in through the Firebase Auth emulator from Android or another client.
6. Call `POST /v1/dev/me/bootstrap` with the emulator-issued Firebase ID token.
7. Continue using the normal REST and WebSocket flows.

## Suggested `.env.local`

```bash
DATABASE_URL=postgres://postgres:postgres@localhost:5432/smallworld?sslmode=disable
REDIS_URL=redis://localhost:6379/0
FIREBASE_PROJECT_ID=smallworld-a0223
FIREBASE_AUTH_EMULATOR_HOST=127.0.0.1:9099
GOOGLE_MAPS_API_KEY=your-google-maps-api-key
```

## Bootstrapping A Local Backend User

The emulator-only bootstrap endpoint creates or updates:

- the backend `User`
- `IdentityVerification` with `provider = dev_emulator` and `status = verified`
- an optional active `Vehicle`

Example:

```bash
curl -X POST http://localhost:8080/v1/dev/me/bootstrap \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "display_name": "Andrea",
    "verified_gender": "female",
    "vehicle": {
      "make": "Toyota",
      "model": "Yaris",
      "color": "Blue",
      "license_plate": "DEV-001",
      "capacity": 3
    }
  }'
```

This is the preferred local bootstrap flow for new emulator users instead of relying on seeded fixed users.

## Shell Smoke Testing

Mint a Firebase emulator token locally:

```bash
export FIREBASE_AUTH_EMULATOR_HOST=127.0.0.1:9099
AUTH_TOKEN="$(./scripts/get-auth-emulator-token.sh andrea@example.com password123)"
```

Verify auth against the backend:

```bash
AUTH_TOKEN="${AUTH_TOKEN}" ./scripts/smoke-auth.sh
```

If you want to continue into routing tests:

```bash
AUTH_TOKEN="${AUTH_TOKEN}" ./scripts/smoke-routing.sh
```

Note:

- `smoke-routing.sh` also requires `VEHICLE_ID`
- a fresh emulator user will not automatically match seeded demo data

## Android Emulator Note

When the Android app runs inside the Android emulator, `10.0.2.2` usually maps back to the host machine.

Typical local Firebase Auth emulator target from Android:

- host: `10.0.2.2`
- port: `9099`

## Operational Notes

- never enable emulator mode in production
- never commit Firebase credentials
- keep local development on the emulator path and production-like environments on real Firebase verification
