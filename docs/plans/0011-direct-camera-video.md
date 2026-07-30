# Direct camera video evidence and streaming plan

Status: Verified and deployed

## Objective

Allow Chrome/mobile frontend flows to submit a directly captured video as `source: camera`, classify it with the same location-dependent verified/non-verified rules as a directly captured photo, and stream authorized stored video through the existing attachment content route.

## Invariants

- `camera` remains a browser-reported application-flow assertion, not hardware attestation.
- Every attached video requires the same immutable upload-location snapshot as every other file.
- A direct camera photo or video is `verified` only when the server accepts reported accuracy and the complete uncertainty circle is inside the current geofence.
- Uploaded/gallery media remains `non_verified` regardless of location.
- Documents cannot claim `source: camera`.
- PostgreSQL metadata and private stored bytes remain authoritative; no transcoding, live broadcast, public file URL, or new media service is introduced.
- Uploads remain bounded and streamed to private staging storage. Authorized video reads use byte ranges and inline disposition; all reads still enforce current project access.

## Surface map

- Migration `000009`: permit camera-source video and verified camera photo/video rows while retaining database enforcement.
- Progress service: accept image/video camera media and reuse location verification for both.
- HTTP delivery: inline video response with byte-range support; attachment disposition remains for other media.
- OpenAPI/frontend contracts: direct video capture input, multipart source mapping, verification rules, and `<video>` playback guidance.
- Focused frontend handoff: `docs/integrations/FRONTEND_DIRECT_CAMERA_VIDEO.md` with copy-paste capture, upload, playback, retry, compatibility, and acceptance guidance.
- Verification: classification, invalid camera document, range streaming, OpenAPI/drift, guarded live workflow, and full suite.
- Documentation: product, evidence, domain, permissions, storage, API, frontend, state, and changelog.

## Acceptance criteria

- A server-detected allowed video with `source: camera` is accepted and retains the required upload geotag.
- The same video is `verified` when location status is `verified`, otherwise `non_verified` with the server reason.
- The same bytes with `source: upload` remain `non_verified`.
- A document with `source: camera` returns validation failure without committing progress or bytes.
- An authorized video GET returns detected video MIME, `Content-Disposition: inline`, `Accept-Ranges: bytes`, and correct `206` range responses.
- Images/documents retain attachment disposition and all download authorization/storage recovery behavior.
- The guarded live script sends a camera photo, camera video, and uploaded document in one ordered multipart request and checks classification plus range playback.

## Non-goals

- Live camera broadcasting, chunked/resumable upload protocol, transcoding, adaptive bitrate/HLS, thumbnails, media editing, antivirus, or cryptographic device attestation.

## Verification result

- Focused progress, HTTP transport, and migration tests pass, including verified camera video, rejected camera document, inline disposition, and a `206` byte-range response.
- OpenAPI validation and frontend handoff drift verification pass with all 43 operations and 31 paths represented.
- The full formatter, module-tidiness, vet, test, build, PowerShell-parse, loopback smoke, and Git-diff verifier passes, as does all-package race detection.
- The guarded database-live workflow was extended but not executed because it requires an explicitly disposable, fully migrated zero-user target.

## Deployment result

- Production was backed up as one coordinated schema-8 database/filesystem recovery point, migrated through `000009`, and rehosted from clean `main`.
- The migration ledger reports nine applied and zero pending migrations. Schema inspection confirms camera source and verified-row constraints permit only image/video media, leaving documents excluded.
- The hosted OpenAPI includes direct-camera video semantics plus authorized byte-range responses. Loopback and public IPv4/IPv6 readiness pass; the data-creating guarded workflow remains restricted to disposable targets.
