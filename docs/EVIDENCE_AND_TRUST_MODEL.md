# Evidence and trust model

## Purpose and limitation

The verified-update flow records useful operational evidence for internal project oversight. Browser camera and geolocation APIs do not provide cryptographic attestation and can be manipulated by a compromised device, browser, operating system, developer tools, or user. The product must never describe evidence as tamper-proof, court-grade, or proof of physical presence.

## Upload-location and attachment-verification algorithm

For any file upload, the browser supplies a current location result. Text-only progress may instead report that location was unavailable or omit it. The server then:

1. authenticates the actor and verifies current project membership or Admin access;
2. loads the task, project, and current geofence from PostgreSQL;
3. validates finite latitude/longitude values and a positive finite accuracy;
4. records whether accuracy exceeds the configured maximum without rejecting the update or file;
5. calculates great-circle distance with the Haversine formula using Earth radius 6,371,008.8 metres;
6. considers the location inside only when `distance + reported_accuracy <= geofence_radius`;
7. requires coordinates for every request containing file bytes and associates every file with the same immutable upload-location snapshot;
8. stores the server receipt time as authoritative;
9. stores reported location, accuracy, optional browser time, geofence centre/radius/accuracy threshold, computed distance, result, and reason as one immutable evidence snapshot;
10. labels an attachment verified only when it is an image or video reported as an in-Chrome direct camera capture and the location result passes both accuracy and geofence checks.

Adding reported accuracy to distance is conservative: the reported uncertainty circle must fit inside the geofence. Boundary equality passes. Client-side display is only a preview; the server recomputes the decision.

Uploaded/gallery images, documents, and videos are always non-verified regardless of coordinates. Direct camera photos and videos with inaccurate, outside, or no-geofence location are non-verified but accepted and geotagged. Missing coordinates reject file bytes but do not reject a text-only update. Storage failure produces pending/failed attachment state rather than a false successful file.

## Trust classifications

- **Server authoritative:** server receipt/upload/edit time, authenticated actor, authorization decision, server-computed SHA-256, server-detected MIME, stored geofence snapshot, computed distance, and verification result.
- **Browser reported:** coordinates, accuracy, browser-observed time, and the assertion that photo/video media came through the active application camera flow.
- **Untrusted embedded metadata:** EXIF capture time, EXIF GPS, device/camera model, and any metadata supplied inside a file.
- **Existing upload:** bytes selected from device storage. The upload geotag is stored, but the file remains non-verified because origin/time are not attested.

Even the “in-application camera” source is application-flow evidence rather than hardware attestation.

## Attachment integrity and recovery

The server streams uploads to a staging directory while enforcing size/type limits and calculating SHA-256. Metadata records use opaque storage identifiers. Step 06 commits explicit pending metadata, atomically renames staged files within the storage volume, marks them available, and reconciles interrupted pending operations immediately and every minute. Content reads always pass authorization and disable MIME sniffing; videos use inline byte-range streaming while images/documents use attachment disposition.

## Tamper evidence and blockchain

Blockchain is deferred. One organization controls the application, database, server, and keys, so a blockchain would not introduce independent consensus. It also cannot make false source data true.

V1 uses append-only records, least-privileged runtime database access, immutable histories, SHA-256 attachment hashes, server timestamps, and tested backups. If an external verification requirement later appears, the system can calculate signed periodic Merkle-root manifests over audit and attachment hashes and store checkpoints outside the primary server. Only non-sensitive hashes would be candidates for public-chain anchoring.

Blockchain should be reconsidered only when multiple mutually distrustful organizations require shared writes or independent verification without trusting the application operator.
