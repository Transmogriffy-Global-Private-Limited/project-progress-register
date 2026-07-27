# Safe Markdown

Status: Implemented; sanitizer, domain, HTTP, contract, race, build, and loopback verification pass

## Purpose and source of truth

Project descriptions and task goals/descriptions store Markdown source in PostgreSQL. The source fields are authoritative. HTML is derived on each API response and is not persisted, versioned separately, or accepted from clients.

```text
JSON Markdown input
-> UTF-8 byte and field validation
-> durable Markdown transaction
-> Goldmark GFM rendering
-> Bluemonday UGC allowlist sanitization
-> read-only HTML response field
```

## Contract

- Project `description_markdown` produces `description_html`.
- Task `goals_markdown` produces `goals_html`.
- Task `description_markdown` produces `description_html`.
- Raw embedded HTML, scripts, event attributes, and unsafe URL schemes are not trusted and must not survive in the HTML projection.
- A frontend may use the source for editing. For display it should use the backend HTML projection or apply an equally strict renderer/sanitizer; it must never inject Markdown source directly as HTML.
- Sanitizer improvements affect subsequent reads without a data migration because derived HTML is not stored.

## Limits

- Project description: 20,000 UTF-8 bytes.
- Task goals: 20,000 UTF-8 bytes.
- Task description: 50,000 UTF-8 bytes.
- Request bodies retain the global 64 KiB HTTP limit.

## Failure and recovery

Rendering occurs before task/project writes when the source is supplied, so a renderer failure prevents mutation. Read rendering failure returns the generic server error and leaves durable Markdown unchanged. Restart reconstructs the stateless renderer; PostgreSQL remains authoritative.

## Verification

`internal/safemarkdown/renderer_test.go` asserts that ordinary Markdown renders and representative script/unsafe-link content is removed. Task service/HTTP coverage and `scripts/verify-live-task-register.ps1` exercise the derived fields. These checks are authored but unexecuted during the human-directed testing pause.
