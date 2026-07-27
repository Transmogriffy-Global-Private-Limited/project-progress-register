# ADR 0005 — Server-rendered mobile-first UI

Status: Superseded by ADR 0008 for future feature work; existing minimal shell retained

Date: 2026-07-27

## Context

Users are accustomed to paper logs and spreadsheets. The application needs simple pages, mobile camera/geolocation behavior, accessible controls, low VPS memory use, and no production Node.js runtime.

## Decision

Use Go `html/template` for server-rendered pages, ordinary HTML forms, focused HTMX only where partial-page interaction materially helps, and minimal vanilla JavaScript for camera, geolocation, uploads, Markdown controls, and small UI behavior.

Keep templates presentation-focused. HTML and JSON transports call the same application services and authorization policies.

## Consequences

- Initial rendering, navigation, deployment, and recovery stay simple.
- No SPA state becomes an accidental source of truth.
- Camera/location JavaScript remains a narrow progressive layer, although those permissions are mandatory for verified submissions.
- Accessible semantics, large touch targets, keyboard operation, and non-colour status cues are required in every UI slice.
