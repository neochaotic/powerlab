# 0031. oapi-codegen for OpenAPI to Go

- **Status:** accepted
- **Date:** 2026-05-14

## Context

PowerLab follows an API-first design methodology. Every microservice has a canonical `openapi.yaml` file defining its REST API. To ensure the Go implementation strictly adheres to the OpenAPI specification, we need a tool to generate Go boilerplate (types, HTTP server scaffolding, and request/response validation) directly from the YAML files.

## Decision

We use [`oapi-codegen`](https://github.com/deepmap/oapi-codegen) as our standard OpenAPI-to-Go generator.

- Each service includes a `//go:generate` directive to invoke `oapi-codegen`.
- The generator produces types, server interfaces, and request validators.
- **The codegen output (`codegen/`) is `.gitignore`d.** We do not commit generated Go code to the repository.

## Consequences

- **Positive:** Guaranteed consistency between the documentation (`openapi.yaml`) and the actual implementation. The compiler enforces that our route handlers match the generated interfaces.
- **Positive:** We get free request validation middleware based on the OpenAPI schema.
- **Positive (via gitignore):** PR diffs are clean. Reviewers only see the schema changes in `openapi.yaml` and the handler logic changes, without being spammed by thousands of lines of generated boilerplate.
- **Negative:** Developers must remember to run `go generate ./...` (or `make generate` / `dev.sh`) after pulling changes or modifying an `openapi.yaml` file, otherwise their local build will fail.
