# ADR 0006 — Specification-first OpenAPI and embedded Swagger UI

Status: Accepted

Date: 2026-07-27

## Context

The server-rendered application still exposes programmatic contracts. The schema, viewer, tests, and runtime must not drift or rely on Node.js or a public CDN.

## Decision

Maintain the authoritative OpenAPI 3.1 source at `api/openapi/v1/openapi.yaml`. Embed that exact file into the Go binary, validate it with `kin-openapi`, and use it for both the raw route and locally embedded Swagger UI assets.

Use `/api/v1` for application JSON routes, `/api/openapi/v1/openapi.yaml` for the raw contract, and `/api/docs/` for the viewer. `API_DOCS_ENABLED` defaults false and controls registration of the raw schema, viewer, and redirect together.

Route-coverage tests compare the implemented versioned API route registry with the document. Contract changes update handlers, models, tests, examples, OpenAPI, human semantics, state, plan, and changelog in one slice.

## Consequences

- One committed schema owns the contract; there is no independently edited generated output.
- Interactive assets work without external network access.
- API-documentation exposure is deliberate and verified in both toggle states.
- The embedded UI increases binary size modestly but avoids a production Node runtime and CDN supply-chain dependency.
