package projects

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const taskProjection = `
	SELECT t.id::text, t.project_id::text, t.name, t.goals_markdown, t.description_markdown,
	       creator.id::text, creator.username,
	       COALESCE(responsible.id::text, ''), COALESCE(responsible.username, ''), COALESCE(responsible.enabled, false),
	       COALESCE(t.target_date::text, ''), t.created_at, t.updated_at, t.version
	FROM public.tasks t
	JOIN public.users creator ON creator.id = t.created_by
	LEFT JOIN public.users responsible ON responsible.id = t.responsible_user_id`

func scanTask(row scanner) (Task, error) {
	var task Task
	var responsibleID, responsibleUsername, targetDate string
	var responsibleEnabled bool
	err := row.Scan(
		&task.ID, &task.ProjectID, &task.Name, &task.GoalsMarkdown, &task.DescriptionMarkdown,
		&task.CreatedBy.UserID, &task.CreatedBy.Username,
		&responsibleID, &responsibleUsername, &responsibleEnabled,
		&targetDate, &task.CreatedAt, &task.UpdatedAt, &task.Version,
	)
	if err != nil {
		return Task{}, err
	}
	if responsibleID != "" {
		task.ResponsibleMember = &ResponsibleMember{UserID: responsibleID, Username: responsibleUsername, Enabled: responsibleEnabled}
	}
	if targetDate != "" {
		task.TargetDate = &targetDate
	}
	return task, nil
}

func (r *PostgresRepository) ListTasks(ctx context.Context, actorID string, admin bool, projectID string) ([]Task, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin list tasks: %w", err)
	}
	defer rollback(tx)
	if err := lockAuthorizedProject(ctx, tx, projectID, actorID, admin, false); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, taskProjection+` WHERE t.project_id = $1::uuid ORDER BY lower(t.name), t.id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	tasks := make([]Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit list tasks: %w", err)
	}
	return tasks, nil
}

func (r *PostgresRepository) GetTask(ctx context.Context, actorID string, admin bool, projectID, taskID string) (Task, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Task{}, fmt.Errorf("begin get task: %w", err)
	}
	defer rollback(tx)
	if err := lockAuthorizedProject(ctx, tx, projectID, actorID, admin, false); err != nil {
		return Task{}, err
	}
	task, err := scanTask(tx.QueryRow(ctx, taskProjection+` WHERE t.project_id = $1::uuid AND t.id = $2::uuid`, projectID, taskID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("query task: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, fmt.Errorf("commit get task: %w", err)
	}
	return task, nil
}

func (r *PostgresRepository) CreateTask(ctx context.Context, actorID string, admin bool, projectID string, input taskPersistenceInput, event auditEvent) (Task, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Task{}, fmt.Errorf("begin create task: %w", err)
	}
	defer rollback(tx)
	if err := lockAuthorizedProject(ctx, tx, projectID, actorID, admin, true); err != nil {
		return Task{}, err
	}
	responsibleID, err := validateResponsibleMember(ctx, tx, projectID, input.ResponsibleUserID)
	if err != nil {
		return Task{}, err
	}
	var taskID string
	err = tx.QueryRow(ctx, `
		INSERT INTO public.tasks
			(project_id, name, goals_markdown, description_markdown, created_by, responsible_user_id, target_date)
		VALUES ($1::uuid, $2, $3, $4, $5::uuid, NULLIF($6, '')::uuid, NULLIF($7, '')::date)
		RETURNING id::text`, projectID, input.Name, input.GoalsMarkdown, input.DescriptionMarkdown, actorID, responsibleID, input.TargetDate).Scan(&taskID)
	if err != nil {
		return Task{}, fmt.Errorf("insert task: %w", err)
	}
	event.TargetID = taskID
	if err := insertAudit(ctx, tx, event); err != nil {
		return Task{}, err
	}
	task, err := scanTask(tx.QueryRow(ctx, taskProjection+` WHERE t.id = $1::uuid`, taskID))
	if err != nil {
		return Task{}, fmt.Errorf("query created task: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, fmt.Errorf("commit create task: %w", err)
	}
	return task, nil
}

func (r *PostgresRepository) UpdateTask(ctx context.Context, actorID string, admin bool, projectID, taskID string, input taskPersistenceInput, event auditEvent) (Task, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Task{}, fmt.Errorf("begin update task: %w", err)
	}
	defer rollback(tx)
	if err := lockAuthorizedProject(ctx, tx, projectID, actorID, admin, true); err != nil {
		return Task{}, err
	}
	var currentVersion int64
	err = tx.QueryRow(ctx, `
		SELECT version FROM public.tasks
		WHERE id = $1::uuid AND project_id = $2::uuid AND ($4::boolean OR created_by = $3::uuid)
		FOR UPDATE`, taskID, projectID, actorID, admin).Scan(&currentVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("lock task: %w", err)
	}
	if currentVersion != input.ExpectedVersion {
		return Task{}, ErrConflict
	}
	responsibleID, err := validateResponsibleMember(ctx, tx, projectID, input.ResponsibleUserID)
	if err != nil {
		return Task{}, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE public.tasks
		SET name = $3, goals_markdown = $4, description_markdown = $5,
		    responsible_user_id = NULLIF($6, '')::uuid, target_date = NULLIF($7, '')::date,
		    updated_at = clock_timestamp(), version = version + 1
		WHERE id = $1::uuid AND project_id = $2::uuid`,
		taskID, projectID, input.Name, input.GoalsMarkdown, input.DescriptionMarkdown, responsibleID, input.TargetDate)
	if err != nil {
		return Task{}, fmt.Errorf("update task: %w", err)
	}
	if err := insertAudit(ctx, tx, event); err != nil {
		return Task{}, err
	}
	task, err := scanTask(tx.QueryRow(ctx, taskProjection+` WHERE t.id = $1::uuid`, taskID))
	if err != nil {
		return Task{}, fmt.Errorf("query updated task: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, fmt.Errorf("commit update task: %w", err)
	}
	return task, nil
}

func lockAuthorizedProject(ctx context.Context, tx pgx.Tx, projectID, actorID string, admin, requireActive bool) error {
	var active bool
	err := tx.QueryRow(ctx, `
		SELECT p.active
		FROM public.projects p
		WHERE p.id = $1::uuid
		  AND ($3::boolean OR EXISTS (
			SELECT 1 FROM public.project_members pm
			WHERE pm.project_id = p.id AND pm.user_id = $2::uuid AND pm.removed_at IS NULL
		  ))
		FOR SHARE`, projectID, actorID, admin).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock authorized project: %w", err)
	}
	if requireActive && !active {
		return ErrInactiveProject
	}
	return nil
}

func validateResponsibleMember(ctx context.Context, tx pgx.Tx, projectID, responsibleUserID string) (string, error) {
	if responsibleUserID == "" {
		return "", nil
	}
	var id string
	err := tx.QueryRow(ctx, `
		SELECT u.id::text
		FROM public.users u
		WHERE u.id = $2::uuid AND u.enabled = true AND u.role = 'member'
		  AND EXISTS (
			SELECT 1 FROM public.project_members pm
			WHERE pm.project_id = $1::uuid AND pm.user_id = u.id AND pm.removed_at IS NULL
		  )
		FOR SHARE`, projectID, responsibleUserID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalidResponsible
	}
	if err != nil {
		return "", fmt.Errorf("validate responsible Member: %w", err)
	}
	return id, nil
}
