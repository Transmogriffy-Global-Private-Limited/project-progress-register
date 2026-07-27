# ADR 0003 — Operational evidence and conservative geofence verification

Status: Accepted; Step 06 implementation in progress

Date: 2026-07-27

## Context

Progress updates need mobile camera photographs, current browser location, accuracy, site geofence evaluation, and a server-authoritative time. Browser data is useful but cannot attest hardware truth or prevent a compromised client from spoofing input.

## Decision

Treat evidence as operational, not cryptographic. Every file upload requires finite browser coordinates and positive accuracy; text-only progress may omit location. Store the complete upload-location and geofence snapshot even when accuracy or distance fails policy. Server-side Haversine verification satisfies `distance + accuracy <= radius`.

Verification is attachment-specific. Only an image reported as an in-Chrome camera capture is eligible, and only when reported accuracy is within policy and the uncertainty circle is inside the geofence. Existing images, documents, and videos are explicitly non-verified while remaining geotagged. Geofence failure affects classification, not submission acceptance.

Server receipt time, authenticated actor, hashes, stored policy, calculation, and result are authoritative. Coordinates, accuracy, browser time, camera-flow source, and all EXIF fields retain explicit lesser trust classifications.

## Consequences

- The conservative uncertainty rule may classify a camera photo non-verified when its centre is inside but accuracy overlaps the boundary.
- Client previews never decide verification.
- Missing location blocks file bytes but not text-only progress. Inaccurate/outside locations and non-camera file sources remain accepted but cannot produce a verified label.
- UI and documentation must not claim proof of presence or tamper-proof source evidence.
