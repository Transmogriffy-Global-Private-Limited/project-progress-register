package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const bootstrapAdvisoryLock int64 = 710274602

// PostgresRepository persists identity transitions and their audit events.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository constructs the PostgreSQL identity store.
func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("PostgreSQL pool is required")
	}
	return &PostgresRepository{pool: pool}, nil
}

func (r *PostgresRepository) UserCount(ctx context.Context) (int, error) {
	var count int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM public.users`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

func (r *PostgresRepository) BootstrapAdmin(ctx context.Context, input NewUser, event AuditEvent) (User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin bootstrap transaction: %w", err)
	}
	defer rollback(tx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, bootstrapAdvisoryLock); err != nil {
		return User{}, fmt.Errorf("lock bootstrap: %w", err)
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.users)`).Scan(&exists); err != nil {
		return User{}, fmt.Errorf("inspect bootstrap state: %w", err)
	}
	if exists {
		return User{}, ErrBootstrapUnavailable
	}
	var user User
	if err := tx.QueryRow(ctx, `
		INSERT INTO public.users (username, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, username, email, role, enabled, must_change_password, created_at, updated_at, version`, input.Username, input.Email, input.PasswordHash, input.Role).Scan(
		&user.ID, &user.Username, &user.Email, &user.Role, &user.Enabled, &user.MustChangePassword, &user.CreatedAt, &user.UpdatedAt, &user.Version,
	); err != nil {
		return User{}, fmt.Errorf("insert bootstrap Admin: %w", err)
	}
	event.ActorUserID = user.ID
	event.TargetID = user.ID
	if err := insertAudit(ctx, tx, event); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit bootstrap: %w", err)
	}
	return user, nil
}

func (r *PostgresRepository) FindUserByIdentifier(ctx context.Context, identifier string) (UserRecord, error) {
	var record UserRecord
	if err := r.pool.QueryRow(ctx, `
		SELECT id::text, username, email, role, enabled, must_change_password, created_at, updated_at, version, password_hash
		FROM public.users
		WHERE username = $1 OR email = $1
		LIMIT 1`, identifier).Scan(
		&record.ID, &record.Username, &record.Email, &record.Role, &record.Enabled, &record.MustChangePassword, &record.CreatedAt, &record.UpdatedAt, &record.Version, &record.PasswordHash,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserRecord{}, ErrNotFound
		}
		return UserRecord{}, fmt.Errorf("query user: %w", err)
	}
	return record, nil
}

func (r *PostgresRepository) IsLoginBlocked(ctx context.Context, identifierHash []byte, clientIP string, now time.Time) (bool, error) {
	var blocked bool
	if err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(blocked_until > $3, false)
		FROM public.login_throttles
		WHERE identifier_hash = $1 AND client_ip = $2::inet`, identifierHash, clientIP, now).Scan(&blocked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("query login throttle: %w", err)
	}
	return blocked, nil
}

func (r *PostgresRepository) RecordLoginFailure(ctx context.Context, identifierHash []byte, clientIP string, now time.Time, window time.Duration, maximum int, block time.Duration, event AuditEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin login failure transaction: %w", err)
	}
	defer rollback(tx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO public.login_throttles (identifier_hash, client_ip, window_started_at, failure_count)
		VALUES ($1, $2::inet, $3, 0)
		ON CONFLICT (identifier_hash, client_ip) DO NOTHING`, identifierHash, clientIP, now); err != nil {
		return fmt.Errorf("initialize login throttle: %w", err)
	}
	var windowStarted time.Time
	var failureCount int
	var blockedUntil *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT window_started_at, failure_count, blocked_until
		FROM public.login_throttles
		WHERE identifier_hash = $1 AND client_ip = $2::inet
		FOR UPDATE`, identifierHash, clientIP).Scan(&windowStarted, &failureCount, &blockedUntil); err != nil {
		return fmt.Errorf("lock login throttle: %w", err)
	}
	if !now.Before(windowStarted.Add(window)) {
		windowStarted = now
		failureCount = 0
		blockedUntil = nil
	}
	failureCount++
	if failureCount >= maximum {
		until := now.Add(block)
		if blockedUntil == nil || until.After(*blockedUntil) {
			blockedUntil = &until
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE public.login_throttles
		SET window_started_at = $3, failure_count = $4, blocked_until = $5, updated_at = clock_timestamp()
		WHERE identifier_hash = $1 AND client_ip = $2::inet`, identifierHash, clientIP, windowStarted, failureCount, blockedUntil); err != nil {
		return fmt.Errorf("update login throttle: %w", err)
	}
	if err := insertAudit(ctx, tx, event); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit login failure: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CreateLoginSession(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time, identifierHash []byte, clientIP string, event AuditEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin login session transaction: %w", err)
	}
	defer rollback(tx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO public.sessions (user_id, token_hash, expires_at, created_ip, user_agent)
		VALUES ($1::uuid, $2, $3, $4::inet, $5)`, userID, tokenHash, expiresAt, clientIP, event.Context.UserAgent); err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE public.users SET last_login_at = clock_timestamp() WHERE id = $1::uuid`, userID); err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM public.login_throttles WHERE identifier_hash = $1 AND client_ip = $2::inet`, identifierHash, clientIP); err != nil {
		return fmt.Errorf("clear login throttle: %w", err)
	}
	if err := insertAudit(ctx, tx, event); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit login session: %w", err)
	}
	return nil
}

func (r *PostgresRepository) LookupSession(ctx context.Context, tokenHash []byte, now time.Time) (Session, error) {
	var session Session
	if err := r.pool.QueryRow(ctx, `
		SELECT s.id::text, s.expires_at, u.id::text, u.username, u.email, u.role, u.enabled, u.must_change_password, u.created_at, u.updated_at, u.version
		FROM public.sessions s
		JOIN public.users u ON u.id = s.user_id
		WHERE s.token_hash = $1
		  AND s.revoked_at IS NULL
		  AND s.expires_at > $2
		  AND u.enabled = true`, tokenHash, now).Scan(
		&session.ID, &session.ExpiresAt, &session.User.ID, &session.User.Username, &session.User.Email, &session.User.Role, &session.User.Enabled, &session.User.MustChangePassword, &session.User.CreatedAt, &session.User.UpdatedAt, &session.User.Version,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrUnauthenticated
		}
		return Session{}, fmt.Errorf("query session: %w", err)
	}
	return session, nil
}

func (r *PostgresRepository) RevokeSession(ctx context.Context, tokenHash []byte, event AuditEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin logout transaction: %w", err)
	}
	defer rollback(tx)
	result, err := tx.Exec(ctx, `UPDATE public.sessions SET revoked_at = clock_timestamp() WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if result.RowsAffected() > 0 {
		if err := insertAudit(ctx, tx, event); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit logout: %w", err)
	}
	return nil
}

func (r *PostgresRepository) AppendAudit(ctx context.Context, event AuditEvent) error {
	return insertAudit(ctx, r.pool, event)
}

func (r *PostgresRepository) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, username, email, role, enabled, must_change_password, created_at, updated_at, version
		FROM public.users
		ORDER BY username, id`)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()
	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.Enabled, &user.MustChangePassword, &user.CreatedAt, &user.UpdatedAt, &user.Version); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

func (r *PostgresRepository) ListIdentityAudit(ctx context.Context, limit int) ([]AuditRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id::text, COALESCE(a.actor_user_id::text, ''), COALESCE(u.username, ''),
		       a.action, a.target_type, COALESCE(a.target_id::text, ''), a.outcome,
		       a.occurred_at, a.request_id, host(a.client_ip), a.user_agent, a.details
		FROM public.audit_events a
		LEFT JOIN public.users u ON u.id = a.actor_user_id
		WHERE a.action LIKE 'identity.%' OR a.action LIKE 'auth.%' OR a.action = 'authorization.admin_users_denied'
		ORDER BY a.occurred_at DESC, a.id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query identity audit: %w", err)
	}
	defer rows.Close()
	records := make([]AuditRecord, 0)
	for rows.Next() {
		var record AuditRecord
		var details []byte
		if err := rows.Scan(&record.ID, &record.ActorUserID, &record.ActorUsername, &record.Action, &record.TargetType, &record.TargetID, &record.Outcome, &record.OccurredAt, &record.RequestID, &record.ClientIP, &record.UserAgent, &details); err != nil {
			return nil, fmt.Errorf("scan identity audit: %w", err)
		}
		if err := json.Unmarshal(details, &record.Details); err != nil {
			return nil, fmt.Errorf("decode identity audit details: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate identity audit: %w", err)
	}
	return records, nil
}

func (r *PostgresRepository) ListAudit(ctx context.Context, query auditPersistenceQuery) ([]AuditRecord, error) {
	var cursorTime *time.Time
	cursorID := ""
	if query.Cursor != nil {
		cursorTime = &query.Cursor.OccurredAt
		cursorID = query.Cursor.ID
	}
	rows, err := r.pool.Query(ctx, `
		SELECT a.id::text,COALESCE(a.actor_user_id::text,''),COALESCE(u.username,''),
		       a.action,a.target_type,COALESCE(a.target_id::text,''),a.outcome,
		       a.occurred_at,a.request_id,host(a.client_ip),a.user_agent,a.details
		FROM public.audit_events a
		LEFT JOIN public.users u ON u.id=a.actor_user_id
		WHERE ($1='' OR a.action=$1)
		  AND ($2='' OR a.outcome=$2)
		  AND ($3='' OR a.actor_user_id::text=$3)
		  AND ($4='' OR a.target_type=$4)
		  AND ($5::timestamptz IS NULL OR a.occurred_at<$5 OR (a.occurred_at=$5 AND a.id::text<$6))
		ORDER BY a.occurred_at DESC,a.id DESC
		LIMIT $7`, query.Action, query.Outcome, query.ActorUserID, query.TargetType, cursorTime, cursorID, query.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("query complete audit: %w", err)
	}
	defer rows.Close()
	records := []AuditRecord{}
	for rows.Next() {
		var record AuditRecord
		var details []byte
		if err := rows.Scan(&record.ID, &record.ActorUserID, &record.ActorUsername, &record.Action, &record.TargetType, &record.TargetID, &record.Outcome, &record.OccurredAt, &record.RequestID, &record.ClientIP, &record.UserAgent, &details); err != nil {
			return nil, fmt.Errorf("scan complete audit: %w", err)
		}
		if err := json.Unmarshal(details, &record.Details); err != nil {
			return nil, fmt.Errorf("decode complete audit details: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate complete audit: %w", err)
	}
	return records, nil
}

func (r *PostgresRepository) CreateUser(ctx context.Context, input NewUser, event AuditEvent) (User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin create user transaction: %w", err)
	}
	defer rollback(tx)
	var user User
	err = tx.QueryRow(ctx, `
		INSERT INTO public.users (username, email, password_hash, role, created_by, must_change_password)
		VALUES ($1, $2, $3, $4, $5::uuid, $6)
		RETURNING id::text, username, email, role, enabled, must_change_password, created_at, updated_at, version`,
		input.Username, input.Email, input.PasswordHash, input.Role, input.CreatedBy, input.MustChangePassword,
	).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.Enabled, &user.MustChangePassword, &user.CreatedAt, &user.UpdatedAt, &user.Version)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrConflict
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	event.TargetID = user.ID
	if err := insertAudit(ctx, tx, event); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit create user: %w", err)
	}
	return user, nil
}

func (r *PostgresRepository) UpdateUser(ctx context.Context, targetID string, input UpdateUserInput, event AuditEvent) (User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin update user transaction: %w", err)
	}
	defer rollback(tx)
	enabledAdminCount, err := lockEnabledAdmins(ctx, tx)
	if err != nil {
		return User{}, err
	}
	var current User
	if err := tx.QueryRow(ctx, `
		SELECT id::text, username, email, role, enabled, must_change_password, created_at, updated_at, version
		FROM public.users WHERE id = $1::uuid FOR UPDATE`, targetID).Scan(
		&current.ID, &current.Username, &current.Email, &current.Role, &current.Enabled, &current.MustChangePassword, &current.CreatedAt, &current.UpdatedAt, &current.Version,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("lock user: %w", err)
	}
	if current.Version != input.ExpectedVersion {
		return User{}, ErrConflict
	}
	removesEnabledAdmin := current.Enabled && current.Role == RoleAdmin && (!*input.Enabled || input.Role != RoleAdmin)
	if removesEnabledAdmin && enabledAdminCount <= 1 {
		return User{}, ErrLastAdmin
	}
	var updated User
	if err := tx.QueryRow(ctx, `
		UPDATE public.users
		SET role = $2, enabled = $3, updated_at = clock_timestamp(), version = version + 1
		WHERE id = $1::uuid
		RETURNING id::text, username, email, role, enabled, must_change_password, created_at, updated_at, version`, targetID, input.Role, *input.Enabled).Scan(
		&updated.ID, &updated.Username, &updated.Email, &updated.Role, &updated.Enabled, &updated.MustChangePassword, &updated.CreatedAt, &updated.UpdatedAt, &updated.Version,
	); err != nil {
		return User{}, fmt.Errorf("update user: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE public.sessions SET revoked_at = clock_timestamp() WHERE user_id = $1::uuid AND revoked_at IS NULL`, targetID); err != nil {
		return User{}, fmt.Errorf("revoke sessions after user update: %w", err)
	}
	if event.Details == nil {
		event.Details = map[string]any{}
	}
	event.Details["previous_role"] = current.Role
	event.Details["previous_enabled"] = current.Enabled
	if err := insertAudit(ctx, tx, event); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit update user: %w", err)
	}
	return updated, nil
}

func (r *PostgresRepository) ResetPassword(ctx context.Context, targetID, passwordHash string, event AuditEvent) (User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin password reset transaction: %w", err)
	}
	defer rollback(tx)
	var user User
	if err := tx.QueryRow(ctx, `
		UPDATE public.users
		SET password_hash = $2, must_change_password = true, password_changed_at = clock_timestamp(), updated_at = clock_timestamp(), version = version + 1
		WHERE id = $1::uuid
		RETURNING id::text, username, email, role, enabled, must_change_password, created_at, updated_at, version`, targetID, passwordHash).Scan(
		&user.ID, &user.Username, &user.Email, &user.Role, &user.Enabled, &user.MustChangePassword, &user.CreatedAt, &user.UpdatedAt, &user.Version,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("reset user password: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE public.sessions SET revoked_at = clock_timestamp() WHERE user_id = $1::uuid AND revoked_at IS NULL`, targetID); err != nil {
		return User{}, fmt.Errorf("revoke sessions after password reset: %w", err)
	}
	if err := insertAudit(ctx, tx, event); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit password reset: %w", err)
	}
	return user, nil
}

func (r *PostgresRepository) ChangePassword(ctx context.Context, userID, passwordHash string, event AuditEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin password change transaction: %w", err)
	}
	defer rollback(tx)
	result, err := tx.Exec(ctx, `
		UPDATE public.users
		SET password_hash = $2, must_change_password = false, password_changed_at = clock_timestamp(), updated_at = clock_timestamp(), version = version + 1
		WHERE id = $1::uuid AND enabled = true`, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("change user password: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUnauthenticated
	}
	if _, err := tx.Exec(ctx, `UPDATE public.sessions SET revoked_at = clock_timestamp() WHERE user_id = $1::uuid AND revoked_at IS NULL`, userID); err != nil {
		return fmt.Errorf("revoke sessions after password change: %w", err)
	}
	if err := insertAudit(ctx, tx, event); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password change: %w", err)
	}
	return nil
}

func lockEnabledAdmins(ctx context.Context, tx pgx.Tx) (int, error) {
	rows, err := tx.Query(ctx, `SELECT id::text FROM public.users WHERE role = 'admin' AND enabled = true ORDER BY id FOR UPDATE`)
	if err != nil {
		return 0, fmt.Errorf("lock enabled Admins: %w", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("scan enabled Admin: %w", err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate enabled Admins: %w", err)
	}
	return count, nil
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

type auditExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func insertAudit(ctx context.Context, execer auditExecer, event AuditEvent) error {
	if event.Details == nil {
		event.Details = map[string]any{}
	}
	details, err := json.Marshal(event.Details)
	if err != nil {
		return fmt.Errorf("encode audit details: %w", err)
	}
	var actor any
	if event.ActorUserID != "" {
		actor = event.ActorUserID
	}
	var target any
	if event.TargetID != "" {
		target = event.TargetID
	}
	if _, err := execer.Exec(ctx, `
		INSERT INTO public.audit_events (
			actor_user_id, action, target_type, target_id, outcome,
			request_id, client_ip, user_agent, details
		) VALUES ($1::uuid, $2, $3, $4::uuid, $5, $6, $7::inet, $8, $9::jsonb)`,
		actor, event.Action, event.TargetType, target, event.Outcome,
		event.Context.RequestID, event.Context.ClientIP, event.Context.UserAgent, details,
	); err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

func rollback(tx pgx.Tx) {
	rollbackContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = tx.Rollback(rollbackContext)
}
