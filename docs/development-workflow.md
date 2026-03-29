# Smallworld Backend Development Workflow

This document is the canonical contributor playbook for changing the backend safely.

Use it together with:

- [architecture.md](./architecture.md) for durable product rules and system boundaries
- [openapi.yaml](./openapi.yaml) for the current HTTP contract

## Start With The Request Path

Most feature work becomes easier once you map it onto the actual request path used in this repo:

1. `authMiddleware` in [internal/interfaces/http/auth.go](../internal/interfaces/http/auth.go) verifies the bearer token and resolves request identity.
2. A route is registered in [internal/interfaces/http/router.go](../internal/interfaces/http/router.go).
3. A thin handler decodes input, resolves the current user or identity, and calls one service method.
4. The service in `internal/application/service` loads and persists state through `ports`.
5. Domain rules from [internal/domain/policies.go](../internal/domain/policies.go) and sentinel errors from [internal/domain/errors.go](../internal/domain/errors.go) drive business decisions.
6. The handler returns JSON through `writeJSON`, `writeRequestError`, or `writeServiceError`.

When you understand that path, it becomes clear where a change belongs and where it does not.

## Choose The Right Layer

### Put it in `internal/domain` when

- the rule expresses business meaning, not transport or storage detail
- the rule should stay true no matter how the endpoint or repository changes
- the rule is reusable across more than one service or code path
- the change affects state transitions, eligibility, or invariant enforcement

Typical domain changes:

- new fields in [internal/domain/models.go](../internal/domain/models.go)
- new states in [internal/domain/states.go](../internal/domain/states.go)
- new transition or capability rules in [internal/domain/policies.go](../internal/domain/policies.go)
- new sentinel business errors in [internal/domain/errors.go](../internal/domain/errors.go)

### Put it in `internal/application/service` when

- the change is a use case or workflow
- you need to coordinate multiple repositories or providers
- the logic depends on actor ownership, sequencing, or side effects
- you need to persist state and then trigger push or realtime behavior

Typical service responsibilities:

- load records through ports
- call domain rules
- assemble updated entities
- save changes
- publish dependent side effects

Do not put HTTP concerns in services:

- no status codes
- no response shaping
- no request parsing

### Put it in `internal/interfaces/http` when

- the work is about route registration
- request parsing or validation is transport-specific
- you need to resolve the current user or auth identity
- you need to map service errors to HTTP responses

Handlers should remain thin. If a handler starts making multiple business decisions, move that logic into a service or domain helper.

### Put it in `internal/ports` when

- a service genuinely needs a new infrastructure capability
- the application layer depends on an abstraction that does not yet exist

Do not add ports just to mirror an adapter's internal detail. Add them only when the service layer needs them.

### Put it in `internal/infrastructure/*` when

- you are implementing a port
- you are dealing with SQL, Redis, Firebase, routing providers, or realtime transport
- the change is specific to one concrete adapter

If a port changes, update every implementation that satisfies it, not just Postgres.

## How To Add Or Change An HTTP Endpoint

Use this sequence for new backend API behavior.

1. Confirm whether the change introduces a new business concept, state, or invariant.
2. If it does, change the domain model or policies first.
3. Add or update a service method in `internal/application/service`.
4. Change a port only if the service needs a new repository or provider capability.
5. Implement the port changes in infrastructure.
6. Wire the route and thin handler in [internal/interfaces/http/router.go](../internal/interfaces/http/router.go).
7. Update [docs/openapi.yaml](./openapi.yaml) if the public contract changed.
8. Add tests at the right layer.

The handler itself should usually do only this:

- resolve identity or user ID
- decode JSON
- call one service method
- map errors with `writeServiceError`
- encode the response

## When A New Service Method Is Enough

You usually only need a new service method when:

- the data model stays the same
- the workflow is new, but it can be expressed using existing entities and ports
- the change is mainly orchestration, ownership checks, or side effects

Examples:

- a new booking transition flow using existing booking state
- a new current-state lookup endpoint using existing repositories
- a new sequence that publishes realtime events after an existing persistence update

Do not add domain types or policies just because you added a new endpoint. Add them only when the business model changed.

## When To Add A Domain Policy

Add or update a domain policy when the rule is a true invariant or reusable business constraint.

Good reasons to add a policy:

- a new state transition must be allowed or denied consistently
- driver or rider eligibility changed
- an ownership or capability rule must be enforced in multiple services
- a new invariant should stay true even if new endpoints are added later

Keep the policy close to domain meaning. A good policy should not need to know about:

- HTTP request shape
- SQL queries
- Redis layout
- logger details

If the rule depends on loaded records and sequencing, the service should gather data first and then call the domain helper.

## When To Add A New Domain Error

Add a new sentinel error in [internal/domain/errors.go](../internal/domain/errors.go) when callers need stable branching behavior.

Use a domain error when:

- a handler or another service must distinguish this outcome from generic failure
- the failure represents a business rule, not an infrastructure outage
- you want centralized HTTP mapping to stay consistent

Do not create a new domain error for every internal failure. Repository and provider failures can often bubble up as ordinary errors and become `500` at the boundary.

## Error Handling Rules

The repo already has a clear error pattern. Preserve it.

### In domain and service code

- return domain sentinel errors for expected business failures
- return infrastructure errors upward when they are not business decisions
- use ownership checks such as `ErrUnauthorized` in services, not in handlers
- use transition helpers such as `RequireTransition` instead of scattering ad hoc transition logic

### In handlers

- use `decodeJSONBody` plus `writeRequestError` for direct request-decoding failures
- use `writeServiceError` for service and domain errors
- let `resolveServiceError` in [internal/interfaces/http/router.go](../internal/interfaces/http/router.go) decide status and public message
- keep malformed JSON and unknown-field failures on the stable client message `invalid request body`
- do not hand-roll route-specific status logic unless the route truly needs special handling

### In auth flow

- keep bearer-token parsing and token verification in [internal/interfaces/http/auth.go](../internal/interfaces/http/auth.go)
- do not duplicate auth parsing inside handlers
- use `currentIdentity` or `currentUserID` from the request context

### Logging and safety

- keep logging structured
- do not log bearer tokens, request bodies, or secrets
- preserve the rule that internal server errors are masked at the HTTP boundary

## When To Change A Port

Change a port only when a service cannot express its use case with the current abstraction.

Good reasons:

- a service needs a repository lookup that does not exist
- a provider must expose a new capability
- a worker needs a new storage operation

Whenever a port changes, check at least:

- Postgres implementations under [internal/infrastructure/postgres](../internal/infrastructure/postgres)
- in-memory adapters under [internal/infrastructure/memory](../internal/infrastructure/memory)

## Persistence And Config Changes

### If schema changes

1. Add a migration in [db/migrations](../db/migrations).
2. Update Postgres reads, writes, scans, and row mappers.
3. Update specialized transactional paths such as [internal/infrastructure/postgres/offer_acceptor.go](../internal/infrastructure/postgres/offer_acceptor.go) when relevant.
4. Update memory adapters if service tests depend on the changed shape.
5. Add or update tests that prove round-trip persistence.

### If config changes

1. Add explicit env loading in bootstrap code.
2. Update [.env.example](../.env.example).
3. Document the new variable in the most relevant doc.

Use explicit env vars only. Do not introduce magic config discovery.

## Testing Expectations

Choose the smallest useful layer that proves the behavior.

### Domain tests

Use for:

- pure eligibility rules
- transition logic
- invariant enforcement

### Service tests

Use for:

- orchestration
- ownership checks
- domain-rule enforcement through the service layer
- side-effect sequencing

Most service tests in this repo should use the in-memory adapters in [internal/infrastructure/memory](../internal/infrastructure/memory), not Postgres or Redis directly.

### Router tests

Use for:

- request and response behavior
- auth behavior
- centralized error mapping
- route wiring

See [internal/interfaces/http/router_test.go](../internal/interfaces/http/router_test.go).

### Infrastructure tests

Use for:

- SQL behavior
- Postgres scanning or transactional code
- Redis behavior
- provider-specific adapter behavior

Before wrapping up, run:

```bash
go test ./...
```

## Feature Completion Checklist

- Domain
  - Did the business model actually change?
  - Are transitions and invariants still enforced in the domain layer?
- Services
  - Is the orchestration in the service layer?
  - Did you avoid transport concerns in services?
- Ports and infrastructure
  - Did you change ports only when needed?
  - Did you update every implementation consistently?
- HTTP
  - Did you keep the handler thin?
  - Did you preserve centralized error mapping?
- Database
  - Did you add a migration and update all affected SQL paths?
- Config
  - Did you update `.env.example` and bootstrap wiring?
- Public API
  - Did you update [docs/openapi.yaml](./openapi.yaml) when needed?
- Docs
  - Did you update [architecture.md](./architecture.md) if a durable product rule or invariant changed?
- Tests
  - Did you add or update the right tests?
  - Does `go test ./...` pass?

## Documentation Update Rules

Update docs when behavior changes, not only when shipping a major feature.

- Update [architecture.md](./architecture.md) for durable product rules, architecture boundaries, or invariant changes.
- Update [docs/openapi.yaml](./openapi.yaml) for public API changes.
- Update [.env.example](../.env.example) for new env vars.
- Update the specialized setup docs when auth, emulator flow, or migration operations change.

## Rule Of Thumb

When in doubt:

- put business meaning in `domain`
- put orchestration in `service`
- put transport handling in `interfaces/http`
- put concrete external-system details in `infrastructure`
- keep `cmd/api` responsible for wiring, not business logic
