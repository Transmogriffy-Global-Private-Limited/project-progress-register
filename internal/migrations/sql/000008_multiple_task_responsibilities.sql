CREATE TABLE public.task_responsibilities (
    task_id uuid NOT NULL REFERENCES public.tasks (id) ON DELETE RESTRICT,
    user_id uuid NOT NULL REFERENCES public.users (id) ON DELETE RESTRICT,
    assigned_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (task_id, user_id)
);

CREATE INDEX task_responsibilities_user_task_idx
    ON public.task_responsibilities (user_id, task_id);

INSERT INTO public.task_responsibilities (task_id, user_id, assigned_at)
SELECT id, responsible_user_id, updated_at
FROM public.tasks
WHERE responsible_user_id IS NOT NULL;

ALTER TABLE public.task_revisions
    ADD COLUMN previous_responsible_user_ids uuid[] NOT NULL DEFAULT '{}'::uuid[],
    ADD COLUMN new_responsible_user_ids uuid[] NOT NULL DEFAULT '{}'::uuid[];

DROP TRIGGER task_revisions_append_only ON public.task_revisions;

UPDATE public.task_revisions
SET previous_responsible_user_ids = CASE
        WHEN previous_responsible_user_id IS NULL THEN '{}'::uuid[]
        ELSE ARRAY[previous_responsible_user_id]
    END,
    new_responsible_user_ids = CASE
        WHEN new_responsible_user_id IS NULL THEN '{}'::uuid[]
        ELSE ARRAY[new_responsible_user_id]
    END;

ALTER TABLE public.task_revisions
    DROP COLUMN previous_responsible_user_id,
    DROP COLUMN new_responsible_user_id;

CREATE TRIGGER task_revisions_append_only
BEFORE UPDATE OR DELETE ON public.task_revisions
FOR EACH ROW EXECUTE FUNCTION public.reject_task_revision_mutation();

DROP INDEX public.tasks_responsible_idx;

ALTER TABLE public.tasks
    DROP COLUMN responsible_user_id;
