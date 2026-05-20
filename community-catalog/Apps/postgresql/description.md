# PostgreSQL

PostgreSQL is the world's most advanced open-source relational database. This instance runs as a headless backend for other apps. Connect on port 5432 with any Postgres-compatible client (psql, pgAdmin, etc.).

**Database port:** PostgreSQL is a backend service — connect with `psql -h <host> -p 5432 -U postgres`. Credentials are set via `POSTGRES_PASSWORD` environment variable.

**Data:** `/DATA/PowerLabAppData/postgresql/` — Database cluster data, WAL logs, and PostgreSQL configuration.
