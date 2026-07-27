CREATE TABLE public.update_comments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    progress_update_id uuid NOT NULL REFERENCES public.progress_updates (id) ON DELETE RESTRICT,
    content_markdown text NOT NULL,
    created_by uuid NOT NULL REFERENCES public.users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT update_comments_content_size CHECK (octet_length(content_markdown) BETWEEN 1 AND 20000)
);

CREATE INDEX update_comments_progress_time_idx
    ON public.update_comments (progress_update_id, created_at, id);

CREATE TABLE public.accepted_suggestions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    comment_id uuid NOT NULL UNIQUE REFERENCES public.update_comments (id) ON DELETE RESTRICT,
    accepted_by uuid NOT NULL REFERENCES public.users (id) ON DELETE RESTRICT,
    accepted_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX accepted_suggestions_time_idx
    ON public.accepted_suggestions (accepted_at, id);

CREATE TABLE public.task_assessments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id uuid NOT NULL REFERENCES public.tasks (id) ON DELETE RESTRICT,
    version bigint NOT NULL CHECK (version > 0),
    verdict text NOT NULL,
    remark_markdown text NOT NULL,
    assessed_by uuid NOT NULL REFERENCES public.users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT task_assessments_verdict CHECK (verdict IN ('on_track', 'needs_attention', 'blocked', 'complete')),
    CONSTRAINT task_assessments_remark_size CHECK (octet_length(remark_markdown) BETWEEN 1 AND 20000),
    UNIQUE (task_id, version)
);

CREATE INDEX task_assessments_current_idx
    ON public.task_assessments (task_id, version DESC);

CREATE FUNCTION public.reject_review_history_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'review history is append-only';
END;
$$;

CREATE TRIGGER update_comments_append_only
BEFORE UPDATE OR DELETE ON public.update_comments
FOR EACH ROW EXECUTE FUNCTION public.reject_review_history_mutation();

CREATE TRIGGER accepted_suggestions_append_only
BEFORE UPDATE OR DELETE ON public.accepted_suggestions
FOR EACH ROW EXECUTE FUNCTION public.reject_review_history_mutation();

CREATE TRIGGER task_assessments_append_only
BEFORE UPDATE OR DELETE ON public.task_assessments
FOR EACH ROW EXECUTE FUNCTION public.reject_review_history_mutation();
