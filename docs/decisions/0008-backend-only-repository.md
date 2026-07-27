# ADR 0008 — Backend-only repository boundary

Status: Accepted

Date: 2026-07-27

## Context

The human clarified that current and future work in this repository covers only the backend. ADR 0005 previously assigned server-rendered product UI ownership to this codebase.

## Decision

This repository owns the Go/PostgreSQL backend, authentication and authorization, durable workflows, files and evidence policy, JSON HTTP APIs, OpenAPI, backend verification, and integration documentation. New product features expose backend contracts only. Product screens and client workflows belong to a separate frontend boundary.

The already implemented minimal setup/login/home/logout shell is retained for compatibility and local diagnostics; it does not establish permission to add new feature UI here.

## Consequences

- Feature slices remain end-to-end across backend persistence, policy, transport, contracts, recovery, verification assets, and documentation, but omit product UI implementation.
- OpenAPI and frontend-integration semantics must be complete enough for a separate client team.
- No React, HTMX, product templates, or client-side feature code is added to this repository without a new explicit decision.
- ADR 0005 remains historical and is superseded for future work.
