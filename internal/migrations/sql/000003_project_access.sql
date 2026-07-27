CREATE TABLE public.projects (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    description_markdown text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    created_by uuid NOT NULL REFERENCES public.users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CONSTRAINT projects_name_trimmed CHECK (name = btrim(name) AND length(name) BETWEEN 1 AND 120),
    CONSTRAINT projects_description_size CHECK (octet_length(description_markdown) <= 20000)
);

CREATE INDEX projects_active_name_idx ON public.projects (active, lower(name), id);

CREATE TABLE public.project_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES public.projects (id) ON DELETE RESTRICT,
    user_id uuid NOT NULL REFERENCES public.users (id) ON DELETE RESTRICT,
    added_by uuid NOT NULL REFERENCES public.users (id) ON DELETE RESTRICT,
    added_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    removed_by uuid REFERENCES public.users (id) ON DELETE RESTRICT,
    removed_at timestamptz,
    CONSTRAINT project_members_removal_complete CHECK (
        (removed_at IS NULL AND removed_by IS NULL)
        OR (removed_at IS NOT NULL AND removed_by IS NOT NULL AND removed_at >= added_at)
    )
);

CREATE UNIQUE INDEX project_members_current_unique
    ON public.project_members (project_id, user_id)
    WHERE removed_at IS NULL;
CREATE INDEX project_members_user_current_idx
    ON public.project_members (user_id, project_id)
    WHERE removed_at IS NULL;

CREATE TABLE public.project_geofences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES public.projects (id) ON DELETE RESTRICT,
    version bigint NOT NULL CHECK (version > 0),
    latitude numeric(9, 6) NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude numeric(9, 6) NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    radius_metres numeric(10, 2) NOT NULL CHECK (radius_metres BETWEEN 1 AND 100000),
    max_accuracy_metres numeric(10, 2) NOT NULL CHECK (max_accuracy_metres BETWEEN 0.1 AND 10000),
    created_by uuid NOT NULL REFERENCES public.users (id) ON DELETE RESTRICT,
    valid_from timestamptz NOT NULL DEFAULT clock_timestamp(),
    valid_to timestamptz,
    CONSTRAINT project_geofences_valid_period CHECK (valid_to IS NULL OR valid_to >= valid_from),
    UNIQUE (project_id, version)
);

CREATE UNIQUE INDEX project_geofences_current_unique
    ON public.project_geofences (project_id)
    WHERE valid_to IS NULL;
CREATE INDEX project_geofences_history_idx
    ON public.project_geofences (project_id, version DESC);
