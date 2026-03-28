# Firebase Auth Setup

This guide covers production and staging-style Firebase auth setup for the Smallworld backend.

For local development, use [firebase-auth-emulator-setup.md](./firebase-auth-emulator-setup.md).

## Auth Model

The backend is a resource server, not the identity provider.

The expected flow is:

1. The Android app signs the user in with Google through Firebase Authentication.
2. The Android app obtains a Firebase ID token.
3. The Android app sends that token to the backend as a bearer token.
4. The backend verifies the token and maps it to a local `User`.

Example request:

```http
Authorization: Bearer <firebase-id-token>
```

Because the backend verifies Firebase tokens, it does not need its own username/password or Google login endpoint.

## Required Env Vars

Set these when running without the Firebase Auth emulator:

```bash
export FIREBASE_PROJECT_ID="smallworld-a0223"
export FIREBASE_CREDENTIALS_FILE="/absolute/path/to/.secrets/firebase-service-account.json"
```

- `FIREBASE_PROJECT_ID` is the Firebase project ID.
- `FIREBASE_CREDENTIALS_FILE` points to a Firebase service-account JSON file the backend can use for token verification.

If `FIREBASE_AUTH_EMULATOR_HOST` is set instead, the backend runs in emulator mode and this production credential file is not required.

## Service Account Guidance

To let the backend verify Firebase tokens in production-style mode:

1. Open the Firebase project.
2. Go to project settings.
3. Open `Service accounts`.
4. Generate a new private key.
5. Store the JSON file outside git and point `FIREBASE_CREDENTIALS_FILE` at it.

Recommended local path:

- `.secrets/firebase-service-account.json`

Do not:

- commit the credential file
- keep it at the repo root as `firebase.json`

The Firebase CLI uses repo-root `firebase.json` for project config, so reusing that filename for credentials causes confusion.

## Firebase Console Setup

1. Create or open the Firebase project.
2. Open `Authentication`.
3. Go to `Sign-in method`.
4. Enable `Google`.
5. Register the Android app in Firebase.
6. Download `google-services.json` for the Android app.

## Backend Behavior

The backend requires:

- `FIREBASE_PROJECT_ID` always
- either `FIREBASE_CREDENTIALS_FILE` or `FIREBASE_AUTH_EMULATOR_HOST`

That behavior is wired in [cmd/api/main.go](../cmd/api/main.go).

## Useful Bootstrap Endpoints

After the Android app has a Firebase ID token, it can start with:

```http
GET /v1/auth/me
Authorization: Bearer <firebase-id-token>
```

That confirms the backend can verify the token and resolve the caller identity.

Then the app can create or update the backend profile:

```http
POST /v1/profile
Authorization: Bearer <firebase-id-token>
Content-Type: application/json
```

Example body:

```json
{
  "display_name": "Andrea",
  "preferences": {
    "walk_to_pickup": "medium",
    "walk_from_dropoff": "medium",
    "driver_pickup_detour": "medium"
  }
}
```

The backend stores those three values as preference tiers and maps them to concrete distance thresholds internally.

## Recommended Environment Strategy

- Use the Firebase Auth emulator for local development.
- Use real Firebase credentials in staging and production-like environments.
- Keep production verification on Firebase Admin credentials only.
