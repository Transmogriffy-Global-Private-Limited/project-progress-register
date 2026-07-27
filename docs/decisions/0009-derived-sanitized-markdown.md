# ADR 0009 — Store Markdown and derive sanitized HTML

Status: Accepted; implementation and automated verification complete

Date: 2026-07-27

## Context

Projects and tasks need simple rich text. Accepting or persisting client HTML would create an avoidable cross-site-scripting boundary and a second representation that could drift from Markdown source. A custom Markdown parser or sanitizer would be security-sensitive code without a product-specific benefit.

## Decision

Store only Markdown source. Use Goldmark with GitHub-flavored Markdown support for parsing and Bluemonday's UGC allowlist for sanitization. Derive read-only HTML fields on every API response. Render supplied Markdown before committing a write so renderer failure cannot produce a successful mutation with an unusable response.

## Consequences

- PostgreSQL has one textual source of truth.
- Sanitizer fixes apply to future reads without rewriting rows.
- Frontends receive both editable source and safe display HTML.
- Two focused Go dependencies are added; no process, service, cache, or Node.js runtime is introduced.
- Raw HTML behavior is intentionally constrained by the sanitizer rather than treated as trusted content.
