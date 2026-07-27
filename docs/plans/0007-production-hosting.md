# Plan 0007 — Production hosting at `/backend`

Status: Verified

## Objective

Run the verified Go backend as a hardened loopback-only systemd service, reset and migrate the explicitly selected `pprdb` production database, preserve private attachments, and expose only `/backend/*` through Caddy at `ppr.transev.site`.

## Scope

- Runtime `BASE_PATH` support across routes, redirects, compatibility pages, assets, session cookies, OpenAPI server selection, and embedded Swagger addressing.
- Root-protected production configuration with generated database, CSRF, and bootstrap secrets.
- A dedicated `ppr` operating-system user and least-privileged `ppr_runtime` PostgreSQL login.
- Verified pre-reset database dump, exact-target recreation, all five migrations, and zero initial business rows.
- A hardened `ppr.service`, private `/var/lib/ppr/attachments`, loopback listener, readiness, and restart behavior.
- Caddy routing for `ppr.transev.site/backend/*`, root rejection, validation, reload, and rollback backup.
- Authoritative production operations, bootstrap-removal, recovery, and verification documentation.

## Non-goals

- Running guarded lifecycle verifiers against the clean production database.
- Creating the first real Admin on the operator's behalf.
- Exposing Swagger in production.
- Adding a product frontend, container, public Go listener, or automatic migration at startup.
- Managing DNS records outside the already configured Caddy host.

## Acceptance criteria

- Unprefixed application routes return `404`; `/backend` redirects to `/backend/`.
- `ppr.service` runs as `ppr`, listens only on `127.0.0.1:18090`, and returns `200` for prefixed liveness/readiness.
- The API-documentation toggle controls both public Swagger and raw schema routes; the operator has temporarily enabled them. `/backend/setup` is available only until the first Admin exists.
- `pprdb` contains five applied migrations and zero initial users/business rows.
- The runtime login has DML-only application access while migrations use the privileged URL copied from the operator's `.env`.
- Caddy's full active configuration validates and preserves the prefix to the loopback upstream.
- Public HTTPS health succeeds after authoritative DNS resolves `ppr.transev.site` to the VPS.

## Verification state

Passed: focused prefix tests, formatting, module tidiness, vet, all Go tests, race tests, static build, OpenAPI validation/route coverage, systemd validation, Caddy template and active-config validation, migration status, clean row counts, service state, listener inspection, loopback health, disabled-doc routes, setup route, Caddy reload, HTTP-to-HTTPS hostname redirect, adapted upstream inspection, authoritative IPv4/IPv6 DNS, Let's Encrypt issuance, and public HTTPS health/route isolation over both address families.

The first certificate attempt correctly failed while DNS was absent. After the operator published A `72.61.245.64` and AAAA `2a02:4780:12:5ec8::1`, both authoritative records matched the VPS and reloading Caddy completed certificate issuance and public verification.
