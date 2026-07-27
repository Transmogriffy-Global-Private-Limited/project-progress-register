# ADR 0003 — Operational evidence and conservative geofence verification

Status: Accepted; implementation planned

Date: 2026-07-27

## Context

Progress updates need mobile camera photographs, current browser location, accuracy, site geofence evaluation, and a server-authoritative time. Browser data is useful but cannot attest hardware truth or prevent a compromised client from spoofing input.

## Decision

Treat evidence as operational, not cryptographic. Require at least one in-application camera capture, finite browser coordinates, positive accuracy no greater than project policy, and server-side Haversine distance satisfying `distance + accuracy <= radius`. Store the complete geofence snapshot and reason with the evidence.

Server receipt time, authenticated actor, hashes, stored policy, calculation, and result are authoritative. Coordinates, accuracy, browser time, camera-flow source, and all EXIF fields retain explicit lesser trust classifications.

## Consequences

- The conservative uncertainty rule may reject a reading whose centre is inside but accuracy overlaps the boundary.
- Client previews never decide verification.
- Denied permissions, inaccurate/outside location, missing camera capture, and storage failures cannot produce a verified label.
- UI and documentation must not claim proof of presence or tamper-proof source evidence.
