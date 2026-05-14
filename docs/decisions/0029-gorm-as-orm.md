# 0029. GORM as the ORM (AutoMigrate Forbidden)

- **Status:** accepted
- **Date:** 2026-05-14

## Context

Several PowerLab services (`core`, `app-management`, `user-service`, `local-storage`) require persistent state. We use SQLite as the underlying database engine. To interact with the database, we need a consistent pattern for data access.

## Decision

We use [GORM](https://gorm.io/) as the standard ORM for all persistent services.

However, we place a strict restriction on its use: **`db.AutoMigrate()` is explicitly forbidden.** 

- All database schemas must be explicitly defined.
- Schema changes and initial table creation must be handled by `goose` migration scripts (as defined in [ADR-0018](0018-goose-for-versioned-migrations.md)).
- All model structs must explicitly implement the `TableName()` method to define their exact SQLite table name, rather than relying on GORM's pluralization reflection.

## Consequences

- **Positive:** Developer velocity. GORM provides a rich query builder that abstracts away much of the SQLite-specific SQL boilerplate.
- **Positive:** Consistent data access patterns across all microservices.
- **Positive (via Restriction):** By forbidding `AutoMigrate()`, we prevent silent, unpredictable schema mutations in production. All schema changes are versioned, reproducible, and tracked in git via `goose`.
- **Negative:** GORM can obscure inefficient SQL queries (N+1 queries). Developers must explicitly use `Preload` for relationships and monitor query performance.
