# Progress evidence and file verification

Status: Implemented, automated-verified, migrated, and deployed; guarded database-live lifecycle verification pending

## The two separate decisions

The backend records two related but different facts:

1. **Upload location result** — whether browser-reported coordinates passed the current project geofence and accuracy policy.
2. **Attachment verification result** — whether one specific file is eligible to be called verified.

Every file needs coordinates. Coordinates may be inside, outside, inaccurate, or evaluated without a configured geofence; all those files are accepted and retain the geotag. Text-only progress may omit location or report why the browser could not supply it.

Only an image or video whose `source` is `camera` may be verified. The `camera` value is still a browser-reported Chrome workflow assertion, not hardware attestation. The media becomes verified only when the server also returns upload-location status `verified`.

| File | Source | Location result | Attachment result |
|---|---|---|---|
| Image | `camera` | `verified` | `verified` |
| Image | `camera` | outside/inaccurate/no geofence | `non_verified` |
| Image | `upload` | any | `non_verified` |
| Document | `upload` | any | `non_verified` |
| Video | `camera` | `verified` | `verified` |
| Video | `camera` | outside/inaccurate/no geofence | `non_verified` |
| Video | `upload` | any | `non_verified` |

The frontend must display the backend's exact `verification_status` and `verification_reason`. It must not infer verification from the presence of coordinates.

## Creation flow

```text
Authenticated current project actor
-> browser obtains upload location when files are present
-> multipart metadata plus ordered files
-> backend validates CSRF, idempotency, location, count, size, and type
-> server snapshots current geofence and calculates Haversine distance
-> each file receives its own verification result
-> PostgreSQL commits progress plus pending attachment metadata and audit
-> staged files are atomically finalized and marked available
-> response returns sanitized Markdown, evidence, attachment metadata, and authorized content paths
```

The client must send a stable `Idempotency-Key` for retries. Reusing a key with different metadata or file hashes returns `409`.

## Failure and recovery

If metadata commits but a file cannot finish its atomic rename, the API returns `503 attachment_pending`. Retry with the same idempotency key. The runtime also reconciles pending attachments immediately and every minute. Pending bytes are never served as available.

Current progress edits change written Markdown only. Evidence and attachments stay immutable; every edit appends a before/after revision.

Authorized video content uses the existing nested `content_path`. The response uses detected video MIME, inline disposition, and HTTP byte ranges so a browser `<video>` element can seek without loading the entire file. Images and documents retain attachment disposition.
