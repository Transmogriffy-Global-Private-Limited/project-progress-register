package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const taskProjection = `
	SELECT t.id::text, t.project_id::text, t.name, t.goals_markdown, t.description_markdown,
	       creator.id::text, creator.username,
	       COALESCE(responsibilities.members, '[]'::jsonb),
	       COALESCE(t.target_date::text, ''), t.created_at, t.updated_at, t.version
	FROM public.tasks t
	JOIN public.users creator ON creator.id = t.created_by
	LEFT JOIN LATERAL (
		SELECT jsonb_agg(
			jsonb_build_object('user_id', u.id, 'username', u.username, 'enabled', u.enabled)
			ORDER BY lower(u.username), u.id
		) AS members
		FROM public.task_responsibilities tr
		JOIN public.users u ON u.id = tr.user_id
		WHERE tr.task_id = t.id
	) responsibilities ON true`

func scanTask(row scanner) (Task, error) {
	var task Task
	var responsibleMembers []byte
	var targetDate string
	err := row.Scan(
		&task.ID, &task.ProjectID, &task.Name, &task.GoalsMarkdown, &task.DescriptionMarkdown,
		&task.CreatedBy.UserID, &task.CreatedBy.Username,
		&responsibleMembers,
		&targetDate, &task.CreatedAt, &task.UpdatedAt, &task.Version,
	)
	if err != nil {
		return Task{}, err
	}
	if err := json.Unmarshal(responsibleMembers, &task.ResponsibleMembers); err != nil {
		return Task{}, fmt.Errorf("decode task responsible Members: %w", err)
	}
	if task.ResponsibleMembers == nil {
		task.ResponsibleMembers = []ResponsibleMember{}
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
	responsibleIDs, err := validateResponsibleMembers(ctx, tx, projectID, input.ResponsibleUserIDs)
	if err != nil {
		return Task{}, err
	}
	var taskID string
	err = tx.QueryRow(ctx, `
		INSERT INTO public.tasks
			(project_id, name, goals_markdown, description_markdown, created_by, target_date)
		VALUES ($1::uuid, $2, $3, $4, $5::uuid, NULLIF($6, '')::date)
		RETURNING id::text`, projectID, input.Name, input.GoalsMarkdown, input.DescriptionMarkdown, actorID, input.TargetDate).Scan(&taskID)
	if err != nil {
		return Task{}, fmt.Errorf("insert task: %w", err)
	}
	if err := replaceTaskResponsibilities(ctx, tx, taskID, responsibleIDs); err != nil {
		return Task{}, err
	}
	event.TargetID = taskID
	event.Details["responsible_user_ids"] = responsibleIDs
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
	var previousName, previousGoals, previousDescription, previousTargetDate string
	var previousResponsibleIDs []string
	err = tx.QueryRow(ctx, `
		SELECT t.version,t.name,t.goals_markdown,t.description_markdown,
		       ARRAY(SELECT tr.user_id::text FROM public.task_responsibilities tr WHERE tr.task_id=t.id ORDER BY tr.user_id),
		       COALESCE(t.target_date::text,'')
		FROM public.tasks t
		WHERE t.id = $1::uuid AND t.project_id = $2::uuid AND ($4::boolean OR t.created_by = $3::uuid)
		FOR UPDATE`, taskID, projectID, actorID, admin).Scan(&currentVersion, &previousName, &previousGoals, &previousDescription, &previousResponsibleIDs, &previousTargetDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("lock task: %w", err)
	}
	if currentVersion != input.ExpectedVersion {
		return Task{}, ErrConflict
	}
	if input.LegacySingular && len(previousResponsibleIDs) > 1 {
		return Task{}, ErrTaskV2Required
	}
	responsibleIDs, err := validateResponsibleMembers(ctx, tx, projectID, input.ResponsibleUserIDs)
	if err != nil {
		return Task{}, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE public.tasks
		SET name = $3, goals_markdown = $4, description_markdown = $5,
		    target_date = NULLIF($6, '')::date,
		    updated_at = clock_timestamp(), version = version + 1
		WHERE id = $1::uuid AND project_id = $2::uuid`,
		taskID, projectID, input.Name, input.GoalsMarkdown, input.DescriptionMarkdown, input.TargetDate)
	if err != nil {
		return Task{}, fmt.Errorf("update task: %w", err)
	}
	if err := replaceTaskResponsibilities(ctx, tx, taskID, responsibleIDs); err != nil {
		return Task{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO public.task_revisions(
			task_id,from_version,to_version,
			previous_name,previous_goals_markdown,previous_description_markdown,previous_responsible_user_ids,previous_target_date,
			new_name,new_goals_markdown,new_description_markdown,new_responsible_user_ids,new_target_date,change_reason,edited_by
		) VALUES(
			$1::uuid,$2,$3,
			$4,$5,$6,$7::uuid[],NULLIF($8,'')::date,
			$9,$10,$11,$12::uuid[],NULLIF($13,'')::date,'user_edit',$14::uuid
		)`, taskID, currentVersion, currentVersion+1,
		previousName, previousGoals, previousDescription, previousResponsibleIDs, previousTargetDate,
		input.Name, input.GoalsMarkdown, input.DescriptionMarkdown, responsibleIDs, input.TargetDate, actorID); err != nil {
		return Task{}, fmt.Errorf("insert task revision: %w", err)
	}
	event.Details["from_version"] = currentVersion
	event.Details["to_version"] = currentVersion + 1
	event.Details["responsible_user_ids"] = responsibleIDs
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

func validateResponsibleMembers(ctx context.Context, tx pgx.Tx, projectID string, responsibleUserIDs []string) ([]string, error) {
	if len(responsibleUserIDs) == 0 {
		return []string{}, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT u.id::text
		FROM public.users u
		WHERE u.id = ANY($2::uuid[]) AND u.enabled = true AND u.role = 'member'
		  AND EXISTS (
			SELECT 1 FROM public.project_members pm
			WHERE pm.project_id = $1::uuid AND pm.user_id = u.id AND pm.removed_at IS NULL
		  )
		ORDER BY u.id
		FOR SHARE`, projectID, responsibleUserIDs)
	if err != nil {
		return nil, fmt.Errorf("validate responsible Members: %w", err)
	}
	defer rows.Close()
	validated := make([]string, 0, len(responsibleUserIDs))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan responsible Member: %w", err)
		}
		validated = append(validated, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate responsible Members: %w", err)
	}
	if len(validated) != len(responsibleUserIDs) {
		return nil, ErrInvalidResponsible
	}
	return validated, nil
}

func replaceTaskResponsibilities(ctx context.Context, tx pgx.Tx, taskID string, responsibleUserIDs []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM public.task_responsibilities WHERE task_id=$1::uuid AND NOT (user_id=ANY($2::uuid[]))`, taskID, responsibleUserIDs); err != nil {
		return fmt.Errorf("remove task responsibilities: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO public.task_responsibilities(task_id,user_id)
		SELECT $1::uuid, id FROM unnest($2::uuid[]) AS ids(id)
		ON CONFLICT (task_id,user_id) DO NOTHING`, taskID, responsibleUserIDs); err != nil {
		return fmt.Errorf("add task responsibilities: %w", err)
	}
	return nil
}
