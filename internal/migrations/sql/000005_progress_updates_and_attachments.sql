CREATE TABLE public.progress_updates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id uuid NOT NULL REFERENCES public.tasks (id) ON DELETE RESTRICT,
    content_markdown text NOT NULL,
    created_by uuid NOT NULL REFERENCES public.users (id) ON DELETE RESTRICT,
    idempotency_key text NOT NULL,
    request_sha256 char(64) NOT NULL,
    location_status text NOT NULL,
    location_reason text NOT NULL,
    reported_latitude numeric,
    reported_longitude numeric,
    reported_accuracy_metres numeric,
    browser_observed_at timestamptz,
    location_unavailable_reason text,
    geofence_id uuid REFERENCES public.project_geofences (id) ON DELETE RESTRICT,
    geofence_version bigint,
    geofence_latitude numeric,
    geofence_longitude numeric,
    geofence_radius_metres numeric,
    geofence_max_accuracy_metres numeric,
    computed_distance_metres numeric,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CONSTRAINT progress_content_size CHECK (octet_length(content_markdown) BETWEEN 1 AND 50000),
    CONSTRAINT progress_idempotency_size CHECK (length(idempotency_key) BETWEEN 16 AND 128),
    CONSTRAINT progress_request_hash_format CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT progress_location_status CHECK (location_status IN ('verified', 'unverified_outside', 'unverified_accuracy', 'unverified_no_geofence', 'unverified_unavailable', 'not_supplied')),
    CONSTRAINT progress_unavailable_reason CHECK (location_unavailable_reason IS NULL OR location_unavailable_reason IN ('permission_denied', 'timeout', 'unavailable', 'not_supported')),
    CONSTRAINT progress_location_reason_size CHECK (length(location_reason) BETWEEN 1 AND 160),
    CONSTRAINT progress_reported_location_complete CHECK (
        num_nonnulls(reported_latitude, reported_longitude, reported_accuracy_metres) = 0
        OR (
            num_nonnulls(reported_latitude, reported_longitude, reported_accuracy_metres) = 3
            AND reported_latitude BETWEEN -90 AND 90
            AND reported_longitude BETWEEN -180 AND 180
            AND reported_accuracy_metres BETWEEN 0.1 AND 100000
        )
    ),
    CONSTRAINT progress_geofence_snapshot_complete CHECK (
        num_nonnulls(geofence_id, geofence_version, geofence_latitude, geofence_longitude, geofence_radius_metres, geofence_max_accuracy_metres) = 0
        OR (
            num_nonnulls(geofence_id, geofence_version, geofence_latitude, geofence_longitude, geofence_radius_metres, geofence_max_accuracy_metres) = 6
            AND geofence_version > 0
            AND geofence_latitude BETWEEN -90 AND 90
            AND geofence_longitude BETWEEN -180 AND 180
            AND geofence_radius_metres > 0
            AND geofence_max_accuracy_metres > 0
        )
    ),
    CONSTRAINT progress_distance_nonnegative CHECK (computed_distance_metres IS NULL OR computed_distance_metres >= 0),
    CONSTRAINT progress_location_status_shape CHECK (
        (location_status IN ('verified', 'unverified_outside', 'unverified_accuracy') AND reported_latitude IS NOT NULL AND geofence_id IS NOT NULL AND computed_distance_metres IS NOT NULL AND location_unavailable_reason IS NULL)
        OR (location_status = 'unverified_no_geofence' AND reported_latitude IS NOT NULL AND geofence_id IS NULL AND computed_distance_metres IS NULL AND location_unavailable_reason IS NULL)
        OR (location_status = 'unverified_unavailable' AND reported_latitude IS NULL AND geofence_id IS NULL AND computed_distance_metres IS NULL AND location_unavailable_reason IS NOT NULL)
        OR (location_status = 'not_supplied' AND reported_latitude IS NULL AND geofence_id IS NULL AND computed_distance_metres IS NULL AND location_unavailable_reason IS NULL)
    ),
    UNIQUE (created_by, idempotency_key)
);

CREATE INDEX progress_updates_task_time_idx ON public.progress_updates (task_id, created_at, id);

CREATE TABLE public.progress_update_revisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    progress_update_id uuid NOT NULL REFERENCES public.progress_updates (id) ON DELETE RESTRICT,
    from_version bigint NOT NULL CHECK (from_version > 0),
    to_version bigint NOT NULL CHECK (to_version = from_version + 1),
    previous_content_markdown text NOT NULL,
    new_content_markdown text NOT NULL,
    edited_by uuid NOT NULL REFERENCES public.users (id) ON DELETE RESTRICT,
    edited_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT progress_revision_previous_size CHECK (octet_length(previous_content_markdown) BETWEEN 1 AND 50000),
    CONSTRAINT progress_revision_new_size CHECK (octet_length(new_content_markdown) BETWEEN 1 AND 50000),
    UNIQUE (progress_update_id, to_version)
);

CREATE INDEX progress_revisions_update_idx ON public.progress_update_revisions (progress_update_id, to_version);

CREATE TABLE public.progress_attachments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    progress_update_id uuid NOT NULL REFERENCES public.progress_updates (id) ON DELETE RESTRICT,
    original_name text NOT NULL,
    storage_key char(64) NOT NULL UNIQUE,
    reported_mime text NOT NULL DEFAULT '',
    detected_mime text NOT NULL,
    media_kind text NOT NULL,
    source text NOT NULL,
    verification_status text NOT NULL,
    verification_reason text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes > 0),
    sha256 char(64) NOT NULL,
    browser_last_modified_at timestamptz,
    embedded_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    storage_state text NOT NULL DEFAULT 'pending',
    failure_reason text NOT NULL DEFAULT '',
    uploaded_by uuid NOT NULL REFERENCES public.users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    available_at timestamptz,
    CONSTRAINT attachment_name_size CHECK (length(original_name) BETWEEN 1 AND 255),
    CONSTRAINT attachment_storage_key_format CHECK (storage_key ~ '^[0-9a-f]{64}$'),
    CONSTRAINT attachment_hash_format CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT attachment_media_kind CHECK (media_kind IN ('image', 'document', 'video')),
    CONSTRAINT attachment_source CHECK (source IN ('camera', 'upload')),
    CONSTRAINT attachment_camera_is_image CHECK (source <> 'camera' OR media_kind = 'image'),
    CONSTRAINT attachment_verification_status CHECK (verification_status IN ('verified', 'non_verified')),
    CONSTRAINT attachment_verified_shape CHECK (verification_status <> 'verified' OR (source = 'camera' AND media_kind = 'image')),
    CONSTRAINT attachment_verification_reason_size CHECK (length(verification_reason) BETWEEN 1 AND 160),
    CONSTRAINT attachment_metadata_object CHECK (jsonb_typeof(embedded_metadata) = 'object'),
    CONSTRAINT attachment_storage_state CHECK (storage_state IN ('pending', 'available', 'failed')),
    CONSTRAINT attachment_availability_consistent CHECK ((storage_state = 'available') = (available_at IS NOT NULL)),
    CONSTRAINT attachment_failure_reason_consistent CHECK (
        (storage_state = 'failed' AND length(failure_reason) BETWEEN 1 AND 160)
        OR (storage_state <> 'failed' AND failure_reason = '')
    )
);

CREATE INDEX progress_attachments_update_idx ON public.progress_attachments (progress_update_id, created_at, id);
CREATE INDEX progress_attachments_pending_idx ON public.progress_attachments (storage_state, created_at) WHERE storage_state = 'pending';
