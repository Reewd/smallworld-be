# Smallworld Backend Architecture

This document is the canonical source for backend architecture, durable product rules, system boundaries, and invariants that contributors should preserve.

For how to make changes in this repo, use [development-workflow.md](./development-workflow.md). For the public HTTP contract, use [openapi.yaml](./openapi.yaml).

## Product Summary

Smallworld is a real-time carpooling app for short-notice, flexible rides.

A rider submits an origin and destination "now". The backend checks whether another verified user who is already driving on a compatible route can pick them up, potentially with:

- driver detour
- rider walking to pickup
- rider walking from dropoff

The backend is authoritative for:

- identity capability checks
- driver and rider eligibility
- route-aware matching
- offer dispatch
- booking state
- quoted fare
- reviews

The Android app owns:

- map rendering
- turn-by-turn navigation UX
- polyline rendering
- on-device navigation SDK

## Non-Negotiable Domain Rules

- Do not model `driver` and `rider` as mutually exclusive account types.
- Use a single `User` identity.
- Capability is layered on top of `User`, not encoded as a permanent role.
- Role is contextual per trip or session.
- A user may be a rider on one trip and a driver on another.

Current capability model:

- `User`
- `IdentityVerification`
- `Vehicle`
- `DriverSession`
- `TripDemand`
- `RideOffer`
- `RideBooking`
- `Review`

## Locked Product Decisions

- Pooled riders are supported.
- A driver session can host multiple riders up to vehicle capacity.
- Vehicle capacity is mandatory and matchable.
- License plate is stored on the vehicle and revealed after booking assignment.
- Users have walking and detour preferences:
  - rider max walk to pickup
  - rider max walk from dropoff
  - driver max pickup detour
- Riders can require women drivers only.
- Gender comes from the identity verification provider.
- Women-only matching is a hard filter using verified gender only.
- Rider search has no independent TTL in the prototype.
- If the rider leaves the search flow, the app should explicitly cancel the demand.
- No extra fare is charged for detours.
- Fare is based on the matched in-car ride leg only.
- Matching preference is strict:
  - first prefer driver detours within the driver's threshold
  - only then consider rider walking pickup and dropoff points
- Pickup timing rule:
  - rider may wait
  - driver must not materially wait
  - implemented as a hard ETA constraint with a small safety buffer

## Domain Model

- `User`
  Shared account identity with `auth_subject`, `display_name`, `average_rating`, and default preferences.
- `IdentityVerification`
  Provider-backed verification state, including verification status and verified gender.
- `Vehicle`
  Driver capability attachment with capacity, license plate, and active flag.
- `DriverSession`
  Live supply object for a driver currently on route, including route geometry, current location, remaining capacity, detour threshold, and heartbeat freshness.
- `TripDemand`
  Rider search request, including requested origin and destination, matched pickup and dropoff, women-only filter, and walk limits.
- `RideOffer`
  One pending offer sent to one driver session.
- `RideBooking`
  Accepted rider-to-driver assignment, including matched pickup and dropoff, walking distances, pickup detour, fare quote, and vehicle plate reveal.
- `Review`
  Post-completion feedback between rider and driver.

Key domain code:

- [internal/domain/models.go](../internal/domain/models.go)
- [internal/domain/states.go](../internal/domain/states.go)
- [internal/domain/policies.go](../internal/domain/policies.go)
- [internal/domain/errors.go](../internal/domain/errors.go)

## State Machines

### Trip demand

- `draft`
- `searching`
- `offered`
- `matched`
- `canceled`
- `aborted`

### Ride offer

- `pending`
- `accepted`
- `declined`
- `expired`
- `withdrawn`

`withdrawn` means the backend invalidated an offer before normal expiry because it was no longer current or valid.

### Ride booking

- `assigned`
- `rider_walking_to_pickup`
- `driver_en_route_to_pickup`
- `pickup_ready`
- `onboard`
- `completed`
- `canceled`
- `no_show`

### Driver session

- `active`
- `full`
- `paused`
- `ended`

Transition guards are implemented in [internal/domain/policies.go](../internal/domain/policies.go).

## Matching Semantics To Preserve

- Search active driver sessions only.
- Respect remaining vehicle capacity.
- Apply the women-only hard filter when requested.
- First attempt matching with driver detour only.
- Only if phase 1 fails, evaluate rider walking pickup and dropoff candidates.
- Enforce the driver pickup detour threshold as a hard limit.
- Enforce rider walk limits as hard limits.
- Enforce the no-driver-wait pickup timing rule with an ETA safety buffer.
- Rank candidates by:
  - lower driver detour
  - faster pickup ETA
  - lower rider walking burden
  - better overlap
  - healthier remaining capacity

Current limitation: matching uses route and ETA provider calls, but route overlap is still heuristic rather than full corridor-aware geometry matching.

Matching engine code:

- [internal/matching/engine.go](../internal/matching/engine.go)

## Layered Architecture

### `internal/domain`

Owns business vocabulary, state enums, transition rules, capability rules, and sentinel errors. Keep pure domain meaning here when the rule:

- expresses an invariant that should survive transport or storage changes
- is reusable across multiple services
- can be evaluated without knowing about HTTP or database details

Examples:

- eligibility helpers such as `RiderEligible` and `DriverEligible`
- state-transition checks such as `CanTransitionTo`
- reusable transition enforcement through `RequireTransition`

### `internal/application/service`

Owns use-case orchestration. Services compose repositories and providers through `ports`, call domain rules, persist state changes, and trigger dependent side effects such as realtime or push delivery.

Services should:

- load domain records from repositories
- enforce ownership and policy checks
- call domain helpers before transitions
- persist updated entities
- publish dependent effects when the use case requires them

Services should not:

- return HTTP status codes
- shape JSON responses
- know about request decoding details

### `internal/ports`

Defines the interfaces that application services depend on. Add or change a port only when a service truly needs a new capability from infrastructure.

Examples:

- repositories for Postgres-backed business state
- provider interfaces for routing, pricing, auth, push, realtime, and Redis-backed ephemeral state

### `internal/interfaces/http`

Owns transport concerns only:

- auth and request identity resolution
- route registration
- request decoding
- calling a service
- centralized error mapping
- response encoding

Handlers should stay thin. Business rules belong in domain and service code, not in HTTP handlers.

### `internal/infrastructure/*`

Owns concrete adapters:

- Postgres repositories and transactional offer acceptance
- Redis-backed driver presence and ephemeral offers
- Firebase auth verification
- routing providers
- realtime hub
- noop push provider and pricing implementation
- in-memory adapters used by service tests

### `cmd/api`

Owns process bootstrap and dependency wiring:

- env/config loading
- logger setup
- provider selection
- repository and adapter construction
- service assembly through `application.NewServices`
- worker startup
- HTTP server lifecycle

Key entrypoints:

- [cmd/api/main.go](../cmd/api/main.go)
- [internal/application/app.go](../internal/application/app.go)
- [internal/interfaces/http/router.go](../internal/interfaces/http/router.go)

## Request And Execution Model

At a high level, a normal HTTP request moves through the backend like this:

1. `authMiddleware` verifies the bearer token and resolves the current identity.
2. The router dispatches to a thin handler.
3. The handler decodes JSON and calls the relevant service.
4. The service loads records through `ports`, applies domain rules, and persists changes.
5. The service triggers dependent effects such as realtime publication or push.
6. The handler maps the result or error to JSON.

That flow is visible in:

- [internal/interfaces/http/auth.go](../internal/interfaces/http/auth.go)
- [internal/interfaces/http/router.go](../internal/interfaces/http/router.go)
- [internal/application/service](../internal/application/service)

## Error Model

The backend uses a layered error strategy.

- Domain sentinel errors live in [internal/domain/errors.go](../internal/domain/errors.go).
- Services return those domain errors when callers need stable branching behavior, for example `ErrUnauthorized`, `ErrVerificationRequired`, `ErrVehicleRequired`, or entity-not-found errors.
- Services may also return infrastructure errors directly when the failure is not a business decision, such as a routing provider failure or repository write failure.
- The HTTP layer maps domain sentinel errors centrally through `resolveServiceError` and `writeServiceError` in [internal/interfaces/http/router.go](../internal/interfaces/http/router.go).
- The HTTP layer handles JSON request-decoding failures separately through `decodeJSONBody` and `writeRequestError` in [internal/interfaces/http/router.go](../internal/interfaces/http/router.go).
- Auth failures are handled before handlers run in [internal/interfaces/http/auth.go](../internal/interfaces/http/auth.go).

Important consequences:

- Do not put HTTP status logic in services.
- Prefer adding a domain sentinel error when multiple callers need to distinguish a business failure.
- Let handlers call `writeServiceError` instead of re-implementing per-route status mapping.
- Let handlers use `decodeJSONBody` for JSON bodies instead of returning raw decoder errors directly.
- Internal server errors are intentionally masked to `internal server error` at the HTTP boundary.
- Malformed or unknown JSON request bodies are intentionally sanitized to the stable client message `invalid request body`.
- Request logs may still record the internal decode cause for `4xx` failures, but they must not log the raw request body.

## Runtime And Infrastructure

### Persistent and ephemeral state

- PostgreSQL + PostGIS is the system of record for durable business state.
- Redis backs live driver presence and ephemeral pending offers.

Important Postgres code:

- [internal/infrastructure/postgres/repositories.go](../internal/infrastructure/postgres/repositories.go)
- [internal/infrastructure/postgres/offer_acceptor.go](../internal/infrastructure/postgres/offer_acceptor.go)

If `driver_sessions` schema changes, both of those files usually need updates.

### Auth

- The backend does not own login.
- Android signs in with Google through Firebase Auth.
- Android sends a Firebase ID token as a bearer token.
- The backend verifies the token and maps it to a local `User`.

Local development should use the Firebase Auth emulator, not a backend-only dev token path.

### Routing

- Routing is provider-agnostic behind `ports.RoutingProvider`.
- If `GOOGLE_MAPS_API_KEY` is set, the backend uses the Google Routes adapter.
- Otherwise it falls back to the haversine provider.

Routing code:

- [internal/ports/providers.go](../internal/ports/providers.go)
- [internal/infrastructure/routing/google_routes.go](../internal/infrastructure/routing/google_routes.go)

### Realtime and background work

- WebSocket delivery is implemented for server-to-client events.
- FCM push is still stubbed through a noop notifier.
- Offer sweeper worker cleans up stale offers and returns demands to `searching`.
- Driver presence reconciler pauses stale sessions based on heartbeat freshness.

Relevant code:

- [internal/infrastructure/realtime/hub.go](../internal/infrastructure/realtime/hub.go)
- [internal/background/offers](../internal/background/offers)
- [internal/background/presence](../internal/background/presence)

### Config

Use explicit env vars only. No magic discovery.

Important env vars:

- `DATABASE_URL`
- `REDIS_URL`
- `APP_ENV`
- `FIREBASE_PROJECT_ID`
- `FIREBASE_CREDENTIALS_FILE`
- `FIREBASE_AUTH_EMULATOR_HOST`
- `GOOGLE_MAPS_API_KEY`
- `LOG_LEVEL`
- `LOG_FORMAT`
- `DB_LOG_SLOW_QUERY_MS`
- `DB_LOG_ALL_QUERIES`
- `SEED_DEMO_DATA`

## Current Implementation Status

Implemented:

- Go modular monolith
- PostgreSQL + PostGIS repositories
- Redis-backed driver presence and ephemeral offers
- transactional Postgres offer acceptance with row locking
- offer cleanup sweeper worker
- driver presence reconciliation worker
- Firebase Auth verification with production credentials or Firebase Auth emulator support
- Google Routes adapter with haversine fallback
- driver session route polyline persistence
- WebSocket delivery for per-user server events
- emulator-only dev bootstrap endpoint for verified local users with optional vehicle creation
- convenience current-state endpoints for Android
- REST API for profile, vehicles, driver sessions, trip demands, offers, bookings, and reviews

Still missing or intentionally stubbed:

- real FCM push delivery
- real identity verification provider integration
- full route-overlap sophistication
- admin and support APIs
- audit event writes beyond schema presence

## Repo Landmarks

- [cmd/api/main.go](../cmd/api/main.go): process bootstrap and dependency wiring
- [internal/application/app.go](../internal/application/app.go): service graph assembly
- [internal/domain/models.go](../internal/domain/models.go): domain entities
- [internal/domain/policies.go](../internal/domain/policies.go): capability and transition rules
- [internal/domain/errors.go](../internal/domain/errors.go): sentinel business errors
- [internal/matching/engine.go](../internal/matching/engine.go): candidate evaluation and ranking
- [internal/interfaces/http/router.go](../internal/interfaces/http/router.go): route registration, handlers, and error mapping
- [internal/interfaces/http/auth.go](../internal/interfaces/http/auth.go): auth middleware and request identity resolution
- [internal/infrastructure/postgres/repositories.go](../internal/infrastructure/postgres/repositories.go): durable persistence implementation
- [internal/infrastructure/postgres/offer_acceptor.go](../internal/infrastructure/postgres/offer_acceptor.go): transactional offer acceptance path
- [docs/openapi.yaml](./openapi.yaml): canonical HTTP contract

## Highest-Value Next Backend Work

1. Real FCM push adapter
2. Better route-aware overlap matching
3. Real identity verification provider integration
4. Audit event writes
5. Admin and support APIs
6. Tighter public API documentation and tooling
