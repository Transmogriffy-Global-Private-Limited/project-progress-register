package projects

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

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &PostgresRepository{pool: pool}, nil
}

const projectProjection = `
	SELECT p.id::text, p.name, p.description_markdown, p.active, p.created_by::text,
	       p.created_at, p.updated_at, p.version,
	       COALESCE(g.id::text, ''), COALESCE(g.version, 0),
	       COALESCE(g.latitude::double precision, 0), COALESCE(g.longitude::double precision, 0),
	       COALESCE(g.radius_metres::double precision, 0), COALESCE(g.max_accuracy_metres::double precision, 0),
	       COALESCE(g.valid_from, 'epoch'::timestamptz)
	FROM public.projects p
	LEFT JOIN public.project_geofences g ON g.project_id = p.id AND g.valid_to IS NULL`

type scanner interface{ Scan(...any) error }

func scanProject(row scanner) (Project, error) {
	var project Project
	var geofenceID string
	var geofence Geofence
	var validFrom time.Time
	err := row.Scan(&project.ID, &project.Name, &project.DescriptionMarkdown, &project.Active, &project.CreatedBy,
		&project.CreatedAt, &project.UpdatedAt, &project.Version,
		&geofenceID, &geofence.Version, &geofence.Latitude, &geofence.Longitude,
		&geofence.RadiusMetres, &geofence.MaxAccuracyMetres, &validFrom)
	if err != nil {
		return Project{}, err
	}
	if geofenceID != "" {
		geofence.ID = geofenceID
		geofence.ValidFrom = validFrom
		project.Geofence = &geofence
	}
	return project, nil
}

func (r *PostgresRepository) ListProjects(ctx context.Context, actorID string, admin bool) ([]Project, error) {
	rows, err := r.pool.Query(ctx, projectProjection+`
		WHERE $2::boolean OR EXISTS (
			SELECT 1 FROM public.project_members pm
			WHERE pm.project_id = p.id AND pm.user_id = $1::uuid AND pm.removed_at IS NULL
		)
		ORDER BY lower(p.name), p.id`, actorID, admin)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()
	result := make([]Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		result = append(result, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return result, nil
}

func (r *PostgresRepository) GetProject(ctx context.Context, actorID string, admin bool, projectID string) (Project, error) {
	project, err := scanProject(r.pool.QueryRow(ctx, projectProjection+`
		WHERE p.id = $3::uuid AND ($2::boolean OR EXISTS (
			SELECT 1 FROM public.project_members pm
			WHERE pm.project_id = p.id AND pm.user_id = $1::uuid AND pm.removed_at IS NULL
		))`, actorID, admin, projectID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("query project: %w", err)
	}
	return project, nil
}

func (r *PostgresRepository) CreateProject(ctx context.Context, input CreateProjectInput, event auditEvent) (Project, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Project{}, fmt.Errorf("begin create project: %w", err)
	}
	defer rollback(tx)
	var project Project
	err = tx.QueryRow(ctx, `
		INSERT INTO public.projects (name, description_markdown, created_by)
		VALUES ($1, $2, $3::uuid)
		RETURNING id::text, name, description_markdown, active, created_by::text, created_at, updated_at, version`,
		input.Name, input.DescriptionMarkdown, event.ActorUserID,
	).Scan(&project.ID, &project.Name, &project.DescriptionMarkdown, &project.Active, &project.CreatedBy, &project.CreatedAt, &project.UpdatedAt, &project.Version)
	if err != nil {
		return Project{}, fmt.Errorf("insert project: %w", err)
	}
	event.TargetID = project.ID
	if event.Details == nil {
		event.Details = map[string]any{}
	}
	event.Details["name"] = project.Name
	if err := insertAudit(ctx, tx, event); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Project{}, fmt.Errorf("commit create project: %w", err)
	}
	return project, nil
}

func (r *PostgresRepository) UpdateProject(ctx context.Context, projectID string, input UpdateProjectInput, event auditEvent) (Project, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Project{}, fmt.Errorf("begin update project: %w", err)
	}
	defer rollback(tx)
	var currentVersion int64
	if err := tx.QueryRow(ctx, `SELECT version FROM public.projects WHERE id = $1::uuid FOR UPDATE`, projectID).Scan(&currentVersion); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Project{}, ErrNotFound
		}
		return Project{}, fmt.Errorf("lock project: %w", err)
	}
	if currentVersion != input.ExpectedVersion {
		return Project{}, ErrConflict
	}
	var project Project
	err = tx.QueryRow(ctx, `
		UPDATE public.projects
		SET name = $2, description_markdown = $3, active = $4, updated_at = clock_timestamp(), version = version + 1
		WHERE id = $1::uuid
		RETURNING id::text, name, description_markdown, active, created_by::text, created_at, updated_at, version`,
		projectID, input.Name, input.DescriptionMarkdown, *input.Active,
	).Scan(&project.ID, &project.Name, &project.DescriptionMarkdown, &project.Active, &project.CreatedBy, &project.CreatedAt, &project.UpdatedAt, &project.Version)
	if err != nil {
		return Project{}, fmt.Errorf("update project: %w", err)
	}
	if err := insertAudit(ctx, tx, event); err != nil {
		return Project{}, err
	}
	if err := attachCurrentGeofence(ctx, tx, project.ID, &project); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Project{}, fmt.Errorf("commit update project: %w", err)
	}
	return project, nil
}

func (r *PostgresRepository) ListMembers(ctx context.Context, projectID string) ([]Member, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.projects WHERE id = $1::uuid)`, projectID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check project: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}
	rows, err := r.pool.Query(ctx, `
		SELECT u.id::text, u.username, u.email, u.enabled, pm.added_at
		FROM public.project_members pm
		JOIN public.users u ON u.id = pm.user_id
		WHERE pm.project_id = $1::uuid AND pm.removed_at IS NULL
		ORDER BY u.username, u.id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("query project members: %w", err)
	}
	defer rows.Close()
	members := make([]Member, 0)
	for rows.Next() {
		var member Member
		if err := rows.Scan(&member.UserID, &member.Username, &member.Email, &member.Enabled, &member.AddedAt); err != nil {
			return nil, fmt.Errorf("scan project member: %w", err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project members: %w", err)
	}
	return members, nil
}

func (r *PostgresRepository) AddMember(ctx context.Context, projectID, userID string, event auditEvent) (Member, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Member{}, fmt.Errorf("begin add project member: %w", err)
	}
	defer rollback(tx)
	if err := lockProject(ctx, tx, projectID); err != nil {
		return Member{}, err
	}
	var member Member
	var role string
	if err := tx.QueryRow(ctx, `SELECT id::text, username, email, enabled, role FROM public.users WHERE id = $1::uuid FOR SHARE`, userID).Scan(&member.UserID, &member.Username, &member.Email, &member.Enabled, &role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Member{}, ErrInvalidMember
		}
		return Member{}, fmt.Errorf("query membership target: %w", err)
	}
	if !member.Enabled || role != "member" {
		return Member{}, ErrInvalidMember
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO public.project_members (project_id, user_id, added_by)
		VALUES ($1::uuid, $2::uuid, $3::uuid)
		RETURNING added_at`, projectID, userID, event.ActorUserID).Scan(&member.AddedAt)
	if isUniqueViolation(err) {
		return Member{}, ErrConflict
	}
	if err != nil {
		return Member{}, fmt.Errorf("insert project member: %w", err)
	}
	if err := insertAudit(ctx, tx, event); err != nil {
		return Member{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Member{}, fmt.Errorf("commit add project member: %w", err)
	}
	return member, nil
}

func (r *PostgresRepository) RemoveMember(ctx context.Context, projectID, userID string, event auditEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin remove project member: %w", err)
	}
	defer rollback(tx)
	if err := lockProject(ctx, tx, projectID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `
		UPDATE public.project_members
		SET removed_at = clock_timestamp(), removed_by = $3::uuid
		WHERE project_id = $1::uuid AND user_id = $2::uuid AND removed_at IS NULL`, projectID, userID, event.ActorUserID)
	if err != nil {
		return fmt.Errorf("close project membership: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	cleared, err := tx.Exec(ctx, `
		WITH affected AS MATERIALIZED (
			SELECT t.id,t.version,t.name,t.goals_markdown,t.description_markdown,t.target_date,
			       ARRAY(SELECT tr.user_id FROM public.task_responsibilities tr WHERE tr.task_id=t.id ORDER BY tr.user_id) AS responsible_user_ids
			FROM public.tasks t
			WHERE t.project_id=$1::uuid AND EXISTS (
				SELECT 1 FROM public.task_responsibilities tr WHERE tr.task_id=t.id AND tr.user_id=$2::uuid
			)
			FOR UPDATE
		), revisions AS (
			INSERT INTO public.task_revisions(
				task_id,from_version,to_version,
				previous_name,previous_goals_markdown,previous_description_markdown,previous_responsible_user_ids,previous_target_date,
				new_name,new_goals_markdown,new_description_markdown,new_responsible_user_ids,new_target_date,change_reason,edited_by
			)
			SELECT id,version,version+1,name,goals_markdown,description_markdown,responsible_user_ids,target_date,
			       name,goals_markdown,description_markdown,array_remove(responsible_user_ids,$2::uuid),target_date,'membership_removed',$3::uuid
			FROM affected
			RETURNING task_id
		)
		UPDATE public.tasks t
		SET updated_at=clock_timestamp(),version=version+1
		FROM revisions r WHERE t.id=r.task_id`, projectID, userID, event.ActorUserID)
	if err != nil {
		return fmt.Errorf("clear removed Member task responsibility: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM public.task_responsibilities WHERE user_id=$1::uuid AND task_id IN (SELECT id FROM public.tasks WHERE project_id=$2::uuid)`, userID, projectID); err != nil {
		return fmt.Errorf("remove task responsibility assignments: %w", err)
	}
	if event.Details == nil {
		event.Details = map[string]any{}
	}
	event.Details["cleared_task_responsibility_count"] = cleared.RowsAffected()
	if err := insertAudit(ctx, tx, event); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit remove project member: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ReplaceGeofence(ctx context.Context, projectID string, input ReplaceGeofenceInput, event auditEvent) (Geofence, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Geofence{}, fmt.Errorf("begin replace geofence: %w", err)
	}
	defer rollback(tx)
	if err := lockProject(ctx, tx, projectID); err != nil {
		return Geofence{}, err
	}
	var currentID string
	var currentVersion int64
	err = tx.QueryRow(ctx, `SELECT id::text, version FROM public.project_geofences WHERE project_id = $1::uuid AND valid_to IS NULL FOR UPDATE`, projectID).Scan(&currentID, &currentVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		currentVersion = 0
	} else if err != nil {
		return Geofence{}, fmt.Errorf("lock current geofence: %w", err)
	}
	if currentVersion != input.ExpectedVersion {
		return Geofence{}, ErrConflict
	}
	if currentID != "" {
		if _, err := tx.Exec(ctx, `UPDATE public.project_geofences SET valid_to = clock_timestamp() WHERE id = $1::uuid`, currentID); err != nil {
			return Geofence{}, fmt.Errorf("close current geofence: %w", err)
		}
	}
	var geofence Geofence
	err = tx.QueryRow(ctx, `
		INSERT INTO public.project_geofences
			(project_id, version, latitude, longitude, radius_metres, max_accuracy_metres, created_by)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7::uuid)
		RETURNING id::text, version, latitude::double precision, longitude::double precision,
		          radius_metres::double precision, max_accuracy_metres::double precision, valid_from`,
		projectID, currentVersion+1, input.Latitude, input.Longitude, input.RadiusMetres, input.MaxAccuracyMetres, event.ActorUserID,
	).Scan(&geofence.ID, &geofence.Version, &geofence.Latitude, &geofence.Longitude, &geofence.RadiusMetres, &geofence.MaxAccuracyMetres, &geofence.ValidFrom)
	if err != nil {
		return Geofence{}, fmt.Errorf("insert geofence: %w", err)
	}
	if event.Details == nil {
		event.Details = map[string]any{}
	}
	event.Details["version"] = geofence.Version
	if err := insertAudit(ctx, tx, event); err != nil {
		return Geofence{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Geofence{}, fmt.Errorf("commit replace geofence: %w", err)
	}
	return geofence, nil
}

func (r *PostgresRepository) AppendAudit(ctx context.Context, event auditEvent) error {
	return insertAudit(ctx, r.pool, event)
}

func lockProject(ctx context.Context, tx pgx.Tx, projectID string) error {
	var id string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM public.projects WHERE id = $1::uuid FOR UPDATE`, projectID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock project: %w", err)
	}
	return nil
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func attachCurrentGeofence(ctx context.Context, querier queryRower, projectID string, project *Project) error {
	var geofence Geofence
	err := querier.QueryRow(ctx, `
		SELECT id::text, version, latitude::double precision, longitude::double precision,
		       radius_metres::double precision, max_accuracy_metres::double precision, valid_from
		FROM public.project_geofences
		WHERE project_id = $1::uuid AND valid_to IS NULL`, projectID).Scan(
		&geofence.ID, &geofence.Version, &geofence.Latitude, &geofence.Longitude,
		&geofence.RadiusMetres, &geofence.MaxAccuracyMetres, &geofence.ValidFrom,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		project.Geofence = nil
		return nil
	}
	if err != nil {
		return fmt.Errorf("query current geofence: %w", err)
	}
	project.Geofence = &geofence
	return nil
}

type auditExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func insertAudit(ctx context.Context, execer auditExecer, event auditEvent) error {
	if event.Details == nil {
		event.Details = map[string]any{}
	}
	details, err := json.Marshal(event.Details)
	if err != nil {
		return fmt.Errorf("encode project audit details: %w", err)
	}
	_, err = execer.Exec(ctx, `
		INSERT INTO public.audit_events
			(actor_user_id, action, target_type, target_id, outcome, request_id, client_ip, user_agent, details)
		VALUES (NULLIF($1, '')::uuid, $2, $3, NULLIF($4, '')::uuid, $5, $6, $7::inet, $8, $9::jsonb)`,
		event.ActorUserID, event.Action, event.TargetType, event.TargetID, event.Outcome,
		event.Context.RequestID, event.Context.ClientIP, event.Context.UserAgent, details)
	if err != nil {
		return fmt.Errorf("insert project audit event: %w", err)
	}
	return nil
}

func rollback(tx pgx.Tx) { _ = tx.Rollback(context.Background()) }

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
