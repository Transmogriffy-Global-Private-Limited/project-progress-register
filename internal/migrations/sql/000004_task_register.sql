CREATE TABLE public.tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES public.projects (id) ON DELETE RESTRICT,
    name text NOT NULL,
    goals_markdown text NOT NULL DEFAULT '',
    description_markdown text NOT NULL DEFAULT '',
    created_by uuid NOT NULL REFERENCES public.users (id) ON DELETE RESTRICT,
    responsible_user_id uuid REFERENCES public.users (id) ON DELETE RESTRICT,
    target_date date,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CONSTRAINT tasks_name_trimmed CHECK (name = btrim(name) AND length(name) BETWEEN 1 AND 160),
    CONSTRAINT tasks_goals_size CHECK (octet_length(goals_markdown) <= 20000),
    CONSTRAINT tasks_description_size CHECK (octet_length(description_markdown) <= 50000)
);

CREATE INDEX tasks_project_name_idx ON public.tasks (project_id, lower(name), id);
CREATE INDEX tasks_creator_idx ON public.tasks (created_by, project_id);
CREATE INDEX tasks_responsible_idx ON public.tasks (responsible_user_id, project_id) WHERE responsible_user_id IS NOT NULL;
