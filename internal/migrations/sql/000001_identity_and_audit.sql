CREATE TABLE public.users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username text NOT NULL,
    email text NOT NULL,
    password_hash text NOT NULL,
    role text NOT NULL CHECK (role IN ('admin', 'member')),
    enabled boolean NOT NULL DEFAULT true,
    created_by uuid REFERENCES public.users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    password_changed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    last_login_at timestamptz,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CONSTRAINT users_username_normalized CHECK (
        username = lower(btrim(username))
        AND username ~ '^[a-z0-9][a-z0-9._-]{2,31}$'
    ),
    CONSTRAINT users_email_normalized CHECK (
        email = lower(btrim(email))
        AND length(email) BETWEEN 3 AND 254
    ),
    CONSTRAINT users_password_hash_present CHECK (length(password_hash) BETWEEN 32 AND 512)
);

CREATE UNIQUE INDEX users_username_unique ON public.users (username);
CREATE UNIQUE INDEX users_email_unique ON public.users (email);
CREATE INDEX users_enabled_role_idx ON public.users (enabled, role);

CREATE TABLE public.sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_ip inet NOT NULL,
    user_agent text NOT NULL DEFAULT '' CHECK (length(user_agent) <= 512),
    CONSTRAINT sessions_expiry_after_creation CHECK (expires_at > created_at),
    CONSTRAINT sessions_revocation_after_creation CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX sessions_user_active_idx ON public.sessions (user_id, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX sessions_expiry_idx ON public.sessions (expires_at);

CREATE TABLE public.login_throttles (
    identifier_hash bytea NOT NULL CHECK (octet_length(identifier_hash) = 32),
    client_ip inet NOT NULL,
    window_started_at timestamptz NOT NULL,
    failure_count integer NOT NULL CHECK (failure_count >= 0),
    blocked_until timestamptz,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (identifier_hash, client_ip)
);

CREATE INDEX login_throttles_blocked_idx ON public.login_throttles (blocked_until) WHERE blocked_until IS NOT NULL;

CREATE TABLE public.audit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id uuid REFERENCES public.users (id) ON DELETE RESTRICT,
    action text NOT NULL CHECK (length(action) BETWEEN 3 AND 100),
    target_type text NOT NULL CHECK (length(target_type) BETWEEN 3 AND 50),
    target_id uuid,
    outcome text NOT NULL CHECK (outcome IN ('succeeded', 'failed', 'denied')),
    occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    request_id text NOT NULL CHECK (length(request_id) BETWEEN 8 AND 64),
    client_ip inet NOT NULL,
    user_agent text NOT NULL DEFAULT '' CHECK (length(user_agent) <= 512),
    details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details) = 'object')
);

CREATE INDEX audit_events_occurred_idx ON public.audit_events (occurred_at DESC);
CREATE INDEX audit_events_actor_idx ON public.audit_events (actor_user_id, occurred_at DESC) WHERE actor_user_id IS NOT NULL;
CREATE INDEX audit_events_target_idx ON public.audit_events (target_type, target_id, occurred_at DESC) WHERE target_id IS NOT NULL;
CREATE INDEX audit_events_action_idx ON public.audit_events (action, occurred_at DESC);

CREATE FUNCTION public.ppr_reject_audit_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only';
END;
$$;

CREATE TRIGGER audit_events_reject_update
BEFORE UPDATE ON public.audit_events
FOR EACH ROW EXECUTE FUNCTION public.ppr_reject_audit_mutation();

CREATE TRIGGER audit_events_reject_delete
BEFORE DELETE ON public.audit_events
FOR EACH ROW EXECUTE FUNCTION public.ppr_reject_audit_mutation();

REVOKE UPDATE, DELETE ON public.audit_events FROM PUBLIC;
