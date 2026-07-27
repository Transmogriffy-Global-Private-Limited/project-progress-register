# Evidence and trust model

## Purpose and limitation

The verified-update flow records useful operational evidence for internal project oversight. Browser camera and geolocation APIs do not provide cryptographic attestation and can be manipulated by a compromised device, browser, operating system, developer tools, or user. The product must never describe evidence as tamper-proof, court-grade, or proof of physical presence.

## Verified update algorithm

The browser obtains a fresh high-accuracy geolocation result and captures at least one image through the in-application camera workflow. The server then:

1. authenticates the actor and verifies current project membership or Admin access;
2. loads the task, project, and current geofence from PostgreSQL;
3. validates finite latitude/longitude values and a positive finite accuracy;
4. rejects accuracy greater than the configured maximum;
5. calculates great-circle distance with the Haversine formula using Earth radius 6,371,008.8 metres;
6. considers the location inside only when `distance + reported_accuracy <= geofence_radius`;
7. requires at least one attachment classified by the application flow as an in-application camera capture;
8. stores the server receipt time as authoritative;
9. stores reported location, accuracy, optional browser time, geofence centre/radius/accuracy threshold, computed distance, result, and reason as one immutable evidence snapshot.

Adding reported accuracy to distance is conservative: the reported uncertainty circle must fit inside the geofence. Boundary equality passes. Client-side display is only a preview; the server recomputes the decision.

If camera or location permission is denied, location is missing/stale, accuracy is insufficient, the location is outside, or storage fails, the submission is never labelled verified.

## Trust classifications

- **Server authoritative:** server receipt/upload/edit time, authenticated actor, authorization decision, server-computed SHA-256, server-detected MIME, stored geofence snapshot, computed distance, and verification result.
- **Browser reported:** coordinates, accuracy, browser-observed time, and the assertion that media came through the active application camera flow.
- **Untrusted embedded metadata:** EXIF capture time, EXIF GPS, device/camera model, and any metadata supplied inside a file.
- **Existing upload:** bytes selected from device storage. UI text must state “Location and timestamp not verified.”

Even the “in-application camera” source is application-flow evidence rather than hardware attestation.

## Attachment integrity and recovery

The server streams uploads to a staging directory while enforcing size/type limits and calculating SHA-256. Metadata records use opaque storage identifiers. The future attachment slice will commit explicit pending metadata, atomically rename staged files within the storage volume, mark them available, and reconcile interrupted pending operations after restart. Downloads always pass authorization and use attachment disposition with MIME-sniffing disabled.

## Tamper evidence and blockchain

Blockchain is deferred. One organization controls the application, database, server, and keys, so a blockchain would not introduce independent consensus. It also cannot make false source data true.

V1 uses append-only records, least-privileged runtime database access, immutable histories, SHA-256 attachment hashes, server timestamps, and tested backups. If an external verification requirement later appears, the system can calculate signed periodic Merkle-root manifests over audit and attachment hashes and store checkpoints outside the primary server. Only non-sensitive hashes would be candidates for public-chain anchoring.

Blockchain should be reconsidered only when multiple mutually distrustful organizations require shared writes or independent verification without trusting the application operator.
