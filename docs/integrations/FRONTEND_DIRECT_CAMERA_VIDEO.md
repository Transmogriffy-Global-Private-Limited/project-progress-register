# Frontend integration — direct camera photo/video evidence and streaming

Status: Backend implementation verified locally; migration `000009` and deployment pending

This is the focused frontend integration guide for capturing a photo or video directly through the browser, uploading it with the mandatory upload location, rendering the backend verification result, and streaming an authorized stored video. It supplements the complete [`FRONTEND_INTEGRATION.md`](FRONTEND_INTEGRATION.md) handoff and the authoritative [`api/openapi/v1/openapi.yaml`](../../api/openapi/v1/openapi.yaml).

Do not integrate this against production until the backend migration and binary are deployed. Existing production behavior rejects camera-source video.

## 1. Contract summary

No new endpoint or response type is introduced. Use:

```text
POST /api/v1/projects/{project_id}/tasks/{task_id}/progress-updates
GET  /api/v1/projects/{project_id}/tasks/{task_id}/progress-updates/{update_id}/attachments/{attachment_id}/content
```

The POST is the existing ordered multipart operation `createTaskProgressUpdate`. The GET remains `downloadProgressAttachment` for compatibility, but its implemented behavior supports authorized inline video streaming and byte ranges.

Production uses the `/backend` prefix. Keep one configurable browser base path:

```ts
export const API_BASE_PATH = "/backend"; // use "" for an unprefixed local backend
```

For local Vite development, proxy `/backend` to the hosted/backend origin without rewriting the application prefix. Browser calls must remain root-relative and same-origin from the frontend's point of view so the protected session cookie accompanies API and `<video>` requests.

## 2. Trust and verification rule

`source: "camera"` is a browser-reported application-flow assertion. It is operational evidence, not hardware attestation.

The backend returns the authoritative result:

| Media flow | Location result | Attachment result |
| --- | --- | --- |
| Direct camera photo | `verified` | `verified` |
| Direct camera video | `verified` | `verified` |
| Direct camera photo/video | outside, inaccurate, or no geofence | `non_verified` |
| Gallery/existing photo/video | any | `non_verified` |
| Document | any | `non_verified` |

Every file requires current browser coordinates at upload time. Geofence failure does not reject otherwise valid bytes; it changes the evidence label. Missing coordinates reject a request containing files.

Render the exact backend `verification_status`, `verification_reason`, `location_status`, and `location_reason`. Never infer or override them in the frontend.

## 3. Capture controls

Keep camera capture separate from gallery/file selection:

```html
<input id="camera-photo" type="file" accept="image/*" capture="environment">
<input id="camera-video" type="file" accept="video/*" capture="environment">
<input id="existing-files" type="file" multiple>
```

Browser support for `capture` varies. Label these as “Take photo” and “Record video”, but treat the resulting source as browser-reported. If a platform does not offer direct capture, do not silently label a normal picker result as camera media.

Map the selected `File` to its source at the moment it enters the draft:

```ts
type AttachmentSource = "camera" | "upload";

interface PendingAttachment {
  id: string; // frontend-only stable key
  file: File;
  source: AttachmentSource;
}

function fromDirectCamera(file: File): PendingAttachment {
  return { id: crypto.randomUUID(), file, source: "camera" };
}

function fromExistingFile(file: File): PendingAttachment {
  return { id: crypto.randomUUID(), file, source: "upload" };
}
```

Documents must always use `upload`. The backend permits `camera` only when server-detected media kind is image or video.

## 4. Obtain the upload location

Obtain location immediately before submission rather than trusting old draft coordinates:

```ts
interface ReportedLocation {
  latitude: number;
  longitude: number;
  accuracy_metres: number;
  browser_observed_at: string | null;
}

function currentUploadLocation(): Promise<ReportedLocation> {
  return new Promise((resolve, reject) => {
    navigator.geolocation.getCurrentPosition(
      ({ coords, timestamp }) => resolve({
        latitude: coords.latitude,
        longitude: coords.longitude,
        accuracy_metres: coords.accuracy,
        browser_observed_at: new Date(timestamp).toISOString(),
      }),
      reject,
      { enableHighAccuracy: true, timeout: 15_000, maximumAge: 0 },
    );
  });
}
```

If location fails, retain the media draft locally and let the user retry. Do not submit file bytes with `location_unavailable_reason`; that field is only usable for text-only progress.

## 5. Ordered multipart upload

Authentication remains the opaque session cookie. Send `credentials: "include"`, `X-CSRF-Token`, and one stable 16-128 character `Idempotency-Key`. Do not use JWT.

The `metadata` part must be ordinary text. Do not append a JSON `Blob` or `File` because Go correctly treats any part with a filename as a file part.

```ts
interface CreateProgressMetadata {
  content_markdown: string;
  location: ReportedLocation;
  location_unavailable_reason: null;
  attachments: Array<{
    source: AttachmentSource;
    browser_last_modified_at: string | null;
  }>;
}

async function submitCameraProgress(
  projectId: string,
  taskId: string,
  contentMarkdown: string,
  pending: PendingAttachment[],
  csrfToken: string,
  idempotencyKey: string,
): Promise<ProgressUpdate> {
  const location = await currentUploadLocation();
  const metadata: CreateProgressMetadata = {
    content_markdown: contentMarkdown,
    location,
    location_unavailable_reason: null,
    attachments: pending.map(({ file, source }) => ({
      source,
      browser_last_modified_at:
        Number.isFinite(file.lastModified) && file.lastModified > 0
          ? new Date(file.lastModified).toISOString()
          : null,
    })),
  };

  const form = new FormData();
  form.append("metadata", JSON.stringify(metadata));
  for (const item of pending) {
    form.append("files", item.file, item.file.name);
  }

  const response = await fetch(
    `${API_BASE_PATH}/api/v1/projects/${projectId}/tasks/${taskId}/progress-updates`,
    {
      method: "POST",
      credentials: "include",
      headers: {
        "X-CSRF-Token": csrfToken,
        "Idempotency-Key": idempotencyKey,
      },
      body: form,
    },
  );

  const body = await response.json();
  if (!response.ok) throw body.error;
  return body.progress_update as ProgressUpdate;
}
```

The descriptor array and repeated `files` parts must have identical lengths and order. Never manually set multipart `Content-Type`; the browser supplies the boundary.

The backend streams/spools bounded multipart files to private staging storage, detects their MIME from bytes, calculates SHA-256, and atomically finalizes them. Default limits remain 10 files and 100 MiB per file; always handle `413 request_too_large`.

## 6. Success state

Replace optimistic media state with the returned attachment objects:

```ts
interface ProgressAttachment {
  id: string;
  original_name: string;
  detected_mime: string;
  media_kind: "image" | "document" | "video";
  source: "camera" | "upload";
  verification_status: "verified" | "non_verified";
  verification_reason: string;
  storage_state: "pending" | "available" | "failed";
  content_path: string;
}
```

Do not synthesize verification from the selected source or client-side distance. Use the response even when it differs from preview expectations.

## 7. Stream authorized video

Use the backend-provided `attachment.content_path`; never reconstruct nested IDs and never expose storage keys.

For same-origin or correctly proxied FE delivery:

```tsx
function EvidenceVideo({ attachment }: { attachment: ProgressAttachment }) {
  if (attachment.media_kind !== "video" || attachment.storage_state !== "available") {
    return null;
  }

  return (
    <video
      controls
      preload="metadata"
      src={attachment.content_path}
      playsInline
    />
  );
}
```

The browser sends range requests as needed. The backend returns:

- detected video `Content-Type`;
- `Content-Disposition: inline`;
- `Accept-Ranges: bytes`;
- `206 Partial Content` plus `Content-Range` for valid ranges;
- `Cache-Control: private, no-store`;
- `X-Content-Type-Options: nosniff`.

Do not fetch the complete video into a Blob before playback. That defeats seeking and increases browser memory usage. Use an authenticated Blob download only for an explicit “Save file” action.

## 8. Retry and failure behavior

- Network failure or `503 attachment_pending`: retry byte-identical metadata/files with the same idempotency key.
- Changed content, location, sources, order, or files: create a new idempotency key.
- `401 unauthenticated`: recover/login, then refetch the progress update before playback.
- `403 csrf_invalid`: recover the session CSRF token; require a safe user retry of the mutation.
- `404 not_found`: treat wrong-parent, inaccessible, and absent attachment identically.
- `410 attachment_unavailable`: metadata remains but bytes cannot be served; show an unavailable state.
- `413 request_too_large`: retain the draft and require a smaller/shorter video.
- `415 unsupported_media_type`: the server-detected bytes are not allowlisted.
- `422 validation_failed`: show the safe backend message, including a rejected camera-source document.

If `<video>` reports a playback error, refetch its containing progress update. Do not repeatedly reconstruct or probe alternate content URLs.

## 9. Compatibility and rollout

The route and JSON shapes remain V1-compatible. The changed behavior is additive after migration `000009`:

1. Deploy migration `000009` and the matching backend binary together.
2. Confirm the hosted OpenAPI describes camera image/video verification and `206` delivery.
3. Enable the separate video capture control.
4. Submit direct camera video with `source: "camera"`.
5. Render the returned verification result.
6. Stream from returned `content_path`.

Before backend deployment, keep the video capture control disabled or send selected video as `source: "upload"`; old backend versions reject camera-source video.

## 10. FE acceptance tests

1. Direct camera photo plus verified location returns `verified`.
2. Direct camera video plus verified location returns `verified`.
3. Direct camera video outside/inaccurate location is accepted as `non_verified`.
4. Gallery video always sends `source: "upload"` and remains `non_verified`.
5. Camera-source document validation preserves the unsaved draft.
6. Missing location blocks file submission in the UI.
7. Descriptor/file ordering survives mixed photo, video, and document submission.
8. Retry after network/`503` reuses identical bytes and idempotency key.
9. Video element uses returned `content_path` and receives a `206` range response while seeking.
10. Unauthorized, unavailable, and pending playback states do not leak storage details.
11. Explicit file save still works for video without replacing streaming playback.
12. Existing image/document download behavior remains unchanged.
