# HTTP API contract

The authoritative machine-readable source is `../../api/openapi/v1/openapi.yaml`. This document owns causal semantics that are awkward to express in schema. It describes only implemented behavior.

## Versioning and representation

Programmatic application routes use `/api/v1`. Foundation responses are JSON encoded as UTF-8. No authentication is required for health endpoints; they expose no configuration, credentials, database errors, hostnames, migration names, or business data.

## `GET /api/v1/health/live`

Purpose: prove that the Go process and HTTP transport can respond.

Flow:

```text
HTTP request -> method check -> static process response
```

It does not query PostgreSQL. Success is `200` with `{"status":"ok"}`. Other methods return `405` and `Allow: GET`.

## `GET /api/v1/health/ready`

Purpose: determine whether the instance may receive stateful application traffic.

Flow:

```text
HTTP request
-> method check
-> bounded PostgreSQL ping
-> migration ledger existence/checksum/current-state verification
-> 200 ready or 503 not_ready
```

Success is `200` with `{"status":"ready"}`. Any dependency or schema failure is `503` with `{"status":"not_ready"}`. Failures are safe to retry. Other methods return `405` and `Allow: GET`.

## API documentation routes

When `API_DOCS_ENABLED=true`:

- `GET /api/openapi/v1/openapi.yaml` serves the exact embedded authoritative schema.
- `GET /api/docs/` serves an embedded Swagger UI configured to read that route.
- `GET /api/docs` redirects to the canonical trailing-slash viewer route.

When false, none of these routes are registered and they return `404`. The toggle does not affect ordinary application or health routes.

## Common transport behavior

Responses receive MIME-sniffing, frame, referrer, and content-security headers. Unexpected handler panics are contained and mapped to `500`; details remain in structured logs. Request logs contain method, path, status, and duration but no request body or credentials.
