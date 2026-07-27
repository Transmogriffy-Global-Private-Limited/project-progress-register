# ADR 0001 — Go modular monolith

Status: Accepted

Date: 2026-07-27

## Context

The product is an internal progress register targeting a 2 GB Ubuntu VPS. Its workflows share identity, project scope, transactions, revisions, files, and audit requirements. Independent scaling or ownership does not exist.

## Decision

Use one Go modular-monolith process and one PostgreSQL database. Keep configuration, transport, authentication/authorization, application services, domain rules, persistence, attachments, evidence, Markdown, and audit concerns explicit. Add packages only with a real vertical feature.

## Consequences

- One binary and process minimize memory and operations.
- In-process calls and database transactions keep causal behavior easy to trace.
- Module ownership still prohibits arbitrary cross-package table writes.
- A future service extraction requires a concrete scaling, security, operational, or ownership need and a new ADR.
