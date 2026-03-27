# Database Migrations

This guide covers the migration workflow for the Smallworld backend.

Use:

- [scripts/migrate.sh](../scripts/migrate.sh)

The script prefers a locally installed `migrate` binary and falls back to the official Docker image when available.

## Common Commands

Apply all migrations:

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/smallworld?sslmode=disable'
./scripts/migrate.sh up
```

Show the current version:

```bash
./scripts/migrate.sh version
```

Roll back one step:

```bash
./scripts/migrate.sh down 1
```

Create a new migration pair:

```bash
./scripts/migrate.sh create add_some_column
```

That creates versioned `up` and `down` SQL files in [db/migrations](../db/migrations).

## Destructive Operations

Destructive commands prompt for confirmation by default.

Skip the prompt only when you are sure:

```bash
./scripts/migrate.sh --yes down 1
./scripts/migrate.sh --yes force 3
```

`migrate drop` is intentionally disabled for this project because PostGIS owns tables such as `spatial_ref_sys`.

For a full local reset, use:

```bash
./scripts/migrate.sh reset-local
```

## Naming Convention

Migration filenames follow the `golang-migrate` convention:

- `000001_name.up.sql`
- `000001_name.down.sql`

Keep migrations:

- small and focused
- forward-safe to apply repeatedly
- reversible when practical

## Typical Local Flow

```bash
docker compose up -d
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/smallworld?sslmode=disable'
./scripts/migrate.sh up
./scripts/run-dev.sh
```

If you need to rebuild the local database from scratch:

```bash
./scripts/migrate.sh reset-local
```
