package projects

import (
	"context"
	"fmt"
)

func (r *PostgresRepository) GetDashboard(ctx context.Context, actorID string, admin bool) ([]DashboardProject, error) {
	rows, err := r.pool.Query(ctx, `
		WITH authorized_projects AS (
			SELECT p.id,p.name,p.active
			FROM public.projects p
			WHERE $2::boolean OR EXISTS (
				SELECT 1 FROM public.project_members pm
				WHERE pm.project_id=p.id AND pm.user_id=$1::uuid AND pm.removed_at IS NULL
			)
		), task_stats AS (
			SELECT t.project_id,count(*) AS task_count
			FROM public.tasks t JOIN authorized_projects p ON p.id=t.project_id
			GROUP BY t.project_id
		), progress_stats AS (
			SELECT t.project_id,count(pu.id) AS progress_count,max(pu.created_at) AS latest_progress_at
			FROM public.tasks t JOIN authorized_projects p ON p.id=t.project_id
			LEFT JOIN public.progress_updates pu ON pu.task_id=t.id
			GROUP BY t.project_id
		), suggestion_stats AS (
			SELECT t.project_id,count(s.id) AS suggestion_count
			FROM public.tasks t JOIN authorized_projects p ON p.id=t.project_id
			JOIN public.progress_updates pu ON pu.task_id=t.id
			JOIN public.update_comments c ON c.progress_update_id=pu.id
			JOIN public.accepted_suggestions s ON s.comment_id=c.id
			GROUP BY t.project_id
		), latest_assessments AS (
			SELECT DISTINCT ON (a.task_id) t.project_id,a.verdict
			FROM public.task_assessments a
			JOIN public.tasks t ON t.id=a.task_id
			JOIN authorized_projects p ON p.id=t.project_id
			ORDER BY a.task_id,a.version DESC
		), assessment_stats AS (
			SELECT project_id,
			       count(*) FILTER (WHERE verdict='on_track') AS on_track,
			       count(*) FILTER (WHERE verdict='needs_attention') AS needs_attention,
			       count(*) FILTER (WHERE verdict='blocked') AS blocked,
			       count(*) FILTER (WHERE verdict='complete') AS complete
			FROM latest_assessments GROUP BY project_id
		)
		SELECT p.id::text,p.name,p.active,
		       COALESCE(t.task_count,0),COALESCE(pr.progress_count,0),COALESCE(s.suggestion_count,0),pr.latest_progress_at,
		       COALESCE(a.on_track,0),COALESCE(a.needs_attention,0),COALESCE(a.blocked,0),COALESCE(a.complete,0)
		FROM authorized_projects p
		LEFT JOIN task_stats t ON t.project_id=p.id
		LEFT JOIN progress_stats pr ON pr.project_id=p.id
		LEFT JOIN suggestion_stats s ON s.project_id=p.id
		LEFT JOIN assessment_stats a ON a.project_id=p.id
		ORDER BY p.active DESC,lower(p.name),p.id`, actorID, admin)
	if err != nil {
		return nil, fmt.Errorf("query dashboard: %w", err)
	}
	defer rows.Close()
	items := []DashboardProject{}
	for rows.Next() {
		var item DashboardProject
		if err := rows.Scan(&item.ID, &item.Name, &item.Active, &item.TaskCount, &item.ProgressUpdateCount, &item.AcceptedSuggestionCount, &item.LatestProgressAt, &item.CurrentAssessments.OnTrack, &item.CurrentAssessments.NeedsAttention, &item.CurrentAssessments.Blocked, &item.CurrentAssessments.Complete); err != nil {
			return nil, fmt.Errorf("scan dashboard: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dashboard: %w", err)
	}
	return items, nil
}
