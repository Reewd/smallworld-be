# Smallworld Backend

Smallworld is a real-time carpooling app for short-notice, flexible rides. This repository contains the Go backend for the MVP.

The backend is authoritative for:

- backend user bootstrap from Firebase auth identity
- driver and rider capability checks
- route-aware matching between riders and active driver sessions
- ride offer dispatch and acceptance
- booking state and fare quote persistence
- protected driver tracking visibility for matched riders only
- reviews and realtime server-to-client events

The Android app owns maps, turn-by-turn navigation UX, and client-side journey presentation.

## Local Orientation

1. Start local infrastructure with `docker compose up -d`.
2. Create `.env.local` from [.env.example](./.env.example).
3. Apply database migrations with `./scripts/migrate.sh up`.
4. Start the API with `./scripts/run-dev.sh`.

Local development should use the Firebase Auth emulator. Production-style verification should use Firebase credentials.

## Documentation

- [docs/architecture.md](./docs/architecture.md): canonical backend architecture, domain rules, invariants, and runtime shape
- [docs/development-workflow.md](./docs/development-workflow.md): contributor playbook for adding handlers, services, policies, persistence, and tests
- [docs/firebase-auth-setup.md](./docs/firebase-auth-setup.md): production and staging Firebase auth setup
- [docs/firebase-auth-emulator-setup.md](./docs/firebase-auth-emulator-setup.md): local emulator auth workflow and bootstrap flow
- [docs/database-migrations.md](./docs/database-migrations.md): migration commands and local reset workflow
- [docs/openapi.yaml](./docs/openapi.yaml): canonical HTTP API contract
