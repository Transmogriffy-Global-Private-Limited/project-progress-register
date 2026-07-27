# Plan 0003 — Account administration

Status: Implemented; automated verification passes, database-live lifecycle verification pending

## Objective

Deliver the complete internal account lifecycle as one larger backend vertical slice: Admin user inventory and creation, role and enabled-state management, password reset, forced temporary-credential replacement, session revocation, final-enabled-Admin protection, audit query, JSON contracts, and operational verification assets.

## Decisions

- No email or external identity provider is introduced. Creation and reset generate a high-entropy temporary password that is returned once to the initiating Admin and never stored or logged in plaintext.
- A created or reset user has `must_change_password=true`. After authenticating, that user may access only session recovery, logout, and password replacement until the password is changed.
- Password replacement revokes every session, including the current one, so the user signs in again with the new password. This avoids a second session-token rotation path.
- Admin changes use optimistic `version` checks. Role/enable mutations lock enabled Admin rows in PostgreSQL before evaluating the final-enabled-Admin invariant, so concurrent operations cannot remove every enabled Admin.
- Disabling, role-changing, resetting, or self-changing credentials revokes affected sessions in the same transaction as the user mutation and audit event.
- Usernames and emails remain immutable in this slice. Renaming identity creates audit and integration ambiguity without a current requirement.
- Account listing is ordered and unpaginated for the small internal-user assumption. Pagination is added only if a real size requirement appears.

## Surface map

- Migration: `users.must_change_password`; the existing enabled/role index supports the Admin lock query
- Identity policy: Admin authorization, temporary credential generation, create/update/reset/change commands, final-Admin errors, and forced-change access state
- PostgreSQL: atomic account mutation, session revocation, version checks, Admin locks, and audit insertion
- HTTP: authenticated Admin routing, path ID parsing, CSRF, strict JSON, status/error mapping, and no-store credential responses
- API: list/create/update/reset users and change own password
- Verification assets: service/store contract tests, HTTP tests, OpenAPI coverage, and a disposable live account-lifecycle verifier
- Documentation: architecture, domain, permissions, API semantics, PostgreSQL integration, project state, plan, and changelog

## Routes

JSON API:

- `GET /api/v1/admin/users`
- `POST /api/v1/admin/users`
- `PATCH /api/v1/admin/users/{user_id}`
- `POST /api/v1/admin/users/{user_id}/password-reset`
- `POST /api/v1/auth/password`
- `GET /api/v1/admin/audit/identity`

## Audit actions

- `identity.user_created`
- `identity.user_updated`
- `identity.password_reset`
- `identity.password_changed`
- `authorization.admin_users_denied`

## Acceptance criteria

- Only an authenticated Admin without a forced-password-change requirement can list or mutate users.
- Created accounts have unique normalized username/email, a generated temporary password, and `must_change_password=true`.
- The temporary password appears in exactly one successful response and is absent from persistence, audit, logs, URLs, and later reads.
- Expected-version mismatch returns conflict without mutation.
- Disabling or demoting the final enabled Admin fails even under concurrent attempts.
- Account disablement, role changes, resets, and password changes revoke affected sessions atomically.
- A forced-change user cannot access the application home or Admin operations until replacing the password.
- All authenticated writes require the existing session-bound CSRF token.
- The API contract is complete enough for a separate frontend to implement every account workflow without undocumented assumptions.
- OpenAPI and human contracts cover every endpoint, status, schema, side effect, and recovery rule.
- Admins can inspect the newest 200 identity/authentication/account-authorization audit events without exposing secrets.
- Tests and scripts are authored now but remain explicitly unexecuted until the human resumes verification.

## Verification backlog

When resumed: formatter, migration discovery, identity service tests, PostgreSQL integration against a reset disposable database, HTTP authorization/CSRF/one-time-secret tests, OpenAPI validation/route coverage, build, vet, race detector, loopback smoke, residue scan, full suite, and Git checks.
