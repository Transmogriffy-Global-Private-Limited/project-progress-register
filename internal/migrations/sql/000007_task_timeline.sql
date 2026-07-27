CREATE TABLE public.task_revisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id uuid NOT NULL REFERENCES public.tasks (id) ON DELETE RESTRICT,
    from_version bigint NOT NULL CHECK (from_version > 0),
    to_version bigint NOT NULL CHECK (to_version = from_version + 1),
    previous_name text NOT NULL,
    previous_goals_markdown text NOT NULL,
    previous_description_markdown text NOT NULL,
    previous_responsible_user_id uuid REFERENCES public.users (id) ON DELETE RESTRICT,
    previous_target_date date,
    new_name text NOT NULL,
    new_goals_markdown text NOT NULL,
    new_description_markdown text NOT NULL,
    new_responsible_user_id uuid REFERENCES public.users (id) ON DELETE RESTRICT,
    new_target_date date,
    change_reason text NOT NULL CHECK (change_reason IN ('user_edit', 'membership_removed')),
    edited_by uuid NOT NULL REFERENCES public.users (id) ON DELETE RESTRICT,
    edited_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (task_id, to_version)
);

CREATE INDEX task_revisions_task_version_idx
    ON public.task_revisions (task_id, to_version);

CREATE FUNCTION public.reject_task_revision_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'task revisions are append-only';
END;
$$;

CREATE TRIGGER task_revisions_append_only
BEFORE UPDATE OR DELETE ON public.task_revisions
FOR EACH ROW EXECUTE FUNCTION public.reject_task_revision_mutation();
