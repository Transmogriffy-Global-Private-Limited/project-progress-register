# ADR 0002 — Local accounts and backend authorization

Status: Accepted; automated identity, account, project, and task authorization verification passes; database-live verification remains pending

Date: 2026-07-27

## Context

V1 requires no public registration, routine website administration, Admin and Member roles, project-scoped Member access, immutable creator ownership, and operation on one internal deployment without an external identity provider.

## Decision

Use application-local username/email and password accounts, modern adaptive password hashing, opaque random database-backed sessions, secure HTTP-only cookies, CSRF protection, and database-backed login throttling. Store only hashes of session and reset tokens.

Authorize in application services, with project-scoped queries and database constraints reinforcing membership. Admin is a global application role as explicitly required. Member ownership uses immutable creator identity. Disablement revokes sessions, and the final enabled Admin cannot be disabled or demoted.

Bootstrap the first Admin through a one-time guarded setup flow. Routine administration remains website-only.

## Consequences

- No identity-provider dependency or public registration surface is introduced.
- Session revocation and account state are durable across restart.
- Every HTML form and cookie-authenticated JSON command requires CSRF validation.
- Password-reset delivery initially uses a single-use Admin-generated token shown once; email requires a later integration decision.
