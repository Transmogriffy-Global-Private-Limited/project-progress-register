# ADR 0006 — Specification-first OpenAPI and embedded Swagger UI

Status: Accepted

Date: 2026-07-27

## Context

The server-rendered application still exposes programmatic contracts. The schema, viewer, tests, and runtime must not drift or rely on Node.js or a public CDN.

## Decision

Maintain the authoritative OpenAPI 3.1 source at `api/openapi/v1/openapi.yaml`. Embed that exact file into the Go binary and validate it with `kin-openapi`. The raw route and locally embedded Swagger UI use a defensive copy whose only runtime-resolved value is the `basePath` server-variable default.

Use `/api/v1` for application JSON routes, `/api/openapi/v1/openapi.yaml` for the raw contract, and `/api/docs/` for the viewer. `API_DOCS_ENABLED` defaults false and controls registration of the raw schema, viewer, and redirect together.

The OpenAPI server URL exposes a `basePath` variable. Runtime `BASE_PATH` mounts all operations below that gateway prefix while keeping the documented operation paths canonical; production uses `/backend`. When documentation routes are registered, the served schema copy resolves that variable's default to the configured prefix so interactive requests work without manual server selection. The embedded Swagger viewer and schema URL use the same prefix, and Caddy preserves it rather than rewriting it.

Route-coverage tests compare the implemented versioned API route registry with the document. Contract changes update handlers, models, tests, examples, OpenAPI, human semantics, state, plan, and changelog in one slice.

## Consequences

- One committed schema owns the contract; there is no independently edited generated output.
- Runtime resolution is deliberately limited to one deployment value and fails route construction if its unique source marker drifts.
- Interactive assets work without external network access.
- API-documentation exposure is deliberate and verified in both toggle states.
- The embedded UI increases binary size modestly but avoids a production Node runtime and CDN supply-chain dependency.
