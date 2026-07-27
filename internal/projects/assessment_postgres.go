package projects

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const assessmentProjection = `
	SELECT a.id::text,a.task_id::text,a.version,a.verdict,a.remark_markdown,
	       assessor.id::text,assessor.username,a.created_at
	FROM public.task_assessments a
	JOIN public.users assessor ON assessor.id=a.assessed_by`

func scanAssessment(row scanner) (Assessment, error) {
	var assessment Assessment
	err := row.Scan(&assessment.ID, &assessment.TaskID, &assessment.Version, &assessment.Verdict, &assessment.RemarkMarkdown, &assessment.AssessedBy.UserID, &assessment.AssessedBy.Username, &assessment.CreatedAt)
	return assessment, err
}

func (r *PostgresRepository) GetCurrentAssessment(ctx context.Context, actorID string, admin bool, projectID, taskID string) (*Assessment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if err := lockAuthorizedProject(ctx, tx, projectID, actorID, admin, false); err != nil {
		return nil, err
	}
	if err := requireScopedTask(ctx, tx, projectID, taskID, false); err != nil {
		return nil, err
	}
	assessment, err := scanAssessment(tx.QueryRow(ctx, assessmentProjection+` WHERE a.task_id=$1::uuid ORDER BY a.version DESC LIMIT 1`, taskID))
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &assessment, nil
}

func (r *PostgresRepository) ListAssessments(ctx context.Context, actorID string, admin bool, projectID, taskID string) ([]Assessment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if err := lockAuthorizedProject(ctx, tx, projectID, actorID, admin, false); err != nil {
		return nil, err
	}
	if err := requireScopedTask(ctx, tx, projectID, taskID, false); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, assessmentProjection+` WHERE a.task_id=$1::uuid ORDER BY a.version DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Assessment{}
	for rows.Next() {
		item, err := scanAssessment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PostgresRepository) CreateAssessment(ctx context.Context, actorID string, admin bool, projectID, taskID string, input assessmentPersistenceInput, event auditEvent) (Assessment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Assessment{}, err
	}
	defer rollback(tx)
	if err := lockAuthorizedProject(ctx, tx, projectID, actorID, admin, true); err != nil {
		return Assessment{}, err
	}
	if err := requireScopedTask(ctx, tx, projectID, taskID, true); err != nil {
		return Assessment{}, err
	}
	var currentVersion int64
	err = tx.QueryRow(ctx, `SELECT version FROM public.task_assessments WHERE task_id=$1::uuid ORDER BY version DESC LIMIT 1`, taskID).Scan(&currentVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		currentVersion = 0
	} else if err != nil {
		return Assessment{}, err
	}
	if currentVersion != input.ExpectedVersion {
		return Assessment{}, ErrConflict
	}
	var assessmentID string
	if err := tx.QueryRow(ctx, `INSERT INTO public.task_assessments(task_id,version,verdict,remark_markdown,assessed_by) VALUES($1::uuid,$2,$3,$4,$5::uuid) RETURNING id::text`, taskID, currentVersion+1, input.Verdict, input.RemarkMarkdown, actorID).Scan(&assessmentID); err != nil {
		return Assessment{}, err
	}
	event.TargetID = assessmentID
	if err := insertAudit(ctx, tx, event); err != nil {
		return Assessment{}, err
	}
	assessment, err := scanAssessment(tx.QueryRow(ctx, assessmentProjection+` WHERE a.id=$1::uuid`, assessmentID))
	if err != nil {
		return Assessment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Assessment{}, err
	}
	return assessment, nil
}

func requireScopedTask(ctx context.Context, tx pgx.Tx, projectID, taskID string, forUpdate bool) error {
	query := `SELECT true FROM public.tasks WHERE id=$1::uuid AND project_id=$2::uuid`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var exists bool
	if err := tx.QueryRow(ctx, query, taskID, projectID).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("resolve task: %w", err)
	}
	return nil
}
